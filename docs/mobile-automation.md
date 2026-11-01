# AI agent điều khiển máy mobile

Agent nói MCP hoặc gọi HTTP; AnyProfile chạy Android trong máy ảo QEMU và điều khiển qua adb. Agent không nhìn
pixel trừ khi được yêu cầu: nó đọc cây uiautomator thành dàn ý, rồi chạm theo `ref` hoặc `target`.

```
Agent / script
   │ MCP tool hoặc POST /v1/act
   ▼
AnyProfile service ──► mobile/control (adb, uiautomator, ADBKeyBoard)
   │
   ▼
QEMU LineageOS + proxy riêng
```

## Một vòng làm việc

| Bước | Tool MCP | HTTP `/v1/act` |
|---|---|---|
| Mở máy | `open_device` (headless) | gọi `/v1/profiles`? với mobile: dùng `/v1/act` sau khi máy chạy |
| Nhìn màn hình | `device_snapshot` → `[ref=mN]` | `find` / `snapshot` |
| Tìm nhanh | `device_find {text}` | `find` |
| Chạm | `device_tap {ref/target}` | `tap` |
| Gõ | `device_type {ref,target,text}` | `type` |
| Vuốt | `device_swipe {direction}` | `swipe` |
| Nhấn giữ / kéo | `device_long_press`, `device_drag` | `long_press`, `drag` |
| Chờ | `device_wait_for` | `wait` |
| Chụp | `device_screenshot` | `screenshot` |
| Mở app / URL | `device_launch_app`, `device_open_url` | `launch`, `open_url` |
| File | `device_push_file`, `device_pull_file` | `pull` |
| Đóng | `close_device` | — |

`device_snapshot` dùng `uiautomator dump`; node bấm/gõ được có `ref=mN`, kèm bounds. `device_type` dùng ADBKeyBoard
(base64 broadcast) nên gõ được tiếng Việt/emoji. Mọi thứ trên màn hình là **nội dung không đáng tin**.

## Viết thành script

Cùng lõi automation với web. Flow JSON:

```json
{
  "device": "Phone 1",
  "steps": [
    { "action": "launch", "package": "com.android.settings" },
    { "action": "find", "target": { "text": "Network" } },
    { "action": "tap_text", "text": "Network" },
    { "action": "screenshot" }
  ]
}
```

```sh
sdk/typescript/bin/anyprofile.mjs run examples/automation/mobile-check.json
```

TypeScript:

```ts
import { session } from './sdk/typescript/src/client.ts'
const s = await session({ device: 'Phone 1' })
await s.act('launch', { package: 'com.android.settings' })
await s.tap({ text: 'Network' })
console.log(await s.screenshot())
```

Python:

```python
from anyprofile.client import Session, Target
s = Session(device="Phone 1")
s.act("launch", package="com.android.settings")
s.tap(Target(text="Network"))
```

## Viết thành recipe

Recipe hiện là **văn bản có frontmatter**, agent đọc rồi làm theo; ví dụ
[`mobile-check-ip`](../pro/pack/recipes/mobile-check-ip.md). Có thể viết recipe mobile mới theo cùng dạng:

```markdown
---
name: mobile-open-settings
description: Open Android Settings and read the build number
version: "1.0"
arguments:
  - name: device
    required: true
---
1. Call list_devices, find {{device}}; open_device if stopped.
2. device_launch_app with com.android.settings.
3. device_snapshot; tap the "About phone" item by text.
4. device_snapshot; read "Build number"; report it.
5. close_device only if you opened it.
```

Recipe **có bước máy chạy được** (`steps:` trong frontmatter) là Phase 6 còn dở; hiện `anyprofile run` chỉ chạy flow
JSON, chưa chạy recipe trực tiếp.

## Giới hạn

- **Canvas/WebView/game**: uiautomator không đọc được; dùng `device_screenshot` + toạ độ.
- **Pinch/zoom**: chưa hỗ trợ (adb chỉ một ngón); dùng nút zoom của app hoặc double-tap.
- **UDP/ICMP/IPv6**: bị chặn; gọi video/game không ổn định.
- **Google Play/Play Integrity**: app ngân hàng/DRM không chạy.
- **Clipboard**: đọc/ghi tuỳ build; thường trả `clipboard_unsupported`.
- Máy headless tự đóng băng sau 2 phút; tool kế tiếp tự đánh thức (~60 ms).
