# Bảo trì máy mobile: nguồn, phiên bản, cách cập nhật

Tài liệu cho người bảo trì AnyProfile. Mỗi thành phần bên ngoài của máy mobile: lấy từ đâu, giấy phép, đang ghim
bản nào, ghim ở dòng code nào, cập nhật ra sao và kiểm bằng gì. Dữ liệu kiểm ngày **2026-10-24**.

Tài liệu liên quan:

- `docs/mobile.md`: máy mobile chạy thế nào (cho người dùng).
- `docs/mobile-apps.md`: cài app trong máy (cho người dùng).
- `docs/android-vm-runtime.md`: dựng, ký và phát hành runtime macOS.
- `docs/releasing.md`, mục "Android VM runtime": các bước phát hành.
- `docs/manual-testing.md`, "Phần C: máy mobile": checklist test tay.
- `docs/mcp.md`, mục "Máy mobile": tool cho agent.

## Thành phần và nguồn

| Thành phần | Dùng cho | Nguồn gốc | Giấy phép | Đang ghim | Ghim ở đâu |
|---|---|---|---|---|---|
| Image LineageOS (gói UTM) | Android của máy ảo macOS | [jqssun/android-lineage-qemu](https://github.com/jqssun/android-lineage-qemu/releases) | GPL-3.0 (bản dựng), LineageOS theo AOSP | 23.2, build 20261019, release `v2026.10.18` | `internal/mobile/vm/install.go:37` (`ImageBuiltin`), hoặc mục `android-vm-image` trong `engines.json` |
| QEMU + EDK2 | Chạy máy ảo | Homebrew trên máy dựng ([formula](https://github.com/Homebrew/homebrew-core/blob/HEAD/Formula/q/qemu.rb)) | GPL-2.0 (QEMU), BSD (EDK2) | Bản Homebrew lúc dựng runtime; ghi trong `qemu/SOURCES.txt` của gói | `scripts/build-android-vm-runtime.sh:28` |
| scrcpy + adb | Cửa sổ điện thoại, điều khiển | [Genymobile/scrcpy](https://github.com/Genymobile/scrcpy/releases) | Apache-2.0 | 5.0.1 | `scripts/build-android-vm-runtime.sh:23-24` (phiên bản, SHA256) |
| gvisor-tap-vsock | Mạng ảo (vnet) của máy macOS | [containers/gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock) | Apache-2.0 | v0.9.0 | `go.mod` |
| gVisor (netstack) | Stack TCP/IP của vnet | [google/gvisor](https://github.com/google/gvisor) | Apache-2.0 | `v0.0.0-20260413194555-9680d69bf798` | `go.mod` |
| DNS-over-HTTPS | DNS của máy macOS | Cloudflare `1.1.1.1` | — | `https://1.1.1.1/dns-query`, dự phòng TCP 53 | `internal/mobile/vm/vnet/dns.go:27` |
| ADBKeyBoard | Agent gõ chữ (Unicode) | [senzhk/ADBKeyBoard](https://github.com/senzhk/ADBKeyBoard/releases) | GPL-2.0 | v2.5-dev (`keyboardservice-debug.apk`) | `internal/mobile/control/control.go:38-41` (URL, SHA256) |
| SocksTun | Đưa mạng LDPlayer qua proxy | [heiher/sockstun](https://github.com/heiher/sockstun/releases) | MIT | 8.0 | `internal/mobile/ldplayer/tunnel.go:23-25` (phiên bản, URL, SHA256) |
| LDPlayer 9 | Máy Android trên Windows | Người dùng tự cài từ ldplayer.net | Phần mềm đóng | Không ghim | Tìm qua registry `internal/mobile/ldplayer/find_windows.go:15`, dự phòng `C:\LDPlayer\LDPlayer9` (dòng 33) |

App bên thứ ba mà tài liệu cho người dùng gợi ý (AnyProfile không đóng gói, không tải thay người dùng):

| App | Nguồn chính thức | Giấy phép | Nhắc ở |
|---|---|---|---|
| Aurora Store | [F-Droid](https://f-droid.org/packages/com.aurora.store/), mã nguồn [GitLab AuroraOSS](https://gitlab.com/AuroraOSS/AuroraStore) | GPL-3.0 | `docs/mobile-apps.md` |
| F-Droid | <https://f-droid.org/F-Droid.apk> | GPL-3.0 | `docs/mobile-apps.md` |
| APKMirror | <https://www.apkmirror.com/> | — (trang tải) | `docs/mobile-apps.md` |

Link "Hướng dẫn cài app" trên trang Mobile trỏ tới `docs/mobile-apps.md` trên nhánh `master` của GitHub
(`MOBILE_APPS_GUIDE`, `frontend/src/components/api-settings.tsx:16`). Đổi tên hay chuyển file đó thì sửa hằng này.

## Bản mới đã có nhưng chưa dùng

Kiểm ngày 2026-10-24. Cập nhật bảng này mỗi lần rà phiên bản.

| Thành phần | Bản mới nhất ở nguồn | Trạng thái |
|---|---|---|
| Image LineageOS | `v2026.10.20`: LineageOS **24.0** (`UTM-VM-lineage-24.0-20261020-jqssun-virtio_arm64only.zip`) | Chưa chạy thử, chưa ghim |
| scrcpy | v5.0.1 | Đang dùng |
| ADBKeyBoard | v2.5-dev | Đang dùng |
| SocksTun | 8.0 | Đang dùng |
| gvisor-tap-vsock | v0.9.0 | Đang dùng |

Lệnh rà nhanh:

```bash
for r in jqssun/android-lineage-qemu Genymobile/scrcpy senzhk/ADBKeyBoard heiher/sockstun containers/gvisor-tap-vsock; do
  gh api repos/$r/releases --jq ".[0] | \"$r \(.tag_name) \(.published_at[:10])\""
done
```

## Cập nhật từng thành phần

### Image LineageOS

1. `scripts/pin-android-vm-image.sh <tag>`: tải zip, so SHA256 với digest GitHub, chạy
   `gh attestation verify --repo jqssun/android-lineage-qemu`, in mục `engines.json` kèm hash `vda.qcow2`, `vdb.qcow2`.
2. Dựng runtime dev với image mới (`scripts/dev-android-vm-runtime.sh <dir> <zip>`) và chạy
   `ANYPROFILE_VM_RUNTIME=<dir> go test -tags vmintegration -v -timeout 40m ./internal/mobile/vm/` (`TestVM`, `TestControl`).
3. Chạy lại [ma trận app](#ma-trận-app) trên image mới. Bản Android lớn (23 → 24) có thể đổi lệnh `cmd`/`settings`
   mà driver dùng: múi giờ, ngôn ngữ từng app, đồng hồ, màn hình khoá (`internal/mobile/vm/driver.go`).
4. Đạt thì thêm mục vào `engines.json`, hoặc sửa `ImageBuiltin()`. Chỉ **máy tạo sau đó** dùng image mới.
   Đĩa hệ thống của mỗi máy là lớp phủ qcow2 trên `image/<phiên bản>/vda.qcow2`
   (`internal/mobile/vm/driver.go:261`), nên máy cũ vẫn cần thư mục image cũ. App không xoá image cũ; mỗi bản
   chiếm khoảng 2,3 GB. Muốn dọn image cũ thì phải chuyển máy cũ trước (chưa có tính năng này).
5. Ghi lại vào bảng "Thành phần và nguồn" và "Bản mới đã có".

### Runtime macOS (QEMU, scrcpy, adb)

- QEMU: `brew upgrade qemu` trên máy dựng, rồi dựng lại gói. scrcpy: sửa `scrcpy_ver` và `scrcpy_sha`
  (`scripts/build-android-vm-runtime.sh:23-24`); SHA256 lấy từ digest của asset trên GitHub.
- Dựng, ký, phát hành: `docs/android-vm-runtime.md` và `docs/releasing.md`. Tag `android-vm-vX.Y.Z` chạy
  `.github/workflows/android-vm-runtime.yml`.
- Kiểm: giải nén gói vào runtime sạch, chạy `TestVM` và `TestControl` như trên.

### ADBKeyBoard, SocksTun

Sửa URL và SHA256 ở dòng ghi trong bảng. Lấy SHA256 từ digest GitHub của asset
(`gh api repos/<repo>/releases/tags/<tag> --jq '.assets[] | .name + " " + .digest'`), không tính từ file tự tải.

- ADBKeyBoard: chạy `TestControl` (gõ "Xin chào 😀 tiếng Việt", kiểm bàn phím của người dùng được trả lại).
- SocksTun: driver ghi thẳng `shared_prefs/SocksPrefs.xml`; khoá trong file đọc từ `Preferences.java` của bản đang
  dùng. Bản mới đổi khoá thì sửa `internal/mobile/ldplayer/tunnel.go`. Kiểm trên Windows thật: xem
  `docs/manual-testing.md`, Phần C.

### gvisor-tap-vsock, gVisor

`go get github.com/containers/gvisor-tap-vsock@<tag>` rồi `go mod tidy`. vnet dựng stack từ các khối export của thư
viện (`internal/mobile/vm/vnet`), không fork. Kiểm: `go test -race ./internal/mobile/vm/vnet/`, `govulncheck ./...`,
rồi `TestVM`.

### LDPlayer

Không ghim; LDPlayer tự cập nhật trên máy người dùng. Bản lớn mới (LDPlayer 10…) cần kiểm: khoá registry, tên và tham
số `ldconsole` (`internal/mobile/ldplayer/console.go`), cổng adb `5555 + 2×index` (dòng 86), độ phân giải tạo máy
(`internal/mobile/ldplayer/driver.go:118`, đang `1080,2400,420`).

## Ma trận app

Kết quả chạy thật trên máy mobile macOS. Chạy lại khi đổi image hay runtime, và ghi ngày.

| App | Cách cài | Kết quả | Ngày, image |
|---|---|---|---|
| TikTok `com.zhiliaoapp.musically` | Aurora Store | Cài, mở tới màn hình điều khoản. Đăng nhập chưa kiểm | 2026-10-23, 23.2.20261019 |
| YouTube `com.google.android.youtube` | Aurora Store | Mở rồi tự đóng: "requires the Google Play Store, but it is missing" | 2026-10-23, 23.2.20261019 |
| YouTube web `m.youtube.com` | Trình duyệt có sẵn | Chạy, phát video | 2026-10-23, 23.2.20261019 |
| F-Droid, Aurora Store | Nút Cài app (APK) | Cài, mở, tải được kho app | 2026-10-23, 23.2.20261019 |
| DuckDuckGo | F-Droid | Cài, mở | 2026-10-23, 23.2.20261019 |

Kết quả đổi thì sửa luôn bảng "App phổ biến" trong `docs/mobile-apps.md`.

## Thông số chọn bằng đo đạc

Đổi các số này thì đo lại và ghi kết quả ở đây.

| Thông số | Giá trị | Ở đâu | Lý do |
|---|---|---|---|
| Màn hình máy ảo | 720×1600, density 280 | `internal/mobile/vm/driver.go:47` | Không có GPU. Đo trên M2 khi cuộn: 1080×2400 giật 42% khung hình, CPU 322%; 720×1600 giật 16%, CPU 161% |
| RAM mặc định | 3 GB (2–6 GB) | `internal/service/mobile.go:25` | Chạy được 4 máy trên Mac 32 GB. Chưa đo app nặng như TikTok khi dùng lâu |
| Chỗ trống để tải image | 4 GB | `internal/service/mobile.go:117` | Zip 1,1 GB cộng bản giải nén 2,3 GB |
| Cổng server adb riêng | 5137 | `internal/mobile/adb/adb.go:24` | Không đụng adb của người dùng (5037) |

## Những gì cố ý không làm

Ghi lại để lần cập nhật sau không phải bàn lại:

- **Không đóng gói Google Play / GApps:** không được phép phân phối lại, và máy cài kiểu này bị Play Protect đánh
  dấu "chưa chứng nhận". Người dùng tự cài app qua Aurora Store hay F-Droid.
- **Không root, không Magisk, không giả thông tin thiết bị, không giấu dấu vết máy ảo.** Lời giới thiệu sản phẩm
  chỉ hứa "máy Android riêng, proxy riêng, không rò IP".
- **Không vượt captcha.**
- **GPU (virgl) để sau:** cách tăng tốc lớn nhất, nhưng cần dựng QEMU có virglrenderer và kiểm lại toàn bộ.
