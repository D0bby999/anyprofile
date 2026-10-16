# Phát hành và tự cập nhật

## Người dùng thấy gì

Settings → **Version** hiện phiên bản đang chạy, nút "Check for updates" và công tắc tự kiểm tra (mặc
định bật, tối đa một lần mỗi ngày, lúc mở app). Khi có bản mới, thanh trạng thái có nút "Version X is
available" dẫn tới Settings. Bấm "Update and restart":

1. app tải file của hệ điều hành mình (macOS: `.zip` chứa `AnyProfile.app`; Windows: bộ cài NSIS);
2. kiểm chữ ký Ed25519 bằng khoá công khai nhúng trong app, rồi kiểm SHA-256 của file đã tải. Sai một
   trong hai thì file bị xoá và app báo lỗi;
3. cần đóng hết profile trước (nút bị khoá khi còn profile mở);
4. macOS: kiểm chữ ký code của app mới, đặt nó vào chỗ app cũ, thoát, rồi mở bản mới. Windows: thoát, chạy
   bộ cài im lặng vào đúng thư mục cũ, rồi mở bản mới.

Profile, cơ sở dữ liệu, mật khẩu và engine không bị đụng. Lần kiểm tra chỉ tải `latest.json`, không gửi gì
về người dùng.

Trên macOS, app chạy từ một bản sao chỉ đọc (macOS "App Translocation", khi mở thẳng từ thư mục Downloads)
không tự cập nhật được: app báo người dùng chuyển `AnyProfile.app` vào Applications.

## Khoá ký

- Khoá bí mật nằm trong Keychain của máy Mac phát hành (mục `dev.anyprofile.release-key`), tạo bằng
  `go run ./tools/release keygen`. Khoá công khai nằm trong `internal/update/update.go`.
- **Sao lưu khoá này.** Mất khoá thì mọi bản đã cài không nhận bản cập nhật nào nữa; người dùng phải tải
  bản mới bằng tay.
- Đưa khoá lên GitHub (không in ra màn hình):

  ```
  go run ./tools/release export-key | gh secret set UPDATE_SIGNING_KEY -R D0bby999/anyprofile
  ```

## Ra một phiên bản

1. Đổi `productVersion` trong `wails.json` và `Current` trong `internal/update/update.go` (test
   `TestCurrentMatchesWailsJSON` bắt khi hai số lệch nhau). Commit.
2. Tạo tag có ghi chú; ghi chú hiện trong app:

   ```
   git tag -a v0.2.0 -m "Mở profile nhanh hơn. Sửa lỗi proxy SOCKS5."
   git push origin v0.2.0
   ```

3. Workflow `.github/workflows/release.yml` build bản macOS và Windows, ký, rồi tạo release có
   `AnyProfile-macos-arm64.zip`, `AnyProfile-amd64-installer.exe` và `latest.json`.

App đọc `https://github.com/D0bby999/anyprofile/releases/latest/download/latest.json` (repo công khai).

## Hai repo

- `D0bby999/anyprofile` (công khai): mã nguồn, CI, các bản phát hành. Lịch sử đã viết lại ngày 2026-10-17:
  email tác giả là địa chỉ noreply của GitHub, không có `plans/` hay ảnh chụp màn hình.
- `D0bby999/anyprofile-private` (riêng tư): lịch sử gốc trước khi công khai, cho bản mở rộng cá nhân.
  Lịch sử của nó khác repo công khai; muốn dùng tiếp thì tạo nhánh mới từ `origin/master`.

Bản phát hành đầu: `v0.1.0`, 2026-10-17.

## Thử mà không phát hành

`ANYPROFILE_UPDATE_URL` thay địa chỉ `latest.json`; file trong đó vẫn phải ở `https://` và có chữ ký đúng.
