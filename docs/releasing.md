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
   `AnyProfile-macos-arm64.dmg` (người dùng tải để cài), `AnyProfile-macos-arm64.zip` (trình cập nhật trong app
   tải; bản 0.1.0 và 0.1.1 chỉ biết giải nén `.zip`), `AnyProfile-amd64-installer.exe` và `latest.json`.
   `latest.json` ghi DMG dưới khoá `darwin-arm64-dmg`, để trang tải trên anytools.world có link và SHA-256 của nó.

Dựng DMG trên máy: `make dmg` (cần `python3`; `scripts/make-dmg.sh` cài `dmgbuild` vào một venv tạm). Hình nền
cửa sổ DMG nằm trong `build/darwin/dmg/`.

App đọc `https://github.com/D0bby999/anyprofile/releases/latest/download/latest.json` (repo công khai).

## Ký và công chứng bản macOS (Apple Developer)

Chưa có Developer ID thì app được ký ad hoc, và lần đầu mở bản tải về macOS hiện "Apple could not verify
AnyProfile…": người dùng phải vào System Settings → Privacy & Security → Open Anyway. Chỉ app ký bằng Developer ID
và được Apple công chứng (notarize) mới mở thẳng; đóng gói `.dmg` hay `.pkg` không thay được bước này (đã thử với
`spctl`: app kéo ra từ DMG chưa công chứng vẫn bị `rejected`).

Workflow đã có sẵn các bước ký và công chứng; chúng tự bỏ qua cho tới khi có đủ secret. Khi có tài khoản:

1. Đăng ký [Apple Developer Program](https://developer.apple.com/programs/) (99 USD/năm).
2. Tạo chứng chỉ **Developer ID Application**: Xcode → Settings → Accounts → Manage Certificates → + → Developer
   ID Application. Trong Keychain Access, xuất chứng chỉ kèm khoá riêng thành file `.p12`, đặt mật khẩu.
3. Tạo khoá API công chứng: App Store Connect → Users and Access → Integrations → App Store Connect API → Team
   Keys → +, quyền Developer. Tải file `AuthKey_<ID>.p8` (chỉ tải được một lần), ghi lại Key ID và Issuer ID.
4. Đưa lên GitHub, không in ra màn hình:

   ```
   base64 -i DeveloperID.p12 | gh secret set MACOS_CERT_P12 -R D0bby999/anyprofile
   gh secret set MACOS_CERT_PASSWORD -R D0bby999/anyprofile        # gõ mật khẩu của .p12
   base64 -i AuthKey_XXXX.p8 | gh secret set APPLE_API_KEY -R D0bby999/anyprofile
   gh secret set APPLE_API_KEY_ID -R D0bby999/anyprofile
   gh secret set APPLE_API_ISSUER -R D0bby999/anyprofile
   ```

5. Ra phiên bản tiếp theo như thường. Bước macOS ký app bằng Developer ID (hardened runtime, secure timestamp),
   ký DMG, gửi Apple công chứng, đóng dấu (staple) vào DMG, rồi kiểm bằng `spctl`.

Sau đó Keychain cũng thôi hỏi lại ở mỗi bản cập nhật: macOS nhận app theo Team ID thay vì theo từng bản build.
Bản cập nhật đầu tiên từ bản ad hoc sang bản có Developer ID vẫn hỏi một lần.

Các bước này chưa chạy thật lần nào (chưa có tài khoản): lần phát hành đầu có secret, xem kỹ log bước macOS.

## Engine mới mà không ra app mới

`latest.json` có thêm danh sách engine (`engines`) và chữ ký của nó (`enginesSig`, cùng khoá phát hành). App đọc
danh sách này khi kiểm tra cập nhật, kiểm chữ ký, rồi hiện các phiên bản trong Settings → Engines. Không thêm
request nào: đây vẫn là file `latest.json` app đã đọc.

- **Biết khi có bản mới:** workflow `Engine watch` chạy mỗi ngày và mở issue "Engine: … is out" kèm SHA256SUMS và
  ghi chú phát hành khi Apostate hoặc Fury có bản AnyProfile chưa biết.
- **Thêm một bản:**
  1. Tải file nén, so `shasum -a 256` với SHA256SUMS của bản đó. Giải nén, lấy hash các file cần kiểm (Apostate:
     `Chromium` và `Chromium Framework`, hoặc `chrome.exe` và `chrome.dll`; Fury: `Fury.app/Contents/MacOS/Fury`,
     khớp `core sha256` trong REPORT.md). macOS: `codesign -dv` phải ra đúng team.
  2. Chạy integration test với bản đó (`ANYPROFILE_TEST_APOSTATE_APP` hoặc `ANYPROFILE_TEST_FURY_APP`).
  3. Thêm một mục vào `engines.json` (cùng dạng `engine.Release` trong `internal/engine/release.go`).
     `catalogue`: giữ giá trị của bản trước nếu cùng seed vẫn ra cùng máy giả lập (Apostate: digest của
     `--fingerprint-explain` không đổi; Fury: `fury-config`, vì máy giả lập do app sinh). Khác thì đặt giá trị mới:
     profile cũ khi đó giữ bản cũ. `minApp`: bản AnyProfile thấp nhất chạy được nó.
  4. Commit `engines.json`, rồi ký vào `latest.json` của bản phát hành mới nhất và tải lên lại:

     ```
     gh release download -R D0bby999/anyprofile -p latest.json
     go run ./tools/release engines -manifest latest.json engines.json
     gh release upload "$(gh release view -R D0bby999/anyprofile --json tagName -q .tagName)" latest.json --clobber -R D0bby999/anyprofile
     ```

  Lần phát hành app sau, workflow tự ký `engines.json` vào `latest.json` mới. **Không bao giờ xoá một mục khỏi
  `engines.json`:** người dùng có thể có profile trên bản đó, và app không mở được profile trên một bản nó không
  còn biết.
- **Cần ra app mới khi:** bản engine cần cờ mới, hoặc Fury đổi định dạng cấu hình (khi đó `fury-shared` trong
  `tools/fury-config/Cargo.toml` phải đổi theo). Thêm bản đó vào `Builtin()` của engine thay vì `engines.json`.
- **Profile chuyển phiên bản thế nào:** profile mới dùng bản mới nhất đã cài. Profile cũ tự chuyển sang bản mới hơn
  đã cài khi mở, nếu cùng `catalogue`; khác `catalogue` thì giữ bản của nó. Chromium đổi thì baseline fingerprint
  được đo lại. Xoá một bản: profile của nó chuyển sang bản mới nhất cùng `catalogue`; không có thì không cho xoá.

## Repo

`D0bby999/anyprofile` (công khai) chứa mã nguồn, CI và các bản phát hành. Lịch sử đã viết lại ngày
2026-10-17: email tác giả là địa chỉ noreply của GitHub, không có `plans/` hay ảnh chụp màn hình. Repo riêng
tư cũ đã xoá; lịch sử gốc của nó nằm trong file sao lưu `anyprofile-private-final-261017.bundle` trên máy
phát hành.

Bản phát hành đầu: `v0.1.0`, 2026-10-17.

## Thử mà không phát hành

`ANYPROFILE_UPDATE_URL` thay địa chỉ `latest.json`; file trong đó vẫn phải ở `https://` và có chữ ký đúng.
