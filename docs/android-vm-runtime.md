# Android VM runtime (macOS)

Danh sách mọi thành phần của máy mobile, nguồn và cách cập nhật: `docs/mobile-maintenance.md`.

Máy mobile trên macOS cần hai thứ tải về:

| Engine | Là gì | Ai dựng | App lấy từ đâu |
|---|---|---|---|
| `android-vm` | QEMU, EDK2, scrcpy, adb | AnyProfile, từ mã nguồn (`android-vm/` trong repo riêng AnyProfile Pro) | Máy chủ license, chỉ cho key còn quyền dùng Pro |
| `android-vm-image` | LineageOS cho QEMU (gói UTM) | jqssun/android-lineage-qemu | Bản phát hành của jqssun, qua `engines.json`; AnyProfile không lưu trữ |

Bản image dùng cho cài mới ghim trong `vm.ImageBuiltin()` (`internal/mobile/vm/install.go`).

## Runtime đến cùng Pro

- Mỗi lần app kiểm key (`/v1/pack`), máy chủ trả thêm `runtimes`: bản mới nhất cho nền tảng của máy, kèm link tải
  hết hạn sau 10 phút. Key hết hạn hoặc bị thu hồi thì danh sách rỗng.
- Bản ghi được ký bằng khoá Pro. Thông điệp là `anyprofile-runtime\n<engine>\n<nền tảng>\n<phiên bản>\n<sha256>`,
  khác với pack và bản cập nhật app (`internal/pro.RuntimeMessage`). Bản sai chữ ký bị bỏ qua như không có.
- Service lưu danh sách cùng trạng thái Pro và chuyển cho driver (`Service.OfferMobileRuntime`,
  `mobile.RuntimeOfferer`). Danh sách engine công khai không bao giờ mang runtime vào được.
- Bấm Tải về: app hỏi lại máy chủ để có link mới, tải, so SHA256 của gói và của từng file, rồi mới cài.
- Chưa có bản nào cho máy này: trang Mobile ghi "Sắp có".

## Bên trong runtime

```
<data dir>/mobile/runtime/
  qemu/bin/      qemu-system-aarch64 qemu-img (dựng tĩnh, chỉ liên kết thư viện của macOS) scrcpy adb
  qemu/share/    edk2-aarch64-code.fd edk2-arm-vars.fd scrcpy-server
  qemu/SOURCES.txt   phiên bản, giấy phép và nơi lấy mã nguồn của từng phần
  qemu/VERSION   phiên bản runtime (app ghi khi cài)
  image/CURRENT  phiên bản image đang dùng
  image/<phiên bản>/…/vda.qcow2, vdb.qcow2   gói android-vm-image
```

- **QEMU** dựng từ tarball chính thức, chỉ bật những gì máy Android dùng: aarch64, HVF, virtio, xhci, qcow2 với
  zlib. GLib, pixman, PCRE2, libffi và libfdt được liên kết tĩnh. `MACOSX_DEPLOYMENT_TARGET` cố định (trong
  `android-vm/MIN_MACOS`). Script dựng dừng lại nếu `-accel help` không có `hvf`, nếu có liên kết ra ngoài macOS,
  hoặc nếu một file đòi bản macOS cao hơn.
- **scrcpy và adb** lấy nguyên bản phát hành của Genymobile, ghim phiên bản và SHA256.
- **Ký:** QEMU cần entitlement `com.apple.security.hypervisor`. Với Developer ID thì bật hardened runtime; ký ad hoc
  (`-`) thì không.
- **Giấy phép:** QEMU là GPL-2.0, GLib LGPL-2.1, EDK2 BSD, scrcpy và adb Apache-2.0. Mỗi bản kèm tarball mã nguồn
  đầy đủ (mọi tarball, patch và script dựng). Người có Pro tải nó từ trang Mobile, link "Tải mã nguồn". AnyProfile
  chạy QEMU như chương trình riêng, không liên kết.

## Dựng và phát hành

Làm trong repo AnyProfile Pro, trên Mac Apple Silicon; các bước trong `android-vm/README.md` của repo đó:
`scripts/build.sh` dựng, `scripts/publish.sh` ký và đưa lên máy chủ license. Không cần ra app mới.

## Ghim một image mới

```bash
scripts/pin-android-vm-image.sh v2026.10.18
```

Script tải file zip của bản đó, so SHA256 với digest GitHub ghi cho asset, chạy
`gh attestation verify --repo jqssun/android-lineage-qemu`, giải nén và in mục `engines.json` kèm hash của
`vda.qcow2` và `vdb.qcow2`. Trước khi ghim, chạy máy trên image đó từ đầu tới cuối:

```bash
ANYPROFILE_VM_RUNTIME=<runtime dir> go test -tags vmintegration -v -timeout 30m ./internal/mobile/vm/
```

`TestVM` tạo máy, mở, kiểm mạng qua proxy, DNS, chặn UDP, proxy từ chối thì mất mạng, múi giờ, ngôn ngữ, đồng hồ,
nhận lại sau crash và sao lưu; `TestControl` chạy các thao tác của agent. Đạt thì thêm mục vào
`engines.json`, hoặc sửa `vm.ImageBuiltin()` khi muốn cài mới nhận bản đó mà không cần danh sách ký.

## Runtime cho lúc phát triển

Không cần dựng gói: dùng QEMU, adb và scrcpy của Homebrew cùng một file image đã tải.

```bash
brew install qemu android-platform-tools scrcpy
scripts/dev-android-vm-runtime.sh "<data dir>/mobile/runtime" ~/Downloads/UTM-VM-lineage-*.zip
```

Script tạo đúng cây thư mục như bản tải về, bằng symlink tới Homebrew. Thư mục dữ liệu của bản dev nằm trong
`internal/paths`.
