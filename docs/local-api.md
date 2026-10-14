# API trên máy

API cho script trên chính máy Mac này mở, đóng và điều khiển profile: Puppeteer, Playwright, Python,
n8n chạy cục bộ… API **tắt mặc định**.

## Bật

1. Settings → **Local API** → **On**. Cổng mặc định là `51525`; đổi được.
2. Bấm **Copy token**. Token chỉ đi qua bộ nhớ tạm và tự xoá sau 90 giây; app không hiện token ra màn
   hình. Lưu token vào nơi script đọc được, ví dụ biến môi trường `ANYPROFILE_TOKEN`.
3. **New token** thay token cũ; script dùng token cũ sẽ bị từ chối.

## Bảo vệ

- Chỉ nghe ở `127.0.0.1`. Máy khác trong mạng không gọi được.
- Mọi yêu cầu cần header `Authorization: Bearer <token>`.
- Yêu cầu có header `Origin` bị từ chối, nên một trang web mở trong trình duyệt không gọi được API.
  `Host` phải là `127.0.0.1:<cổng>` hoặc `localhost:<cổng>` (chống DNS rebinding).
- Nhật ký (`logs/anyprofile.log`) ghi phương thức, đường dẫn và mã trả về; không ghi token.
- **Cổng DevTools** chỉ mở khi script gọi `start` với `"devtools": true`, chỉ cho lần mở đó. Trong lúc
  cổng mở, mọi chương trình trên máy đều điều khiển được trình duyệt, kể cả đọc cookie. Đóng profile
  ngay khi script xong.

## Lệnh

| Lệnh | Trả về |
|---|---|
| `GET /v1/profiles` | `{"profiles": [Profile, …]}`, các profile không nằm trong thùng rác |
| `GET /v1/profiles/{id}` | `Profile` |
| `POST /v1/profiles/{id}/start` với `{"devtools": true}` hoặc không có body | `Profile` sau khi mở |
| `POST /v1/profiles/{id}/stop` | `Profile` sau khi đóng |

`Profile`:

```json
{
  "id": "7409ecb50b1a7205", "serial": 12, "name": "TikTok VN 01",
  "sites": ["tiktok", "youtube"], "tags": ["Nuôi"], "state": "running",
  "devtoolsPort": 54123,
  "webSocketDebuggerUrl": "ws://127.0.0.1:54123/devtools/browser/…"
}
```

`devtoolsPort` là 0 và `webSocketDebuggerUrl` vắng mặt khi lần mở không có DevTools. Profile đang mở
mà không có DevTools thì `start` với `devtools` bị từ chối (`api_running_without_devtools`): đóng nó
trước.

Lỗi có dạng `{"error": {"code": "…", "message": "…"}}`: 401 thiếu hay sai token, 403 bị chặn vì
`Origin` hay `Host`, 404 không có profile, 409 app từ chối (ví dụ profile trong thùng rác).

## Ví dụ

curl:

```sh
API=http://127.0.0.1:51525/v1 AUTH="Authorization: Bearer $ANYPROFILE_TOKEN"
curl -s -H "$AUTH" $API/profiles
curl -s -H "$AUTH" -X POST -d '{"devtools":true}' $API/profiles/<id>/start
curl -s -H "$AUTH" -X POST $API/profiles/<id>/stop
```

Puppeteer (Node):

```js
import puppeteer from 'puppeteer-core'
const api = 'http://127.0.0.1:51525/v1', headers = { Authorization: `Bearer ${process.env.ANYPROFILE_TOKEN}` }
const p = await (await fetch(`${api}/profiles/${id}/start`, { method: 'POST', headers, body: '{"devtools":true}' })).json()
const browser = await puppeteer.connect({ browserWSEndpoint: p.webSocketDebuggerUrl, defaultViewport: null })
const [page] = await browser.pages()
await page.goto('https://www.tiktok.com/tiktokstudio')
await browser.disconnect() // giữ trình duyệt mở; không gọi browser.close()
await fetch(`${api}/profiles/${id}/stop`, { method: 'POST', headers })
```

Playwright (Python):

```python
import os, requests
from playwright.sync_api import sync_playwright
api, h = "http://127.0.0.1:51525/v1", {"Authorization": f"Bearer {os.environ['ANYPROFILE_TOKEN']}"}
p = requests.post(f"{api}/profiles/{pid}/start", json={"devtools": True}, headers=h).json()
with sync_playwright() as pw:
    browser = pw.chromium.connect_over_cdp(p["webSocketDebuggerUrl"])
    page = browser.contexts[0].pages[0]
    page.goto("https://studio.youtube.com")
requests.post(f"{api}/profiles/{pid}/stop", headers=h)
```

Dùng trình duyệt sẵn có (`browser.contexts[0]`, `browser.pages()`): context mới của Playwright hay
Puppeteer không mang cookie của profile. `defaultViewport: null` giữ kích thước cửa sổ của profile;
đổi viewport làm lệch fingerprint.

## Không có

Không có Synchronizer (nhiều tài khoản làm cùng một thao tác cùng lúc là dấu hiệu bot), không đổi
fingerprint qua API, không tạo hay xoá profile qua API.
