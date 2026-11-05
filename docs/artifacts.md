# Nơi đặt bản dựng và chứng chỉ

Quy ước của dự án: **mọi bản dựng, bản ký và dữ liệu đo đạc thuộc AnyProfile đều nằm trong `artifacts/` của repo**,
không tạo thư mục rời trong `$HOME`, không để trong `/tmp`.

Lý do: một chỗ duy nhất để tìm lại, để sao lưu và để dọn; `/tmp` bị xoá bất cứ lúc nào; và một thư mục rời trong
`$HOME` rất dễ bị commit nhầm hoặc bị chia sẻ nhầm — trong đó có private key.

## Đặt ở đâu

| Thứ | Đường dẫn |
|---|---|
| App đã build (`.app`, `.dmg`, `.zip`) | `build/bin/` (đã gitignore) |
| Engine AnyBrowser (zip, `.release.json`, source tar) | `artifacts/anybrowser/engine/<dist-…>/` |
| Bộ chạy AnyMobile (archive, source, record) | `artifacts/anybrowser/anymobile-<version>-signed/` |
| Chứng chỉ, khoá, mật khẩu (`p12`, `*.pass`) | `artifacts/anybrowser/anyprofile-signing/` |
| Bằng chứng đo font / rebuild | `artifacts/anybrowser/anybrowser-evidence/` |

`artifacts/` bị `.gitignore` chặn. **Không commit, không đưa lên mạng, không chia sẻ**: `anyprofile-signing/`
chứa private key Developer ID và mật khẩu.

Workspace dựng được phép ở ngoài repo vì đó là cây nguồn nặng, không phải bản phát hành:

- `~/chromium-work` (Chromium 46 GB, `CHROME_WORK` trong `pro/chrome/scripts/lib.sh`);
- `~/android-vm-work` (QEMU cho AnyMobile).

Đầu ra của chúng luôn quay về `artifacts/`.

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
