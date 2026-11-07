# Nơi đặt bản dựng, chứng chỉ và kết quả test

Quy ước bắt buộc của dự án: **mọi bản dựng, bản ký, dữ liệu đo đạc và kết quả/log test thuộc AnyProfile đều phải nằm trong `artifacts/` hoặc `build/bin/` của repo**.
Tuyệt đối **không build, đóng gói hay lưu file test/log/ảnh chụp màn hình linh tinh ra ngoài** (`$HOME`, `~/chromium-work/dist-*`, `/tmp`, hay ngay tại gốc repo).

Lý do: một chỗ duy nhất để tìm lại, để sao lưu và để dọn; `/tmp` bị xoá bất cứ lúc nào; và một thư mục rời trong
`$HOME` hoặc gốc repo rất dễ bị bỏ sót, chiếm hàng GB dung lượng hoặc bị commit/chia sẻ nhầm — trong đó có private key.

## Đặt ở đâu

| Thứ | Đường dẫn chuẩn duy nhất |
|---|---|
| App đã build (`.app`, `.dmg`, `.zip`) | `build/bin/` (đã gitignore; **không** chạy `go build` tạo binary rời ở gốc repo) |
| Engine AnyBrowser (zip đã staple, `.release.json`, source tar) | `artifacts/anybrowser/engine/<dist-…>/` |
| Bộ chạy AnyMobile (archive, source, record) | `artifacts/anybrowser/anymobile-<version>-signed/` |
| Chứng chỉ, khoá, mật khẩu, script công chứng (`p12`, `p8`, `*.pass`, `notary.env`) | `artifacts/anybrowser/anyprofile-signing/` |
| Bằng chứng đo font / rebuild | `artifacts/anybrowser/anybrowser-evidence/` |
| Bản sao lưu lịch sử git (`.bundle`) | `artifacts/backups/` |
| Mọi dữ liệu test local (log, ảnh chụp màn hình, thư mục data/engine tạm khi test) | `artifacts/local-test/` (xoá các thư mục data/engine giải nén tạm ngay sau khi test xong) |

`artifacts/` bị `.gitignore` chặn. **Không commit, không đưa lên mạng, không chia sẻ**: `anyprofile-signing/`
chứa private key Developer ID, khoá API App Store Connect (`.p8`) và mật khẩu.

Workspace dựng chỉ dùng để chứa cây mã nguồn và `out/` biên dịch trung gian, **không đặt gói phát hành (`dist-*`) ở đó**:

- `~/chromium-work` (Chromium 46 GB, `CHROME_WORK` trong `pro/chrome/scripts/lib.sh`);
- `~/android-vm-work` (QEMU cho AnyMobile).

Đầu ra đóng gói của chúng (`PACKAGE_OUT`, source tarball, bản đã staple) phải ghi thẳng vào `artifacts/anybrowser/`, và xoá ngay các bản chưa staple trung gian sau khi đã có bản `dist-stapled-<version>` hoàn chỉnh.

## Khi tạo bản ký mới

1. Tạo thư mục mới trong `artifacts/`, có version trong tên — không ghi đè bản cũ. Server từ chối ghi đè một
   version đã phát hành, nên đổi version trước khi publish.
2. Ký, rồi kiểm ngay tại chỗ:

   ```sh
   # App
   codesign -dvv build/bin/AnyProfile.app        # phải ra Authority=Developer ID Application
   spctl -a -vv build/bin/AnyProfile.app          # chỉ accepted sau khi Apple công chứng

   # Engine / AnyMobile
   pro/anymobile/scripts/verify.sh --require-signature artifacts/anybrowser/<bản>
   ```

3. Chỉ publish sau khi bước 2 đạt, và publish từ đúng thư mục trong `artifacts/`.

## Liên quan

- Quy trình phát hành app và engine: `docs/releasing.md`.
- Runtime AnyMobile: `docs/anymobile-runtime.md`.
- Bảng tra từng thư mục đang có: `artifacts/anybrowser/README.md` (local, không commit).
