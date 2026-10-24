# Android VM runtime (macOS)

Danh sách mọi thành phần của máy mobile, nguồn và cách cập nhật: `docs/mobile-maintenance.md`.

**Chưa phát hành được:** QEMU từ Homebrew hoặc đòi macOS 26, hoặc (bản cho Sonoma) không có HVF. Xem mục "Chặn phát
hành" trong `docs/mobile-maintenance.md`; `scripts/build-android-vm-runtime.sh` cần dựng QEMU từ mã nguồn.

Máy mobile trên macOS cần hai thứ tải về, cả hai đi qua danh sách engine có ký như Apostate và Fury
(`internal/engine`, `docs/releasing.md`):

| Engine | Là gì | Ai dựng | Lấy từ |
|---|---|---|---|
| `android-vm` | QEMU, EDK2, scrcpy, adb | AnyProfile (`scripts/build-android-vm-runtime.sh`) | Bản phát hành `android-vm-vX.Y.Z` của repo này |
| `android-vm-image` | LineageOS cho QEMU (gói UTM) | jqssun/android-lineage-qemu | Bản phát hành của jqssun; AnyProfile không lưu trữ |

Bản image dùng cho cài mới ghim trong `vm.ImageBuiltin()` (`internal/mobile/vm/install.go`). Runtime chỉ đến từ
`engines.json`: khi chưa có mục `android-vm` cho máy này, nút Tải về báo bộ chạy chưa được phát hành.

## Bên trong runtime

```
<data dir>/mobile/runtime/
  qemu/bin/      qemu-system-aarch64 qemu-img scrcpy adb
  qemu/lib/      các dylib của QEMU, đã sửa đường dẫn về @executable_path/@loader_path
  qemu/share/    edk2-aarch64-code.fd edk2-arm-vars.fd scrcpy-server
  qemu/SOURCES.txt   nguồn của mọi phần GPL/LGPL
  image/CURRENT  phiên bản image đang dùng
  image/<phiên bản>/vda.qcow2, vdb.qcow2   gói android-vm-image
```

- **QEMU và thư viện** chép từ Homebrew trên máy dựng, sửa liên kết để chạy không cần Homebrew. Script kiểm tra
  điều đó ở cuối (`otool -L` không còn đường dẫn Homebrew).
- **scrcpy và adb** lấy từ bản phát hành của Genymobile, ghim phiên bản và SHA256 trong script.
- **Ký:** QEMU cần entitlement `com.apple.security.hypervisor` (`build/darwin/qemu.entitlements`). Với Developer
  ID, mọi file cùng team nên bật hardened runtime. Ký ad hoc (`-`) thì không bật hardened runtime, vì library
  validation từ chối dylib không có team.
- **Giấy phép:** QEMU là GPL-2.0, EDK2 BSD, scrcpy và adb Apache-2.0. AnyProfile chạy chúng như chương trình riêng,
  không liên kết. `SOURCES.txt` ghi phiên bản và nơi lấy mã nguồn.

## Dựng một bản

Trên Mac Apple Silicon có Homebrew:

```bash
scripts/build-android-vm-runtime.sh 1.0.0 dist            # ký ad hoc
scripts/build-android-vm-runtime.sh 1.0.0 dist "Developer ID Application: …"
```

Script in ra mục `engines.json` (URL để `REPLACE…`). Thường không chạy tay: đẩy tag `android-vm-v1.0.0` và workflow
`.github/workflows/android-vm-runtime.yml` dựng, ký bằng Developer ID nếu có secret `MACOS_CERT_P12`, rồi tạo bản
phát hành với `--latest=false` (để `releases/latest` vẫn là bản app). Các bước phát hành: `docs/releasing.md`, mục
"Android VM runtime".

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
