# Automation examples

Các flow này là mẫu; đổi `profile`/`device` sang tên có thật trên máy bạn.

- `web-read.json` — mở trang và đọc chữ.
- `web-form.json` — điền form; `EMAIL`/`PASSWORD` lấy từ `--var` hoặc môi trường.
- `mobile-check.json` — mở Settings, tìm và chạm "Network", chụp ảnh.

Chạy:

```sh
export ANYPROFILE_TOKEN=…
sdk/typescript/bin/anyprofile.mjs run examples/automation/web-read.json
sdk/typescript/bin/anyprofile.mjs run examples/automation/web-form.json --var EMAIL=… --var PASSWORD=…
```
