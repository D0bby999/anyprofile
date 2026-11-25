// SPDX-License-Identifier: AGPL-3.0-or-later

import AppKit
import AVFoundation
import CoreGraphics
import CoreImage
import Darwin
import Foundation
import ImageIO
import Metal
import QuartzCore

// Zero-copy Metal native viewport for AnyProfile's Avo VMM.
// Maps triple-buffered 16 KiB-aligned POSIX shared memory (/avo.<pid>.0.<gen>)
// directly into a Metal buffer (MTLDevice.makeBuffer(bytesNoCopy:)) and blits/samples
// onto a Retina CAMetalLayer at 60/120 Hz, paired with desktop input, drag-and-drop,
// and a slim MuMu-style control rail.

@_silgen_name("shm_open")
func c_shm_open(_ name: UnsafePointer<CChar>, _ oflag: Int32, _ mode: mode_t) -> Int32

private let kToolbarWidth: CGFloat = 36

private let kMetalShaderSource = """
#include <metal_stdlib>
using namespace metal;

struct VSOut {
    float4 position [[position]];
    float2 uv;
};

vertex VSOut fullscreen_vs(uint vid [[vertex_id]]) {
    float2 pos[3] = { float2(-1.0, -1.0), float2(3.0, -1.0), float2(-1.0, 3.0) };
    float2 uv[3]  = { float2(0.0, 1.0),   float2(2.0, 1.0),   float2(0.0, -1.0) };
    VSOut out;
    out.position = float4(pos[vid], 0.0, 1.0);
    out.uv = uv[vid];
    return out;
}

fragment float4 fullscreen_fs(VSOut in [[stage_in]],
                              texture2d<float, access::sample> tex [[texture(0)]]) {
    constexpr sampler s(mag_filter::linear, min_filter::linear, address::clamp_to_edge);
    float4 c = tex.sample(s, in.uv);
    return float4(c.rgb, 1.0);
}
"""

final class ControlClient {
    private let path: String
    private var fd: Int32 = -1
    private let queue = DispatchQueue(label: "anyprofile.view.control", qos: .userInteractive)

    init(path: String) { self.path = path }

    func send(_ cmd: String) {
        queue.async { [self] in
            self.sendSync(cmd)
        }
    }

    func sendBatch(_ cmds: [String]) {
        guard !cmds.isEmpty else { return }
        queue.async { [self] in
            for cmd in cmds {
                self.sendSync(cmd)
            }
        }
    }

    func sendSyncNow(_ cmd: String) {
        queue.sync { [self] in
            self.sendSync(cmd)
        }
    }

    private func sendSync(_ cmd: String) {
        if self.fd < 0 { self.fd = connectUnix(self.path) }
        guard self.fd >= 0 else { return }
        let line = cmd.hasSuffix("\n") ? cmd : cmd + "\n"
        let bytes = Array(line.utf8)
        let sent = bytes.withUnsafeBytes { Darwin.write(self.fd, $0.baseAddress, $0.count) }
        if sent <= 0 {
            Darwin.close(self.fd)
            self.fd = -1
            return
        }
        var buf = [UInt8](repeating: 0, count: 256)
        _ = Darwin.read(self.fd, &buf, buf.count)
    }
}

final class ADBHelper {
    let bin: String
    let serial: String
    private let queue = DispatchQueue(label: "anyprofile.view.adb", qos: .userInitiated)

    init(bin: String, serial: String) {
        self.bin = bin
        self.serial = serial
    }

    var available: Bool { !bin.isEmpty && !serial.isEmpty }

    func run(_ args: [String], completion: ((Bool, String) -> Void)? = nil) {
        guard available else {
            completion?(false, "ADB unavailable")
            return
        }
        queue.async { [bin, serial] in
            let proc = Process()
            proc.executableURL = URL(fileURLWithPath: bin)
            proc.arguments = ["-s", serial] + args
            let pipe = Pipe()
            proc.standardOutput = pipe
            proc.standardError = pipe
            do {
                try proc.run()
                proc.waitUntilExit()
                let data = pipe.fileHandleForReading.readDataToEndOfFile()
                let out = String(data: data, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
                let ok = proc.terminationStatus == 0
                if let completion {
                    DispatchQueue.main.async { completion(ok, out) }
                }
            } catch {
                if let completion {
                    DispatchQueue.main.async { completion(false, error.localizedDescription) }
                }
            }
        }
    }
}

func connectUnix(_ path: String) -> Int32 {
    let fd = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
    guard fd >= 0 else { return -1 }
    var noSigPipe: Int32 = 1
    _ = setsockopt(fd, SOL_SOCKET, SO_NOSIGPIPE, &noSigPipe, socklen_t(MemoryLayout<Int32>.size))
    var addr = sockaddr_un()
    addr.sun_family = sa_family_t(AF_UNIX)
    let maxLen = MemoryLayout.size(ofValue: addr.sun_path)
    let utf8 = Array(path.utf8CString)
    guard utf8.count <= maxLen else {
        Darwin.close(fd)
        return -1
    }
    withUnsafeMutablePointer(to: &addr.sun_path) { ptr in
        ptr.withMemoryRebound(to: CChar.self, capacity: maxLen) { dst in
            for (i, ch) in utf8.enumerated() { dst[i] = ch }
        }
    }
    let len = socklen_t(MemoryLayout<sockaddr_un>.size)
    let res = withUnsafePointer(to: &addr) { ptr in
        ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sa in
            Darwin.connect(fd, sa, len)
        }
    }
    if res != 0 {
        Darwin.close(fd)
        return -1
    }
    return fd
}

// Streams frames from the Mac's laptop camera into avo-vmm's virtio-media
// `camera feed` socket whenever Android opens a front or back camera.
final class CameraFeeder: NSObject, AVCaptureVideoDataOutputSampleBufferDelegate {
    private struct Slot {
        let width: Int
        let height: Int
        let fps: Int
    }

    private let controlPath: String
    private let metalDevice: MTLDevice?
    private let onToast: (String) -> Void
    private let queue = DispatchQueue(label: "anyprofile.view.camera", qos: .userInteractive)
    private lazy var ciContext: CIContext = metalDevice.map { CIContext(mtlDevice: $0) } ?? CIContext()
    private let colorSpace = CGColorSpaceCreateDeviceRGB()

    private var slots: [Int: Slot] = [:]
    private var lastSent: [Int: CFTimeInterval] = [:]
    private var session: AVCaptureSession?
    private var feedFd: Int32 = -1
    private var stopped = false
    private var authPending = false
    private var warnedDenied = false

    init(controlPath: String, metalDevice: MTLDevice?, onToast: @escaping (String) -> Void) {
        self.controlPath = controlPath
        self.metalDevice = metalDevice
        self.onToast = onToast
        super.init()
    }

    func setActive(index: Int, width: Int, height: Int, fps: Int) {
        guard index >= 0, index < 2, width > 0, height > 0 else { return }
        queue.async { [self] in
            guard !self.stopped else { return }
            self.slots[index] = Slot(width: width, height: height, fps: max(1, fps))
            self.lastSent[index] = nil
            self.ensureStartedLocked()
        }
    }

    func setInactive(index: Int) {
        queue.async { [self] in
            self.slots.removeValue(forKey: index)
            self.lastSent.removeValue(forKey: index)
            if self.slots.isEmpty {
                self.stopSessionLocked()
            }
        }
    }

    func stopAll() {
        queue.sync { [self] in
            self.stopped = true
            self.slots.removeAll()
            self.lastSent.removeAll()
            self.stopSessionLocked()
        }
    }

    private func desiredPresetLocked(_ session: AVCaptureSession) -> AVCaptureSession.Preset {
        let maxW = slots.values.map(\.width).max() ?? 1280
        if maxW >= 1920, session.canSetSessionPreset(.hd1920x1080) {
            return .hd1920x1080
        }
        if session.canSetSessionPreset(.hd1280x720) {
            return .hd1280x720
        }
        return .high
    }

    private func ensureStartedLocked() {
        guard !stopped, !slots.isEmpty else { return }
        if let s = session {
            let preset = desiredPresetLocked(s)
            if s.sessionPreset != preset {
                s.beginConfiguration()
                s.sessionPreset = preset
                s.commitConfiguration()
            }
            if !s.isRunning { s.startRunning() }
            return
        }
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized:
            startSessionLocked()
        case .notDetermined:
            guard !authPending else { return }
            authPending = true
            AVCaptureDevice.requestAccess(for: .video) { [weak self] granted in
                guard let self else { return }
                self.queue.async {
                    self.authPending = false
                    guard !self.stopped, !self.slots.isEmpty else { return }
                    if granted {
                        self.startSessionLocked()
                    } else if !self.warnedDenied {
                        self.warnedDenied = true
                        self.onToast("Camera denied — enable AnyProfile in System Settings > Privacy > Camera")
                    }
                }
            }
        case .denied, .restricted:
            if !warnedDenied {
                warnedDenied = true
                onToast("Camera denied — enable AnyProfile in System Settings > Privacy > Camera")
            }
        @unknown default:
            break
        }
    }

    private func startSessionLocked() {
        guard session == nil else { return }
        guard let device = AVCaptureDevice.default(for: .video) else {
            onToast("No Mac camera found")
            return
        }
        do {
            let input = try AVCaptureDeviceInput(device: device)
            let s = AVCaptureSession()
            s.beginConfiguration()
            s.sessionPreset = desiredPresetLocked(s)
            guard s.canAddInput(input) else {
                s.commitConfiguration()
                return
            }
            s.addInput(input)

            let output = AVCaptureVideoDataOutput()
            output.videoSettings = [
                kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA,
            ]
            output.alwaysDiscardsLateVideoFrames = true
            output.setSampleBufferDelegate(self, queue: queue)
            guard s.canAddOutput(output) else {
                s.commitConfiguration()
                return
            }
            s.addOutput(output)
            s.commitConfiguration()
            self.session = s
            s.startRunning()
        } catch {
            onToast("Camera open failed")
        }
    }

    private func stopSessionLocked() {
        if let s = session {
            if s.isRunning { s.stopRunning() }
            session = nil
        }
        closeFeedLocked()
    }

    private func closeFeedLocked() {
        if feedFd >= 0 {
            Darwin.close(feedFd)
            feedFd = -1
        }
    }

    private func ensureFeedSocketLocked() -> Int32 {
        if feedFd >= 0 { return feedFd }
        let fd = connectUnix(controlPath)
        guard fd >= 0 else { return -1 }
        let cmd = Array("camera feed\n".utf8)
        guard writeAll(fd, cmd) else {
            Darwin.close(fd)
            return -1
        }
        // Read handshake reply ("ok\n") before sending binary frame packets.
        var reply = [UInt8]()
        var ch: UInt8 = 0
        while reply.count < 64 {
            let n = Darwin.read(fd, &ch, 1)
            if n <= 0 {
                Darwin.close(fd)
                return -1
            }
            if ch == 0x0A { break }
            reply.append(ch)
        }
        guard String(bytes: reply, encoding: .utf8)?.hasPrefix("ok") == true else {
            Darwin.close(fd)
            return -1
        }
        feedFd = fd
        return fd
    }

    private func writeAll(_ fd: Int32, _ bytes: [UInt8]) -> Bool {
        return bytes.withUnsafeBytes { raw in
            guard let base = raw.baseAddress else { return true }
            return writeAllRaw(fd, base, raw.count)
        }
    }

    private func writeAllRaw(_ fd: Int32, _ base: UnsafeRawPointer, _ count: Int) -> Bool {
        var sent = 0
        while sent < count {
            let n = Darwin.write(fd, base.advanced(by: sent), count - sent)
            if n <= 0 {
                if n < 0 && errno == EINTR { continue }
                return false
            }
            sent += n
        }
        return true
    }

    private func encodeJPEG(_ image: CIImage, maxBytes: Int) -> Data? {
        for quality in [0.80, 0.50] {
            let opts: [CIImageRepresentationOption: Any] = [
                CIImageRepresentationOption(rawValue: kCGImageDestinationLossyCompressionQuality as String): quality,
            ]
            if let data = ciContext.jpegRepresentation(of: image, colorSpace: colorSpace, options: opts),
               data.count <= maxBytes {
                return data
            }
        }
        return nil
    }

    func captureOutput(_ output: AVCaptureOutput, didOutput sampleBuffer: CMSampleBuffer, from connection: AVCaptureConnection) {
        guard !stopped, !slots.isEmpty,
              let pixelBuffer = CMSampleBufferGetImageBuffer(sampleBuffer) else { return }
        let now = CACurrentMediaTime()
        let src = CIImage(cvPixelBuffer: pixelBuffer)
        let srcExtent = src.extent
        guard srcExtent.width > 0, srcExtent.height > 0 else { return }

        for (index, spec) in slots {
            if let prev = lastSent[index], now - prev < (0.85 / Double(spec.fps)) {
                continue
            }
            let fd = ensureFeedSocketLocked()
            guard fd >= 0 else { return }

            let dstW = CGFloat(spec.width)
            let dstH = CGFloat(spec.height)
            let scale = max(dstW / srcExtent.width, dstH / srcExtent.height)
            let scaled = src.transformed(
                by: CGAffineTransform(translationX: -srcExtent.origin.x, y: -srcExtent.origin.y)
                    .concatenating(CGAffineTransform(scaleX: scale, y: scale))
            )
            let cropX = ((scaled.extent.width - dstW) / 2.0).rounded()
            let cropY = ((scaled.extent.height - dstH) / 2.0).rounded()
            let cropped = scaled
                .cropped(to: CGRect(x: cropX, y: cropY, width: dstW, height: dstH))
                .transformed(by: CGAffineTransform(translationX: -cropX, y: -cropY))

            guard let jpeg = encodeJPEG(cropped, maxBytes: spec.width * spec.height * 3 / 2) else {
                continue
            }

            var header = [UInt8](repeating: 0, count: 16)
            let words: [UInt32] = [
                UInt32(index).littleEndian,
                UInt32(spec.width).littleEndian,
                UInt32(spec.height).littleEndian,
                UInt32(jpeg.count).littleEndian,
            ]
            words.withUnsafeBytes { srcBytes in
                header.withUnsafeMutableBytes { dstBytes in
                    dstBytes.copyMemory(from: srcBytes)
                }
            }
            let wroteHeader = writeAll(fd, header)
            let wroteBody = wroteHeader && jpeg.withUnsafeBytes { raw in
                guard let base = raw.baseAddress else { return true }
                return writeAllRaw(fd, base, raw.count)
            }
            if !wroteBody {
                closeFeedLocked()
                return
            }
            lastSent[index] = now
        }
    }
}

final class MetalRenderer {
    let device: MTLDevice
    let commandQueue: MTLCommandQueue
    let pipeline: MTLRenderPipelineState
    private var texture: MTLTexture?
    private var texW: Int = 0
    private var texH: Int = 0
    private var texFmt: MTLPixelFormat = .rgba8Unorm
    private var lastCmdBuf: MTLCommandBuffer?

    func waitIdle() {
        lastCmdBuf?.waitUntilCompleted()
        lastCmdBuf = nil
    }

    init?() {
        guard let dev = MTLCreateSystemDefaultDevice(),
              let q = dev.makeCommandQueue(),
              let lib = try? dev.makeLibrary(source: kMetalShaderSource, options: nil),
              let vs = lib.makeFunction(name: "fullscreen_vs"),
              let fs = lib.makeFunction(name: "fullscreen_fs") else {
            return nil
        }
        let desc = MTLRenderPipelineDescriptor()
        desc.vertexFunction = vs
        desc.fragmentFunction = fs
        desc.colorAttachments[0].pixelFormat = .bgra8Unorm
        guard let pso = try? dev.makeRenderPipelineState(descriptor: desc) else { return nil }
        self.device = dev
        self.commandQueue = q
        self.pipeline = pso
    }

    func ensureTexture(width: Int, height: Int, format: UInt32) -> MTLTexture? {
        let fmt: MTLPixelFormat = (format == 0) ? .bgra8Unorm : .rgba8Unorm
        if let t = texture, texW == width, texH == height, texFmt == fmt {
            return t
        }
        let td = MTLTextureDescriptor.texture2DDescriptor(pixelFormat: fmt, width: width, height: height, mipmapped: false)
        td.usage = [.shaderRead]
        td.storageMode = .private
        guard let t = device.makeTexture(descriptor: td) else { return nil }
        texture = t
        texW = width
        texH = height
        texFmt = fmt
        return t
    }

    func render(fromShmBuffer shmBuf: MTLBuffer?, rawPtr: UnsafeRawPointer, offset: Int,
                width: Int, height: Int, stride: Int, bufSize: Int, format: UInt32,
                to layer: CAMetalLayer) -> Bool {
        guard let drawable = layer.nextDrawable(),
              let cmdBuf = commandQueue.makeCommandBuffer() else { return false }

        let targetTex: MTLTexture
        if let shmBuf, let privTex = ensureTexture(width: width, height: height, format: format),
           let blit = cmdBuf.makeBlitCommandEncoder() {
            blit.copy(from: shmBuf,
                      sourceOffset: offset,
                      sourceBytesPerRow: stride,
                      sourceBytesPerImage: bufSize,
                      sourceSize: MTLSize(width: width, height: height, depth: 1),
                      to: privTex,
                      destinationSlice: 0,
                      destinationLevel: 0,
                      destinationOrigin: MTLOrigin(x: 0, y: 0, z: 0))
            blit.endEncoding()
            targetTex = privTex
        } else {
            // Fallback when bytesNoCopy is unavailable: shared texture replace (still zero Swift Data/CGImage copy).
            let fmt: MTLPixelFormat = (format == 0) ? .bgra8Unorm : .rgba8Unorm
            let td = MTLTextureDescriptor.texture2DDescriptor(pixelFormat: fmt, width: width, height: height, mipmapped: false)
            td.usage = [.shaderRead]
            td.storageMode = .shared
            guard let sharedTex = device.makeTexture(descriptor: td) else { return false }
            sharedTex.replace(region: MTLRegionMake2D(0, 0, width, height),
                              mipmapLevel: 0,
                              withBytes: rawPtr.advanced(by: offset),
                              bytesPerRow: stride)
            targetTex = sharedTex
        }

        let rp = MTLRenderPassDescriptor()
        rp.colorAttachments[0].texture = drawable.texture
        rp.colorAttachments[0].loadAction = .dontCare
        rp.colorAttachments[0].storeAction = .store
        guard let enc = cmdBuf.makeRenderCommandEncoder(descriptor: rp) else { return false }
        enc.setRenderPipelineState(pipeline)
        enc.setFragmentTexture(targetTex, index: 0)
        enc.drawPrimitives(type: .triangle, vertexStart: 0, vertexCount: 3)
        enc.endEncoding()

        cmdBuf.present(drawable)
        cmdBuf.commit()
        lastCmdBuf = cmdBuf
        return true
    }
}

final class ShmSurface {
    private let lock = NSLock()
    private var ptr: UnsafeMutableRawPointer?
    private var mapLen: Int = 0
    private var shmBuffer: MTLBuffer?
    private(set) var width: Int = 0
    private(set) var height: Int = 0
    private var stride: Int = 0
    private var format: UInt32 = 1
    private var bufOffset: Int = 16384
    private var bufSize: Int = 0
    private var lastSeq: UInt64 = 0
    private let colorSpace = CGColorSpaceCreateDeviceRGB()

    deinit { unmapLocked() }

    private func unmapLocked() {
        shmBuffer = nil
        if let p = ptr {
            Darwin.munmap(p, mapLen)
            ptr = nil
            mapLen = 0
        }
        lastSeq = 0
    }

    func open(name: String, width: Int, height: Int, stride: Int, format: UInt32, device: MTLDevice?) -> Bool {
        lock.lock()
        defer { lock.unlock() }
        unmapLocked()
        let fd = name.withCString { c_shm_open($0, O_RDONLY, 0) }
        guard fd >= 0 else { return false }
        defer { Darwin.close(fd) }
        var st = Darwin.stat()
        guard Darwin.fstat(fd, &st) == 0, st.st_size >= 16384 else { return false }
        let len = Int(st.st_size)
        let mapped = Darwin.mmap(nil, len, PROT_READ, MAP_SHARED, fd, 0)
        guard mapped != MAP_FAILED, let base = mapped else { return false }
        self.ptr = base
        self.mapLen = len
        self.width = width
        self.height = height
        self.stride = stride
        self.format = format
        self.bufOffset = Int(base.loadUnaligned(fromByteOffset: 32, as: UInt32.self))
        self.bufSize = Int(base.loadUnaligned(fromByteOffset: 36, as: UInt32.self))
        if let device {
            // Apple Silicon SHM is 16 KiB page-aligned (frames.rs), enabling zero-copy Metal import.
            self.shmBuffer = device.makeBuffer(bytesNoCopy: base, length: len, options: .storageModeShared, deallocator: nil)
        }
        return true
    }

    func renderLatestMetal(renderer: MetalRenderer, layer: CAMetalLayer, force: Bool = false) -> Bool {
        lock.lock()
        defer { lock.unlock() }
        guard let base = ptr, width > 0, height > 0, bufSize > 0 else { return false }
        OSMemoryBarrier()
        let seq1 = base.loadUnaligned(fromByteOffset: 48, as: UInt64.self)
        guard seq1 > 0, (force || seq1 != lastSeq) else { return false }
        let idx = Int(base.loadUnaligned(fromByteOffset: 40, as: UInt32.self))
        guard idx >= 0 && idx < 3 else { return false }
        let off = bufOffset + idx * bufSize
        let bytesNeeded = stride * height
        guard off + bytesNeeded <= mapLen else { return false }

        let ok = renderer.render(fromShmBuffer: shmBuffer, rawPtr: base, offset: off,
                                 width: width, height: height, stride: stride, bufSize: bufSize,
                                 format: format, to: layer)
        OSMemoryBarrier()
        let seq2 = base.loadUnaligned(fromByteOffset: 48, as: UInt64.self)
        if ok && seq2 <= seq1 + 1 {
            lastSeq = seq1
            return true
        }
        return false
    }

    func latestCGImage(ignoreSeq: Bool = false) -> CGImage? {
        lock.lock()
        defer { lock.unlock() }
        guard let base = ptr, width > 0, height > 0, bufSize > 0 else { return nil }
        OSMemoryBarrier()
        let seq1 = base.loadUnaligned(fromByteOffset: 48, as: UInt64.self)
        guard seq1 > 0, (ignoreSeq || seq1 != lastSeq) else { return nil }
        let idx = Int(base.loadUnaligned(fromByteOffset: 40, as: UInt32.self))
        guard idx >= 0 && idx < 3 else { return nil }
        let off = bufOffset + idx * bufSize
        let bytesNeeded = stride * height
        guard off + bytesNeeded <= mapLen else { return nil }

        let data = Data(bytes: base.advanced(by: off), count: bytesNeeded)
        OSMemoryBarrier()
        let seq2 = base.loadUnaligned(fromByteOffset: 48, as: UInt64.self)
        if !ignoreSeq && seq2 > seq1 + 1 { return nil }
        if !ignoreSeq { lastSeq = seq1 }

        guard let provider = CGDataProvider(data: data as CFData) else { return nil }
        let info: CGBitmapInfo = (format == 0)
            ? CGBitmapInfo(rawValue: CGImageAlphaInfo.noneSkipFirst.rawValue | CGBitmapInfo.byteOrder32Little.rawValue)
            : CGBitmapInfo(rawValue: CGImageAlphaInfo.noneSkipLast.rawValue | CGBitmapInfo.byteOrder32Big.rawValue)

        return CGImage(
            width: width,
            height: height,
            bitsPerComponent: 8,
            bitsPerPixel: 32,
            bytesPerRow: stride,
            space: colorSpace,
            bitmapInfo: info,
            provider: provider,
            decode: nil,
            shouldInterpolate: true,
            intent: .defaultIntent
        )
    }

    func dimensions() -> (Int, Int) {
        lock.lock()
        defer { lock.unlock() }
        return (width, height)
    }
}

final class ViewportView: NSView {
    private let client: ControlClient
    private let adb: ADBHelper
    private let onToast: (String) -> Void
    private let onShortcut: (String) -> Void
    let metalLayer: CAMetalLayer?
    private var touching = false
    private var wheelTimer: DispatchWorkItem?
    private var wheelX: Int = 16384
    private var wheelY: Int = 16384
    private var wheelActive = false

    init(frame: NSRect, client: ControlClient, adb: ADBHelper, metalDevice: MTLDevice?,
         onToast: @escaping (String) -> Void, onShortcut: @escaping (String) -> Void) {
        self.client = client
        self.adb = adb
        self.onToast = onToast
        self.onShortcut = onShortcut
        if let dev = metalDevice {
            let ml = CAMetalLayer()
            ml.device = dev
            ml.pixelFormat = .bgra8Unorm
            ml.framebufferOnly = true
            ml.displaySyncEnabled = true
            ml.maximumDrawableCount = 3
            ml.backgroundColor = NSColor.black.cgColor
            self.metalLayer = ml
        } else {
            self.metalLayer = nil
        }
        super.init(frame: frame)
        wantsLayer = true
        if let ml = self.metalLayer {
            self.layer = ml
        } else {
            layer?.backgroundColor = NSColor.black.cgColor
            layer?.contentsGravity = .resizeAspect
        }
        registerForDraggedTypes([.fileURL])
    }

    required init?(coder: NSCoder) { fatalError() }

    override var acceptsFirstResponder: Bool { true }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        updateDrawableSize()
    }

    override func setFrameSize(_ newSize: NSSize) {
        super.setFrameSize(newSize)
        updateDrawableSize()
    }

    override func viewDidChangeBackingProperties() {
        super.viewDidChangeBackingProperties()
        updateDrawableSize()
    }

    func updateDrawableSize() {
        guard let ml = metalLayer else { return }
        let scale = window?.backingScaleFactor ?? NSScreen.main?.backingScaleFactor ?? 2.0
        ml.contentsScale = scale
        let pw = max(1, (bounds.width * scale).rounded())
        let ph = max(1, (bounds.height * scale).rounded())
        if ml.drawableSize.width != pw || ml.drawableSize.height != ph {
            ml.drawableSize = CGSize(width: pw, height: ph)
        }
    }

    private func touchCoords(for event: NSEvent) -> (Int, Int) {
        let pt = convert(event.locationInWindow, from: nil)
        let w = max(bounds.width, 1)
        let h = max(bounds.height, 1)
        let nx = max(0.0, min(1.0, pt.x / w))
        let ny = max(0.0, min(1.0, 1.0 - (pt.y / h)))
        return (Int(nx * 32767.0), Int(ny * 32767.0))
    }

    override func mouseDown(with event: NSEvent) {
        window?.makeFirstResponder(self)
        touching = true
        let (tx, ty) = touchCoords(for: event)
        client.send("touch 0 1:\(tx):\(ty)")
    }

    override func mouseDragged(with event: NSEvent) {
        guard touching else { return }
        let (tx, ty) = touchCoords(for: event)
        client.send("touch 0 1:\(tx):\(ty)")
    }

    override func mouseUp(with event: NSEvent) {
        guard touching else { return }
        touching = false
        client.send("touch 0")
    }

    override func rightMouseDown(with event: NSEvent) {
        client.sendBatch(["key 158 1", "key 158 0"]) // KEY_BACK
    }

    override func scrollWheel(with event: NSEvent) {
        let dy = event.scrollingDeltaY
        if abs(dy) < 0.1 { return }
        if !wheelActive {
            wheelActive = true
            let (tx, ty) = touchCoords(for: event)
            wheelX = tx
            wheelY = ty
            client.send("touch 0 2:\(wheelX):\(wheelY)")
        }
        let step = Int(dy * (event.hasPreciseScrollingDeltas ? 24.0 : 120.0))
        wheelY = max(0, min(32767, wheelY + step))
        client.send("touch 0 2:\(wheelX):\(wheelY)")
        wheelTimer?.cancel()
        let work = DispatchWorkItem { [weak self] in
            guard let self = self, self.wheelActive else { return }
            self.wheelActive = false
            self.client.send("touch 0")
        }
        wheelTimer = work
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.12, execute: work)
    }

    override func keyDown(with event: NSEvent) {
        let mods = event.modifierFlags.intersection(.deviceIndependentFlagsMask)
        if mods.contains(.command) {
            switch event.charactersIgnoringModifiers?.lowercased() {
            case "h":
                client.sendBatch(["key 172 1", "key 172 0"]) // KEY_HOMEPAGE -> Android Home
                return
            case "[":
                client.sendBatch(["key 158 1", "key 158 0"]) // KEY_BACK
                return
            case "v":
                pasteClipboard()
                return
            case "s":
                onShortcut("screenshot")
                return
            case "r":
                onShortcut("rotate")
                return
            case "a": sendModifiedKey(mod: 29, code: 30); return // Ctrl+A
            case "c": sendModifiedKey(mod: 29, code: 46); return // Ctrl+C
            case "x": sendModifiedKey(mod: 29, code: 45); return // Ctrl+X
            case "z": sendModifiedKey(mod: 29, code: 44); return // Ctrl+Z
            default:
                return
            }
        }
        if let special = specialKeyCode(event.keyCode) {
            client.sendBatch(["key \(special) 1", "key \(special) 0"])
            return
        }
        if let chars = event.characters, !chars.isEmpty {
            var batch: [String] = []
            for ch in chars {
                if let (code, shift) = strokeForCharacter(ch) {
                    if shift { batch.append("key 42 1") }
                    batch.append("key \(code) 1")
                    batch.append("key \(code) 0")
                    if shift { batch.append("key 42 0") }
                }
            }
            if !batch.isEmpty {
                client.sendBatch(batch)
                return
            }
        }
        if let ch = event.charactersIgnoringModifiers?.lowercased().first,
           let (code, _) = strokeForCharacter(ch) {
            let shift = mods.contains(.shift)
            if shift {
                sendModifiedKey(mod: 42, code: code)
            } else {
                client.sendBatch(["key \(code) 1", "key \(code) 0"])
            }
        }
    }

    private func sendModifiedKey(mod: Int, code: Int) {
        client.sendBatch(["key \(mod) 1", "key \(code) 1", "key \(code) 0", "key \(mod) 0"])
    }

    private func pasteClipboard() {
        guard let text = NSPasteboard.general.string(forType: .string), !text.isEmpty else { return }
        // Pure printable ASCII (<= 240 chars): type directly via evdev (instant, works in every app/WebView).
        if text.count <= 240 {
            var batch: [String] = []
            var allASCII = true
            for ch in text {
                if let (code, shift) = strokeForCharacter(ch) {
                    if shift { batch.append("key 42 1") }
                    batch.append("key \(code) 1")
                    batch.append("key \(code) 0")
                    if shift { batch.append("key 42 0") }
                } else {
                    allASCII = false
                    break
                }
            }
            if allASCII && !batch.isEmpty {
                client.sendBatch(batch)
                return
            }
        }
        // Fallback for longer or Unicode/Vietnamese text: adb shell input text (escaped for Android shell).
        guard adb.available else { return }
        let singleLine = text.replacingOccurrences(of: "\r\n", with: " ").replacingOccurrences(of: "\n", with: " ")
        let escaped = singleLine
            .replacingOccurrences(of: "\\", with: "\\\\")
            .replacingOccurrences(of: "'", with: "'\\''")
            .replacingOccurrences(of: " ", with: "%s")
        adb.run(["shell", "input text '\(escaped)'"])
    }

    // MARK: - Drag and Drop (APK install & media push)

    override func draggingEntered(_ sender: NSDraggingInfo) -> NSDragOperation {
        return adb.available ? .copy : []
    }

    override func performDragOperation(_ sender: NSDraggingInfo) -> Bool {
        guard adb.available,
              let urls = sender.draggingPasteboard.readObjects(forClasses: [NSURL.self], options: [.urlReadingFileURLsOnly: true]) as? [URL],
              !urls.isEmpty else { return false }

        let mediaExts: Set<String> = ["jpg", "jpeg", "png", "webp", "gif", "mp4", "mov", "mkv", "webm"]
        let bundleExts: Set<String> = ["apks", "xapk", "apkm"]
        for url in urls {
            let ext = url.pathExtension.lowercased()
            let name = url.lastPathComponent
            if ext == "apk" {
                onToast("Installing \(name)...")
                adb.run(["install", "-r", url.path]) { [onToast] ok, out in
                    if ok {
                        onToast("Installed \(name)")
                    } else if out.contains("NO_MATCHING_ABIS") {
                        onToast("\(name) is not made for ARM phones")
                    } else {
                        onToast("Install failed: \(out.prefix(60))")
                    }
                }
            } else if bundleExts.contains(ext) {
                installSplitBundle(url: url, name: name)
            } else if mediaExts.contains(ext) {
                let remote = "/sdcard/DCIM/Camera/\(name)"
                onToast("Sending \(name)...")
                adb.run(["push", url.path, remote]) { [adb, onToast] ok, _ in
                    if ok {
                        let safeRemote = remote.replacingOccurrences(of: "'", with: "'\\''")
                        adb.run(["shell", "am broadcast -a android.intent.action.MEDIA_SCANNER_SCAN_FILE -d 'file://\(safeRemote)'"])
                        onToast("Saved to Gallery: \(name)")
                    } else {
                        onToast("Failed to send \(name)")
                    }
                }
            } else {
                let remote = "/sdcard/Download/\(name)"
                onToast("Sending \(name)...")
                adb.run(["push", url.path, remote]) { [onToast] ok, _ in
                    onToast(ok ? "Saved to Downloads: \(name)" : "Failed to send \(name)")
                }
            }
        }
        return true
    }

    private func installSplitBundle(url: URL, name: String) {
        onToast("Extracting \(name)...")
        DispatchQueue.global(qos: .userInitiated).async { [adb, onToast] in
            let fm = FileManager.default
            let tmp = fm.temporaryDirectory.appendingPathComponent("anyprofile-apks-\(UUID().uuidString)")
            do {
                try fm.createDirectory(at: tmp, withIntermediateDirectories: true)
            } catch {
                DispatchQueue.main.async { onToast("Extract failed: \(name)") }
                return
            }
            let unzip = Process()
            unzip.executableURL = URL(fileURLWithPath: "/usr/bin/unzip")
            unzip.arguments = ["-q", "-o", url.path, "-d", tmp.path]
            unzip.standardOutput = Pipe()
            unzip.standardError = Pipe()
            do {
                try unzip.run()
                unzip.waitUntilExit()
            } catch {
                try? fm.removeItem(at: tmp)
                DispatchQueue.main.async { onToast("Extract failed: \(name)") }
                return
            }
            guard unzip.terminationStatus == 0 else {
                try? fm.removeItem(at: tmp)
                DispatchQueue.main.async { onToast("Corrupted or encrypted bundle: \(name)") }
                return
            }

            var apks: [String] = []
            var hasOBB = false
            if let en = fm.enumerator(at: tmp, includingPropertiesForKeys: [.isRegularFileKey]) {
                for case let f as URL in en {
                    let rel = f.path.replacingOccurrences(of: tmp.path + "/", with: "")
                    if rel.hasPrefix("__MACOSX/") || f.lastPathComponent.hasPrefix(".") { continue }
                    let fext = f.pathExtension.lowercased()
                    if fext == "obb" { hasOBB = true; break }
                    if fext == "apk" { apks.append(f.path) }
                }
            }
            if hasOBB {
                try? fm.removeItem(at: tmp)
                DispatchQueue.main.async { onToast("Bundles with .obb are not supported") }
                return
            }
            apks.sort()
            if apks.isEmpty || apks.count > 64 {
                try? fm.removeItem(at: tmp)
                DispatchQueue.main.async { onToast(apks.isEmpty ? "No .apk files in \(name)" : "Too many split APKs in \(name)") }
                return
            }
            DispatchQueue.main.async { onToast("Installing \(name) (\(apks.count) splits)...") }
            adb.run(["install-multiple", "-r"] + apks) { ok, out in
                try? fm.removeItem(at: tmp)
                if ok {
                    onToast("Installed \(name)")
                } else if out.contains("NO_MATCHING_ABIS") {
                    onToast("\(name) is not made for ARM phones")
                } else {
                    onToast("Install failed: \(out.prefix(60))")
                }
            }
        }
    }
}

func specialKeyCode(_ keyCode: UInt16) -> Int? {
    switch keyCode {
    case 36, 76: return 28  // Return -> KEY_ENTER
    case 51: return 14      // Backspace -> KEY_BACKSPACE
    case 117: return 111    // Forward Delete -> KEY_DELETE
    case 53: return 158     // Escape -> KEY_BACK
    case 48: return 15      // Tab -> KEY_TAB
    case 49: return 57      // Space -> KEY_SPACE
    case 115: return 102    // Home -> KEY_HOME (cursor start)
    case 119: return 107    // End -> KEY_END
    case 116: return 104    // PageUp -> KEY_PAGEUP
    case 121: return 109    // PageDown -> KEY_PAGEDOWN
    case 123: return 105    // Left
    case 124: return 106    // Right
    case 125: return 108    // Down
    case 126: return 103    // Up
    default: return nil
    }
}

func strokeForCharacter(_ ch: Character) -> (Int, Bool)? {
    let lower: [Character: Int] = [
        "a": 30, "b": 48, "c": 46, "d": 32, "e": 18, "f": 33, "g": 34, "h": 35, "i": 23,
        "j": 36, "k": 37, "l": 38, "m": 50, "n": 49, "o": 24, "p": 25, "q": 16, "r": 19,
        "s": 31, "t": 20, "u": 22, "v": 47, "w": 17, "x": 45, "y": 21, "z": 44,
    ]
    if let code = lower[ch] { return (code, false) }
    if let s = String(ch).lowercased().first, ch.isUppercase, let code = lower[s] {
        return (code, true)
    }
    let plain: [Character: Int] = [
        "1": 2, "2": 3, "3": 4, "4": 5, "5": 6, "6": 7, "7": 8, "8": 9, "9": 10, "0": 11,
        "-": 12, "=": 13, "[": 26, "]": 27, ";": 39, "'": 40, "`": 41, "\\": 43,
        ",": 51, ".": 52, "/": 53, " ": 57, "\n": 28, "\t": 15,
    ]
    if let code = plain[ch] { return (code, false) }
    let shifted: [Character: Int] = [
        "!": 2, "@": 3, "#": 4, "$": 5, "%": 6, "^": 7, "&": 8, "*": 9, "(": 10, ")": 11,
        "_": 12, "+": 13, "{": 26, "}": 27, ":": 39, "\"": 40, "~": 41, "|": 43,
        "<": 51, ">": 52, "?": 53,
    ]
    if let code = shifted[ch] { return (code, true) }
    return nil
}

final class RailButton: NSButton {
    private var tracking: NSTrackingArea?
    private var hovered = false
    var activeTint: NSColor? {
        didSet { updateStyle() }
    }

    init(symbol: String, tooltip: String, target: AnyObject, action: Selector) {
        super.init(frame: NSRect(x: 0, y: 0, width: 28, height: 28))
        isBordered = false
        wantsLayer = true
        layer?.cornerRadius = 6
        toolTip = tooltip
        self.target = target
        self.action = action
        setSymbol(symbol)
        updateStyle()
    }

    required init?(coder: NSCoder) { fatalError() }

    func setSymbol(_ name: String) {
        let cfg = NSImage.SymbolConfiguration(pointSize: 13, weight: .medium)
        let img = NSImage(systemSymbolName: name, accessibilityDescription: toolTip)?.withSymbolConfiguration(cfg)
        image = img
        imagePosition = .imageOnly
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let t = tracking { removeTrackingArea(t) }
        tracking = NSTrackingArea(rect: bounds, options: [.mouseEnteredAndExited, .activeAlways], owner: self, userInfo: nil)
        addTrackingArea(tracking!)
    }

    override func mouseEntered(with event: NSEvent) { hovered = true; updateStyle() }
    override func mouseExited(with event: NSEvent) { hovered = false; updateStyle() }

    private func updateStyle() {
        if let tint = activeTint {
            contentTintColor = tint
            layer?.backgroundColor = tint.withAlphaComponent(0.18).cgColor
        } else {
            contentTintColor = hovered ? .white : NSColor(white: 0.76, alpha: 1.0)
            layer?.backgroundColor = hovered ? NSColor(white: 1.0, alpha: 0.10).cgColor : NSColor.clear.cgColor
        }
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate, NSWindowDelegate {
    private let controlPath: String
    private let titleText: String
    private let initialRect: NSRect
    private var onTop: Bool
    private var muted: Bool
    private let appMode: Bool
    private let ownerPID: pid_t
    private let noFocus: Bool
    private var minimized: Bool = false
    private var closingFromSignal: Bool = false
    private var termSource: DispatchSourceSignal?
    private let boundsFile: String
    private let adb: ADBHelper
    private let client: ControlClient
    private let metalRenderer = MetalRenderer()
    private lazy var cameraFeeder = CameraFeeder(controlPath: controlPath, metalDevice: metalRenderer?.device) { [weak self] msg in
        self?.showToast(msg)
    }

    private var toolbarWidth: CGFloat { appMode ? 0 : kToolbarWidth }

    private var window: NSWindow!
    private var viewport: ViewportView!
    private var rail: NSView!
    private var toastLabel: NSTextField!
    private var toastTimer: DispatchWorkItem?
    private var pinBtn: RailButton!
    private var muteBtn: RailButton!

    private let surface = ShmSurface()
    private let renderQueue = DispatchQueue(label: "anyprofile.view.render", qos: .userInteractive)
    private let pendingLock = NSLock()
    private var framePending = false
    private var aspectRatio: CGFloat
    private var guestW: Int = 0
    private var guestH: Int = 0
    private var landscape = false

    init(controlPath: String, title: String, rect: NSRect, onTop: Bool, muted: Bool, appMode: Bool, ownerPID: pid_t, noFocus: Bool, boundsFile: String, adbBin: String, adbSerial: String) {
        self.controlPath = controlPath
        self.titleText = title
        self.initialRect = rect
        self.onTop = onTop
        self.muted = muted
        self.appMode = appMode
        self.ownerPID = ownerPID
        self.noFocus = noFocus
        self.boundsFile = boundsFile
        self.adb = ADBHelper(bin: adbBin, serial: adbSerial)
        self.client = ControlClient(path: controlPath)
        self.aspectRatio = max(0.3, min(3.0, rect.width / max(rect.height, 1)))
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        let style: NSWindow.StyleMask = [.titled, .closable, .miniaturizable, .resizable]
        let screenH = NSScreen.main?.frame.height ?? 900
        let totalW = initialRect.width + toolbarWidth
        let cocoaRect = NSRect(x: initialRect.origin.x,
                               y: max(0, screenH - initialRect.origin.y - initialRect.height),
                               width: totalW,
                               height: initialRect.height)
        window = NSWindow(contentRect: cocoaRect, styleMask: style, backing: .buffered, defer: false)
        window.title = titleText
        window.delegate = self
        window.backgroundColor = NSColor(red: 0.06, green: 0.07, blue: 0.09, alpha: 1.0)
        window.appearance = NSAppearance(named: .darkAqua)
        window.minSize = NSSize(width: 240 + toolbarWidth, height: 320)
        // Stay on the current desktop; never trigger a macOS Space switch on show.
        window.collectionBehavior = [.moveToActiveSpace]
        if onTop { window.level = .floating }

        let root = NSView(frame: window.contentLayoutRect)
        root.wantsLayer = true
        root.layer?.backgroundColor = NSColor(red: 0.06, green: 0.07, blue: 0.09, alpha: 1.0).cgColor
        window.contentView = root

        viewport = ViewportView(frame: .zero, client: client, adb: adb, metalDevice: metalRenderer?.device,
                                onToast: { [weak self] msg in self?.showToast(msg) },
                                onShortcut: { [weak self] kind in
                                    if kind == "screenshot" { self?.actionScreenshot() }
                                    else if kind == "rotate" { self?.actionRotate() }
                                })
        root.addSubview(viewport)

        rail = buildRail()
        rail.isHidden = appMode
        root.addSubview(rail)

        toastLabel = NSTextField(labelWithString: "")
        toastLabel.font = .systemFont(ofSize: 11, weight: .medium)
        toastLabel.textColor = .white
        toastLabel.alignment = .center
        toastLabel.wantsLayer = true
        toastLabel.layer?.backgroundColor = NSColor(white: 0.08, alpha: 0.88).cgColor
        toastLabel.layer?.cornerRadius = 11
        toastLabel.layer?.borderWidth = 0.5
        toastLabel.layer?.borderColor = NSColor(white: 1.0, alpha: 0.14).cgColor
        toastLabel.isHidden = true
        root.addSubview(toastLabel)

        layoutSubviews()
        // Mirror Web's focus discipline: raise in front only if the user is still in
        // AnyProfile (or launched directly) and --no-focus was not set. If the user
        // already switched to another app while booting, show quietly without stealing focus.
        let frontPID = NSWorkspace.shared.frontmostApplication?.processIdentifier ?? 0
        let selfPID = ProcessInfo.processInfo.processIdentifier
        let shouldFocus = !noFocus && (ownerPID <= 0 || frontPID == ownerPID || frontPID == selfPID)
        if shouldFocus {
            window.makeKeyAndOrderFront(nil)
            window.makeFirstResponder(viewport)
            if #available(macOS 14.0, *) {
                NSApp.activate()
            } else {
                NSApp.activate(ignoringOtherApps: true)
            }
        } else {
            window.orderFront(nil)
        }
        installSignalHandler()
        client.sendBatch(["resume", "fps 0"])
        startFrameReader()
    }

    private func installSignalHandler() {
        signal(SIGTERM, SIG_IGN)
        let src = DispatchSource.makeSignalSource(signal: SIGTERM, queue: .main)
        src.setEventHandler { [weak self] in
            guard let self else { NSApp.terminate(nil); return }
            self.closingFromSignal = true
            self.cameraFeeder.stopAll()
            self.passFocusToCurrentDesktop()
            self.saveBounds()
            self.window?.orderOut(nil)
            NSApp.terminate(nil)
        }
        src.resume()
        termSource = src
    }

    // Mirrors internal/launcher/focus_darwin.go passFocus(): when an accessory window
    // closes while focused, activate the topmost visible regular app on the SAME desktop
    // so macOS never jumps to another Space or random background app.
    private func passFocusToCurrentDesktop() {
        let selfPID = ProcessInfo.processInfo.processIdentifier
        let frontPID = NSWorkspace.shared.frontmostApplication?.processIdentifier ?? 0
        guard frontPID == selfPID || window?.isKeyWindow == true else { return }
        guard let list = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] else {
            return
        }
        for w in list {
            guard let layer = w[kCGWindowLayer as String] as? Int, layer == 0 else { continue }
            if let alpha = w[kCGWindowAlpha as String] as? Double, alpha <= 0 { continue }
            guard let pid = w[kCGWindowOwnerPID as String] as? pid_t, pid != selfPID else { continue }
            guard let app = NSRunningApplication(processIdentifier: pid), app.activationPolicy == .regular else { continue }
            app.activate(options: [])
            return
        }
    }

    private func buildRail() -> NSView {
        let v = NSView(frame: .zero)
        v.wantsLayer = true
        v.layer?.backgroundColor = NSColor(red: 0.085, green: 0.095, blue: 0.115, alpha: 1.0).cgColor

        let sep = NSView(frame: .zero)
        sep.wantsLayer = true
        sep.layer?.backgroundColor = NSColor(white: 1.0, alpha: 0.08).cgColor
        sep.translatesAutoresizingMaskIntoConstraints = false
        v.addSubview(sep)
        NSLayoutConstraint.activate([
            sep.leadingAnchor.constraint(equalTo: v.leadingAnchor),
            sep.topAnchor.constraint(equalTo: v.topAnchor),
            sep.bottomAnchor.constraint(equalTo: v.bottomAnchor),
            sep.widthAnchor.constraint(equalToConstant: 1),
        ])

        let stack = NSStackView()
        stack.orientation = .vertical
        stack.alignment = .centerX
        stack.spacing = 6
        stack.translatesAutoresizingMaskIntoConstraints = false
        v.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: v.topAnchor, constant: 10),
            stack.leadingAnchor.constraint(equalTo: v.leadingAnchor, constant: 4),
            stack.trailingAnchor.constraint(equalTo: v.trailingAnchor, constant: -4),
        ])

        let addBtn = { (sym: String, tip: String, sel: Selector) -> RailButton in
            let b = RailButton(symbol: sym, tooltip: tip, target: self, action: sel)
            b.translatesAutoresizingMaskIntoConstraints = false
            NSLayoutConstraint.activate([b.widthAnchor.constraint(equalToConstant: 28), b.heightAnchor.constraint(equalToConstant: 28)])
            stack.addArrangedSubview(b)
            return b
        }

        _ = addBtn("chevron.backward", "Back (Esc / Right-click)", #selector(actionBack))
        _ = addBtn("circle", "Home (Cmd+H)", #selector(actionHome))
        _ = addBtn("square.on.square", "Recent Apps", #selector(actionRecents))

        let div1 = makeDivider()
        stack.addArrangedSubview(div1)

        _ = addBtn("rotate.right", "Rotate Screen (Cmd+R)", #selector(actionRotate))
        _ = addBtn("camera", "Screenshot to Desktop (Cmd+S)", #selector(actionScreenshot))

        let div2 = makeDivider()
        stack.addArrangedSubview(div2)

        _ = addBtn("speaker.plus", "Volume Up", #selector(actionVolUp))
        _ = addBtn("speaker.minus", "Volume Down", #selector(actionVolDown))
        muteBtn = addBtn(muted ? "speaker.slash" : "speaker.wave.2", "Mute / Unmute", #selector(actionToggleMute))
        if muted { muteBtn.activeTint = NSColor.systemRed }

        let div3 = makeDivider()
        stack.addArrangedSubview(div3)

        pinBtn = addBtn(onTop ? "pin.fill" : "pin", "Always on Top", #selector(actionTogglePin))
        if onTop { pinBtn.activeTint = NSColor.systemBlue }

        return v
    }

    private func makeDivider() -> NSView {
        let d = NSView()
        d.wantsLayer = true
        d.layer?.backgroundColor = NSColor(white: 1.0, alpha: 0.08).cgColor
        d.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([d.widthAnchor.constraint(equalToConstant: 18), d.heightAnchor.constraint(equalToConstant: 1)])
        return d
    }

    private func layoutSubviews() {
        guard let root = window?.contentView else { return }
        let b = root.bounds
        let vw = max(1, b.width - toolbarWidth)
        viewport.frame = NSRect(x: 0, y: 0, width: vw, height: b.height)
        rail.frame = NSRect(x: vw, y: 0, width: toolbarWidth, height: b.height)
        positionToast()
    }

    private func positionToast() {
        guard let toast = toastLabel, !toast.isHidden else { return }
        let sz = toast.fittingSize
        let pw = min(viewport.bounds.width - 24, sz.width + 22)
        let ph: CGFloat = 24
        toast.frame = NSRect(x: (viewport.bounds.width - pw) / 2,
                             y: viewport.bounds.height - ph - 14,
                             width: pw, height: ph)
    }

    private func showToast(_ text: String) {
        DispatchQueue.main.async { [weak self] in
            guard let self else { return }
            self.toastLabel.stringValue = text
            self.toastLabel.isHidden = false
            self.positionToast()
            self.toastTimer?.cancel()
            let work = DispatchWorkItem { [weak self] in
                self?.toastLabel.isHidden = true
            }
            self.toastTimer = work
            DispatchQueue.main.asyncAfter(deadline: .now() + 2.2, execute: work)
        }
    }

    // MARK: - Rail Actions

    @objc private func actionBack()    { client.sendBatch(["key 158 1", "key 158 0"]) }
    @objc private func actionHome()    { client.sendBatch(["key 172 1", "key 172 0"]) }
    @objc private func actionVolUp()   { client.sendBatch(["key 115 1", "key 115 0"]) }
    @objc private func actionVolDown() { client.sendBatch(["key 114 1", "key 114 0"]) }

    @objc private func actionRecents() {
        if adb.available {
            adb.run(["shell", "input keyevent 187"])
        } else {
            client.sendBatch(["key 139 1", "key 139 0"])
        }
    }

    @objc private func actionRotate() {
        let (w, h) = surface.dimensions()
        if w > 0 && h > 0 {
            landscape.toggle()
            // Resize virtio-gpu scanout directly so apps and window swap aspect cleanly.
            let newW = h
            let newH = w
            client.send("display 0 \(newW) \(newH)")
            showToast(newW > newH ? "Landscape" : "Portrait")
        } else if adb.available {
            landscape.toggle()
            let rot = landscape ? "1" : "0"
            adb.run(["shell", "settings put system accelerometer_rotation 0; settings put system user_rotation \(rot)"])
            showToast(landscape ? "Landscape" : "Portrait")
        }
    }

    @objc private func actionScreenshot() {
        guard let img = surface.latestCGImage(ignoreSeq: true) else {
            showToast("No frame yet")
            return
        }
        let rep = NSBitmapImageRep(cgImage: img)
        guard let png = rep.representation(using: .png, properties: [:]) else { return }
        let df = DateFormatter()
        df.dateFormat = "yyyyMMdd-HHmmss"
        let safeTitle = titleText.replacingOccurrences(of: "/", with: "-")
        let desktop = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("Desktop")
        let outURL = desktop.appendingPathComponent("AnyMobile-\(safeTitle)-\(df.string(from: Date())).png")
        do {
            try png.write(to: outURL)
            showToast("Screenshot saved to Desktop")
        } catch {
            showToast("Screenshot failed")
        }
    }

    @objc private func actionTogglePin() {
        onTop.toggle()
        window.level = onTop ? .floating : .normal
        pinBtn.setSymbol(onTop ? "pin.fill" : "pin")
        pinBtn.activeTint = onTop ? NSColor.systemBlue : nil
        saveBounds()
        showToast(onTop ? "Always on top" : "Unpinned")
    }

    @objc private func actionToggleMute() {
        muted.toggle()
        muteBtn.setSymbol(muted ? "speaker.slash" : "speaker.wave.2")
        muteBtn.activeTint = muted ? NSColor.systemRed : nil
        if adb.available {
            let vol = muted ? "0" : "10"
            adb.run(["shell", "cmd media_session volume --stream 3 --set \(vol)"])
        }
        saveBounds()
        showToast(muted ? "Muted" : "Unmuted")
    }

    // MARK: - Window Delegate & Aspect Lock

    func windowWillResize(_ sender: NSWindow, to frameSize: NSSize) -> NSSize {
        let chrome = sender.frameRect(forContentRect: .zero)
        let contentH = max(320, frameSize.height - chrome.height)
        let viewW = max(200, (contentH * aspectRatio).rounded())
        return NSSize(width: viewW + toolbarWidth + chrome.width, height: contentH + chrome.height)
    }

    func windowDidResize(_ notification: Notification) {
        layoutSubviews()
        scheduleRender(force: true)
    }

    func windowDidMove(_ notification: Notification) { saveBounds() }
    func windowDidEndLiveResize(_ notification: Notification) { saveBounds() }
    func windowDidBecomeKey(_ notification: Notification) {
        if minimized {
            minimized = false
            saveBounds()
        }
        client.sendBatch(["resume", "fps 0"])
    }
    func windowDidResignKey(_ notification: Notification) { client.send("fps 15") }
    func windowDidMiniaturize(_ notification: Notification) {
        minimized = true
        saveBounds()
        client.send("fps 1")
    }
    func windowDidDeminiaturize(_ notification: Notification) {
        minimized = false
        saveBounds()
        client.sendBatch(["resume", "fps 0"])
    }

    private func applyGuestAspect(w: Int, h: Int) {
        guard w > 0, h > 0 else { return }
        let changedOrientation = (guestW > 0 && guestH > 0) && ((w > h) != (guestW > guestH))
        guestW = w
        guestH = h
        aspectRatio = CGFloat(w) / CGFloat(h)
        let curContent = window.contentRect(forFrameRect: window.frame)
        let curViewW = max(200, curContent.width - toolbarWidth)
        let curViewH = max(200, curContent.height)
        let targetH: CGFloat = changedOrientation ? curViewW : curViewH
        let targetW: CGFloat = (targetH * aspectRatio).rounded()
        let vis = window.screen?.visibleFrame ?? NSScreen.main?.visibleFrame ?? NSRect(x: 0, y: 0, width: 1440, height: 900)
        let scale = min(1.0, min((vis.width - 40 - toolbarWidth) / max(targetW, 1), (vis.height - 60) / max(targetH, 1)))
        let finalContentW = (targetW * scale).rounded() + toolbarWidth
        let finalContentH = (targetH * scale).rounded()
        let center = NSPoint(x: window.frame.midX, y: window.frame.midY)
        var newFrame = window.frameRect(forContentRect: NSRect(x: 0, y: 0, width: finalContentW, height: finalContentH))
        newFrame.origin.x = max(vis.minX, min(vis.maxX - newFrame.width, center.x - newFrame.width / 2))
        newFrame.origin.y = max(vis.minY, min(vis.maxY - newFrame.height, center.y - newFrame.height / 2))
        window.setFrame(newFrame, display: true, animate: changedOrientation)
        layoutSubviews()
        scheduleRender(force: true)
    }

    private func saveBounds() {
        guard !boundsFile.isEmpty, let win = window else { return }
        let content = win.contentRect(forFrameRect: win.frame)
        let screenH = win.screen?.frame.height ?? NSScreen.main?.frame.height ?? 900
        let x = Int(content.origin.x.rounded())
        let y = Int(max(0, screenH - content.origin.y - content.height).rounded())
        let w = Int(max(200, content.width - toolbarWidth).rounded())
        let h = Int(content.height.rounded())
        guard w >= 200, h >= 200 else { return }
        let obj: [String: Any] = ["x": x, "y": y, "w": w, "h": h, "onTop": onTop, "muted": muted, "minimized": minimized]
        if let out = try? JSONSerialization.data(withJSONObject: obj) {
            try? out.write(to: URL(fileURLWithPath: boundsFile), options: .atomic)
        }
    }

    func windowWillClose(_ notification: Notification) {
        minimized = false
        cameraFeeder.stopAll()
        if !closingFromSignal {
            if appMode {
                // Standalone APK mode: freeze vCPUs immediately for ~0% idle CPU.
                client.sendSyncNow("pause")
            } else {
                client.sendSyncNow("resume")
                client.sendSyncNow("fps 0")
            }
        }
        passFocusToCurrentDesktop()
        saveBounds()
        window?.orderOut(nil)
        NSApp.terminate(nil)
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }

    // MARK: - Coalesced Zero-Copy Frame Reader

    private var forceNextFrame = false

    private func scheduleRender(force: Bool = false) {
        pendingLock.lock()
        if force { forceNextFrame = true }
        if framePending {
            pendingLock.unlock()
            return
        }
        framePending = true
        pendingLock.unlock()

        renderQueue.async { [weak self] in
            guard let self else { return }
            self.pendingLock.lock()
            self.framePending = false
            let forceNow = self.forceNextFrame
            self.forceNextFrame = false
            self.pendingLock.unlock()

            if let renderer = self.metalRenderer, let ml = self.viewport.metalLayer {
                _ = self.surface.renderLatestMetal(renderer: renderer, layer: ml, force: forceNow)
            } else if let img = self.surface.latestCGImage(ignoreSeq: forceNow) {
                DispatchQueue.main.async {
                    CATransaction.begin()
                    CATransaction.setDisableActions(true)
                    self.viewport.layer?.contents = img
                    CATransaction.commit()
                }
            }
        }
    }

    private func startFrameReader() {
        DispatchQueue.global(qos: .userInteractive).async { [weak self] in
            guard let self else { return }
            let fd = connectUnix(self.controlPath)
            guard fd >= 0 else {
                DispatchQueue.main.async { NSApp.terminate(nil) }
                return
            }
            let sub = Array("subscribe\n".utf8)
            _ = sub.withUnsafeBytes { Darwin.write(fd, $0.baseAddress, $0.count) }

            var buf = Data()
            var tmp = [UInt8](repeating: 0, count: 8192)
            while true {
                let n = Darwin.read(fd, &tmp, tmp.count)
                if n <= 0 { break }
                buf.append(tmp, count: n)
                var hasNewFrame = false
                while let nl = buf.firstIndex(of: 0x0A) {
                    let lineData = buf.subdata(in: 0..<nl)
                    buf.removeSubrange(0...nl)
                    guard let line = String(data: lineData, encoding: .utf8) else { continue }
                    let parts = line.split(separator: " ")
                    if parts.first == "frames", parts.count >= 8, parts[1] == "0" {
                        let w = Int(parts[3]) ?? 0
                        let h = Int(parts[4]) ?? 0
                        let stride = Int(parts[5]) ?? 0
                        let fmt = UInt32(parts[6]) ?? 1
                        let shm = String(parts[7].dropFirst(4)) // strip "shm:"
                        self.renderQueue.sync {
                            self.metalRenderer?.waitIdle()
                            _ = self.surface.open(name: shm, width: w, height: h, stride: stride, format: fmt, device: self.metalRenderer?.device)
                        }
                        if w > 0 && h > 0 {
                            DispatchQueue.main.async {
                                self.applyGuestAspect(w: w, h: h)
                            }
                        }
                        hasNewFrame = true
                    } else if parts.first == "frame", parts.count >= 3, parts[1] == "0" {
                        hasNewFrame = true
                    } else if parts.first == "camera", parts.count >= 3, let camIdx = Int(parts[1]) {
                        if parts[2] == "on", parts.count >= 6,
                           let w = Int(parts[3]), let h = Int(parts[4]), let fps = Int(parts[5]) {
                            self.cameraFeeder.setActive(index: camIdx, width: w, height: h, fps: fps)
                        } else if parts[2] == "off" {
                            self.cameraFeeder.setInactive(index: camIdx)
                        }
                    }
                }
                if hasNewFrame {
                    self.scheduleRender()
                }
            }
            Darwin.close(fd)
            self.cameraFeeder.stopAll()
            DispatchQueue.main.async { NSApp.terminate(nil) }
        }
    }
}

var controlPath = ""
var title = "AnyProfile Mobile"
var winX: Double = 1040
var winY: Double = 80
var winW: Double = 332
var winH: Double = 720
var onTop = false
var muted = false
var appMode = false
var ownerPID: pid_t = 0
var noFocus = false
var boundsFile = ""
var adbBin = ""
var adbSerial = ""

var i = 1
let args = CommandLine.arguments
while i < args.count {
    switch args[i] {
    case "--control":     if i + 1 < args.count { controlPath = args[i + 1]; i += 1 }
    case "--title":       if i + 1 < args.count { title = args[i + 1]; i += 1 }
    case "--x":           if i + 1 < args.count { winX = Double(args[i + 1]) ?? winX; i += 1 }
    case "--y":           if i + 1 < args.count { winY = Double(args[i + 1]) ?? winY; i += 1 }
    case "--w":           if i + 1 < args.count { winW = Double(args[i + 1]) ?? winW; i += 1 }
    case "--h":           if i + 1 < args.count { winH = Double(args[i + 1]) ?? winH; i += 1 }
    case "--bounds-file": if i + 1 < args.count { boundsFile = args[i + 1]; i += 1 }
    case "--adb-bin":     if i + 1 < args.count { adbBin = args[i + 1]; i += 1 }
    case "--adb-serial":  if i + 1 < args.count { adbSerial = args[i + 1]; i += 1 }
    case "--owner-pid":   if i + 1 < args.count { ownerPID = pid_t(Int32(args[i + 1]) ?? 0); i += 1 }
    case "--on-top":      onTop = true
    case "--muted":       muted = true
    case "--app-mode":    appMode = true
    case "--no-focus":    noFocus = true
    default: break
    }
    i += 1
}

guard !controlPath.isEmpty else {
    fputs("usage: anyprofile-view --control <control.sock> [--title TITLE] [--x X --y Y --w W --h H] [--bounds-file PATH] [--adb-bin PATH --adb-serial SERIAL] [--owner-pid PID] [--on-top] [--muted] [--app-mode] [--no-focus]\n", stderr)
    exit(2)
}

signal(SIGPIPE, SIG_IGN)
let app = NSApplication.shared
app.setActivationPolicy(.accessory)
let delegate = AppDelegate(controlPath: controlPath, title: title,
                           rect: NSRect(x: winX, y: winY, width: winW, height: winH),
                           onTop: onTop, muted: muted, appMode: appMode, ownerPID: ownerPID,
                           noFocus: noFocus, boundsFile: boundsFile,
                           adbBin: adbBin, adbSerial: adbSerial)
app.delegate = delegate
app.run()
