# AI agent (MCP)

Claude Code, Codex, Gemini CLI, Cursor và các AI agent khác nói chuyện được với AnyProfile qua MCP. Agent mở profile
rồi làm việc ngay trong trình duyệt của profile đó: đọc trang, bấm, gõ, chụp màn hình, xem request mạng và body
response. Trình duyệt dùng đúng tài khoản đã đăng nhập, cookie, proxy và fingerprint của profile. Cửa sổ vẫn hiện
trên màn hình, nên bạn nhìn thấy agent đang làm gì.

MCP nằm trong [API trên máy](local-api.md), dùng chung công tắc và token. API tắt thì MCP cũng tắt.

## Kết nối agent của bạn

1. Mở AnyProfile → **Cài đặt** → **API trên máy** → bấm **Bật**. (Giao diện tiếng Anh: Settings → Local API → On.)
2. Ở mục **AI agent (MCP)**, chọn agent bạn dùng trong ô **Agent bạn dùng**, rồi bấm **Chép lệnh** hoặc **Chép cấu
   hình**. Nội dung chép đã có sẵn địa chỉ và token của máy bạn.
3. Dán vào đúng chỗ, theo dòng hướng dẫn hiện ngay dưới nút (chi tiết từng agent ở dưới).
4. Mở lại agent, kiểm tra rằng `anyprofile` đã kết nối, rồi giao việc. Ví dụ: "Mở profile Shop VN 01, vào trang đơn
   hàng, đọc 10 đơn mới nhất."

Nội dung chép có chứa token, và tự xoá khỏi bộ nhớ tạm sau 90 giây. Dán ngay sau khi chép.

| Agent | Cách nối | Dán vào | Kiểm tra |
|---|---|---|---|
| Claude Code | lệnh | terminal | `claude mcp list`, hoặc gõ `/mcp` trong Claude Code |
| Codex CLI | cấu hình | `~/.codex/config.toml` | `codex mcp list`, hoặc gõ `/mcp` trong Codex |
| Gemini CLI | lệnh | terminal | `gemini mcp list` → `Connected` |
| OpenCode | cấu hình | `~/.config/opencode/opencode.json` | `opencode mcp list` → `connected` |
| Cursor | cấu hình | `~/.cursor/mcp.json` | Cursor Settings → MCP |
| VS Code (Copilot) | cấu hình | `.vscode/mcp.json` của dự án | khung Chat, chế độ Agent, nút công cụ |
| Windsurf | cấu hình | `mcp_config.json` (Cascade → MCP → View raw config) | danh sách MCP của Cascade |

Trên Windows, `~` là thư mục người dùng, ví dụ `C:\Users\<tên>\.codex\config.toml`.

Đã thử ngày 18/10/2026: Claude Code 2.1, Gemini CLI 0.64 và OpenCode 2.0.12 kết nối được; Codex CLI 0.155.1 đọc
đúng cấu hình. Cách nối Cursor, VS Code và Windsurf theo tài liệu chính thức của từng bên.

Trong các ví dụ dưới, `51525` là cổng mặc định và `<token>` là token của bạn. Nút chép trong app điền sẵn cả hai.

### Claude Code

```bash
claude mcp add --scope user --transport http anyprofile http://127.0.0.1:51525/mcp --header "Authorization: Bearer <token>"
```

`--scope user` cho Claude Code dùng AnyProfile ở mọi thư mục. Thiếu nó, Claude Code chỉ thấy AnyProfile trong thư
mục bạn chạy lệnh. Lệnh dùng được trong Terminal của macOS, PowerShell và cmd của Windows.

### Codex CLI

Thêm vào cuối file `~/.codex/config.toml` (tạo file nếu chưa có), rồi mở lại Codex:

```toml
[mcp_servers.anyprofile]
url = "http://127.0.0.1:51525/mcp"
http_headers = { "Authorization" = "Bearer <token>" }
```

Không muốn để token trong file thì đặt biến môi trường `ANYPROFILE_TOKEN` (trong `~/.zshrc` hoặc biến môi trường của
Windows), rồi dùng lệnh `codex mcp add anyprofile --url http://127.0.0.1:51525/mcp --bearer-token-env-var ANYPROFILE_TOKEN`.

### Gemini CLI

```bash
gemini mcp add -s user --transport http anyprofile http://127.0.0.1:51525/mcp --header "Authorization: Bearer <token>"
```

Gemini chỉ bật MCP trong thư mục bạn đã tin cậy. Nếu `gemini mcp list` báo `Disabled` vì thư mục chưa được tin cậy,
mở `gemini` trong thư mục đó và chọn tin cậy.

### OpenCode

Dán vào `~/.config/opencode/opencode.json` (dùng cho mọi dự án), hoặc `opencode.json` trong thư mục dự án:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "anyprofile": {
      "type": "remote",
      "url": "http://127.0.0.1:51525/mcp",
      "headers": { "Authorization": "Bearer <token>" }
    }
  }
}
```

Đừng dùng `opencode mcp add`: ở bản 2.0.12, lệnh này ghi cấu hình ra một dạng mà chính `opencode mcp list` không
đọc được ("No MCP servers configured").

### Cursor

Dán vào `~/.cursor/mcp.json` (mọi dự án) hoặc `.cursor/mcp.json` trong dự án:

```json
{
  "mcpServers": {
    "anyprofile": {
      "url": "http://127.0.0.1:51525/mcp",
      "headers": { "Authorization": "Bearer <token>" }
    }
  }
}
```

### VS Code (GitHub Copilot)

Dán vào `.vscode/mcp.json` trong dự án. Lưu ý khoá ngoài cùng là `servers`, không phải `mcpServers`:

```json
{
  "servers": {
    "anyprofile": {
      "type": "http",
      "url": "http://127.0.0.1:51525/mcp",
      "headers": { "Authorization": "Bearer <token>" }
    }
  }
}
```

### Windsurf

Trong Windsurf: Cascade → MCP → **View raw config**, rồi dán vào `mcp_config.json`. Windsurf dùng `serverUrl`:

```json
{
  "mcpServers": {
    "anyprofile": {
      "serverUrl": "http://127.0.0.1:51525/mcp",
      "headers": { "Authorization": "Bearer <token>" }
    }
  }
}
```

### Agent khác

Agent nào hỗ trợ MCP kiểu **Streamable HTTP** và cho thêm header đều dùng được:

- Địa chỉ: `http://127.0.0.1:<cổng>/mcp`
- Header: `Authorization: Bearer <token>`

Mục **Agent khác** trong app chép khối JSON `mcpServers` mà đa số agent đọc. Agent chỉ chạy được lệnh trên máy
(kiểu stdio) thì chưa nối thẳng được.

### File đã có nội dung

Nếu file cấu hình đã có server khác, đừng dán đè cả file. Chỉ chép phần `"anyprofile": { … }` vào trong khối
`mcpServers` (Cursor, Windsurf), `servers` (VS Code) hoặc `mcp` (OpenCode), nhớ dấu phẩy giữa các server. Với Codex,
dán khối `[mcp_servers.anyprofile]` xuống cuối file là đủ.

### Giữ token an toàn

- Token nằm trong file cấu hình của agent. **Đừng commit** file cấu hình của dự án (`.vscode/mcp.json`,
  `.cursor/mcp.json`, `opencode.json`) lên git: thêm chúng vào `.gitignore`, hoặc dùng file cấu hình chung của
  người dùng.
- Cursor và Windsurf đọc được biến môi trường: viết `"Bearer ${env:ANYPROFILE_TOKEN}"` thay cho token.
- Bấm **Token mới** trong app thì token cũ hết hiệu lực ngay. Sau đó chép lại cấu hình cho từng agent.

## Khi không kết nối được

| Hiện tượng | Nguyên nhân | Cách sửa |
|---|---|---|
| Không kết nối được, `connection refused`, `fetch failed` | AnyProfile chưa mở, API đang Tắt, hoặc đã đổi cổng | Mở AnyProfile, bật API trên máy, kiểm tra cổng, chép lại cấu hình |
| `401` hoặc `Unauthorized` | Token đã đổi (bấm Token mới), hoặc dán thiếu | Chép lại cấu hình rồi dán đè phần cũ |
| Claude Code chỉ thấy `anyprofile` ở một thư mục | Lệnh cũ không có `--scope user` | `claude mcp remove anyprofile`, rồi chép lệnh mới trong app |
| Gemini báo `Disabled` | Thư mục chưa được tin cậy | Mở `gemini` trong thư mục đó và chọn tin cậy |
| OpenCode báo `No MCP servers configured` | Đã thêm bằng `opencode mcp add` | Xoá phần đó, dán khối JSON ở trên |
| Agent báo `agent_no_pipe` | Profile mở từ trước khi AnyProfile khởi động lại | Đóng profile rồi mở lại |
| Agent không thấy file để upload | File nằm ngoài thư mục uploads | Cài đặt → API trên máy → Mở thư mục uploads, chép file vào đó |

`logs/anyprofile.log` ghi mỗi lần agent gọi tool một dòng, kèm lỗi nếu có.

## Tool

Mọi tool trình duyệt nhận `profile` (id, đúng tên, hoặc `#số thứ tự`) và `tab` (tuỳ chọn). Không ghi `tab` thì
tool dùng tab agent dùng gần nhất, nếu chưa có thì dùng tab đang hiện trên màn hình.

| Tool | Làm gì |
|---|---|
| `list_profiles` | id, số thứ tự, tên, đang mở hay không, nền tảng, tag; lọc theo tên, tag, nền tảng, đang mở |
| `open_profile` / `close_profile` | mở profile (có proxy, như bấm Open) và đợi sẵn sàng; đóng profile |
| `list_tabs`, `open_tab`, `close_tab` | các tab; mở tab mới tới một địa chỉ; đóng một tab |
| `navigate` | đi tới địa chỉ, hoặc lùi, tiến, tải lại; đợi trang tải xong |
| `snapshot` | trang dưới dạng dàn ý vai trò và tên, gồm cả nội dung iframe; phần tử bấm được có `[ref=eN]` (trong iframe: `eNfK`) |
| `click`, `type`, `press_key`, `scroll` | thao tác theo `ref` |
| `select_option` | chọn một mục của ô thả xuống gốc (`<select>`); bấm vào ô đó sẽ mở danh sách của hệ điều hành, agent không với tới |
| `wait_for` | đợi chữ hiện ra hoặc mất đi, hay địa chỉ khớp |
| `read_page` | chữ đang hiện trên trang hoặc của một phần tử, kèm link nếu muốn |
| `screenshot` | ảnh phần đang hiện, rộng tối đa 1280 px |
| `network_requests`, `response_body` | các request của tab, và body dạng chữ hoặc JSON của một response |
| `list_uploads`, `upload_file` | các file trong thư mục uploads; đặt file vào ô chọn file của trang |
| `recipe` | liệt kê recipe, hoặc lấy các bước của một recipe |

Giới hạn: `snapshot` và `read_page` trả tối đa 40.000 ký tự, `response_body` tối đa 200 KB. Phần bị cắt có ghi
`offset` để đọc tiếp. Body nhị phân (ảnh, video) chỉ báo loại và kích thước.

## Upload file

Agent chỉ upload được file nằm trong **thư mục uploads** (Cài đặt → API trên máy → Mở thư mục uploads; trên đĩa là
`uploads` trong thư mục dữ liệu của app). Đường dẫn tuyệt đối, `..` và symlink trỏ ra ngoài đều bị từ chối, vì một
trang độc có thể lừa agent tải file riêng tư của bạn lên. Thư mục con được phép (`clips/a.mp4`).

`upload_file` nhận hai kiểu `ref`: một ô `<input type=file>`, hoặc nút mở hộp chọn file (như "Select files" của
YouTube Studio, nơi ô chọn file bị ẩn). Với kiểu thứ hai, AnyProfile chặn hộp chọn file của hệ điều hành, bấm nút
bằng chuột thật, rồi đặt file vào ô mà trang vừa mở.

## Recipe

Recipe là các bước làm một việc trên một nền tảng, viết cho agent: một file `.md` trong **thư mục recipes**
(Cài đặt → API trên máy → Mở thư mục recipe).

```markdown
---
name: youtube-upload
description: Upload một video lên YouTube Studio, để Private
version: "1.0"
arguments:
  - name: file
    description: tên file trong thư mục uploads
    required: true
---
Upload {{file}} … Stop before clicking Publish and ask the user.
```

- `name`: chữ thường, số và `-`. Tham số: chữ thường, số và `_`. Mỗi `{{tham_số}}` trong thân phải được khai báo.
- Agent hỗ trợ prompt của MCP hiện mỗi recipe thành một lệnh; trong Claude Code là `/mcp__anyprofile__<name>`.
  Agent nào cũng tự tìm được recipe bằng tool `recipe`.
- Thả file vào thư mục là dùng được ngay, không cần khởi động lại. File sai định dạng không được nạp; tool `recipe`
  (không có tên) liệt kê chúng cùng lý do.
- Recipe nên ghi rõ chỗ dừng lại hỏi người dùng trước việc không quay lại được (Publish, gửi, xoá).

Mẫu miễn phí: `examples/recipes/check-login.md`. Template script Playwright, Puppeteer và Python: `examples/`.

## Agent làm việc thế nào

- AnyProfile đã nối sẵn với mọi trình duyệt nó mở qua một **đường ống riêng**, không qua cổng mạng. Agent đi qua
  AnyProfile tới đường ống đó, nên **không cổng DevTools nào được mở**, và chương trình khác trên máy không chen vào
  được. Đây là khác biệt với `start` kèm `"devtools": true` của API.
- Agent đọc trang trong một vùng tách biệt mà script của trang không thấy, và không bật `Runtime`, thứ mà script
  chống bot hay dò.
- Bấm và gõ bằng sự kiện chuột và bàn phím thật: chuột di tới phần tử rồi mới bấm, phím gõ từng chữ có nghỉ ngắn.
  Trang thấy `isTrusted: true`, như khi người dùng thao tác.
- Không đổi kích thước cửa sổ, viewport hay thiết bị giả lập. Ảnh chụp được thu nhỏ trong AnyProfile, không thu
  nhỏ trên trang.
- Agent làm việc **phía sau**: profile mở qua MCP hoặc API không giành focus của app bạn đang dùng, `open_tab` mở
  một tab trong cửa sổ của profile (không mở cửa sổ thứ hai), và agent không kéo trình duyệt lên trước khi bấm, gõ hay chụp. Muốn xem, bấm nút
  Focus ở dòng của profile. Nếu Chromium vẫn tự nhảy lên lúc khởi động, AnyProfile lấy lại focus ngay; có thể thấy
  cửa sổ loé lên một chút.
- Profile đang bị agent điều khiển có nhãn **Agent** ở bảng profile và ở ngăn chi tiết. Nhãn tắt sau một phút agent
  không gọi tool nào, hoặc khi profile đóng.
- `logs/anyprofile.log` ghi mỗi lần gọi tool một dòng: tên tool, id profile, thời gian, thành công hay không.
  Không ghi nội dung trang, chữ đã gõ hay body.

Đã đo ngày 17/10/2026 trên Apostate 0.5.1, macOS, với Claude Code điều khiển qua MCP:
`deviceandbrowserinfo.com/are_you_a_bot` báo "You are human!" (`isAutomatedWithCDP: false`, 22/22 mục `false`).
Bẫy `Runtime.enable` trong test tích hợp không bị kích hoạt. Trang đó không đo hành vi (nhịp chuột, tốc độ gõ).

## Hộp thoại của trình duyệt

Hộp thoại `alert`, `confirm`, `prompt` và "Rời trang?" chặn trang cho tới khi có người trả lời. AnyProfile trả lời
thay và ghi lại trong kết quả công cụ (dòng `Dialog:`):

- `alert`, `confirm`: OK. Hộp thoại xác nhận đi sau chính cú bấm của agent (Xoá, Gửi…), nên agent muốn vậy.
- `prompt`: Huỷ; agent gõ vào trang thay vì vào hộp thoại.
- "Rời trang?": lần đầu ở lại, vì những gì chưa lưu sẽ mất. Agent lưu trước, hoặc đi tiếp lần nữa trong 2 phút thì rời.

Để nhận hộp thoại, agent bật miền `Page` của DevTools; script của trang không thấy được việc này. Miền `Runtime`,
thứ trang có thể nhận ra, vẫn không bao giờ bật.

## Máy mobile

Khi có máy mobile (AnyProfile Pro, `docs/mobile.md`), agent có thêm các tool dưới đây. Mọi tool nhận `device`: id,
đúng tên, hoặc `#số thứ tự`.

| Tool | Làm gì |
|---|---|
| `list_devices` | id, số thứ tự, tên, đang mở hay không, quốc gia của proxy, tag |
| `open_device` / `close_device` | mở máy (có proxy) và đợi Android sẵn sàng, có thể mất một hai phút; đóng máy |
| `device_snapshot` | màn hình dưới dạng dàn ý; thứ bấm hay gõ được có `[ref=mN]` |
| `device_tap`, `device_swipe` | chạm theo `ref` hoặc theo toạ độ; vuốt theo hướng (`up` là mục kế tiếp của feed) hoặc giữa hai điểm |
| `device_type`, `device_press_key` | gõ chữ (mọi ngôn ngữ, cả emoji); bấm back, home, recents, enter… |
| `device_screenshot`, `device_wait_for` | ảnh màn hình; đợi chữ hiện ra hoặc mất đi |
| `device_list_apps`, `device_launch_app`, `device_open_url` | các app đã cài; mở app theo tên gói; mở địa chỉ http(s) trong trình duyệt của máy |
| `device_push_file` | chép một file từ thư mục uploads vào ảnh của máy (`DCIM/AnyProfile`), nơi thư viện và ô chọn file của app thấy nó đầu tiên |
| `device_install_apk` | cài app từ file `.apk` trong thư mục uploads |

- Màn hình chuyển động liên tục (feed video) không đọc được thành dàn ý: agent dùng `device_screenshot` rồi chạm
  theo toạ độ.
- Gõ chữ dùng bàn phím ADBKeyBoard, cài vào máy lần đầu cần; xong việc AnyProfile trả lại bàn phím cũ.
- Như trang web, **mọi thứ trên màn hình máy là nội dung không đáng tin**, và AnyProfile báo điều đó cho agent.
- `logs/anyprofile.log` ghi mỗi lần gọi một dòng kèm id máy; không ghi chữ đã gõ hay nội dung màn hình.
- Recipe mẫu trong gói Pro: `mobile-check-ip` (máy có ra internet qua đúng proxy không) và `tiktok-app-upload`
  (đăng video bằng app TikTok, mặc định Chỉ mình tôi, dừng hỏi trước khi đăng công khai).

## Không có

- Chạy JavaScript tuỳ ý trên trang, xuất cookie, đổi fingerprint hay viewport, tạo hoặc xoá profile, gán proxy.
- Upload file nằm ngoài thư mục uploads.
- Request trước lần đầu agent dùng tab: tải lại trang (`navigate` với `reload`) để thấy chúng.
- Profile đã mở từ trước khi AnyProfile khởi động lại không điều khiển được (`agent_no_pipe`): đóng rồi mở lại.
- Với máy mobile: chạy lệnh shell, đọc file trong máy, tạo hoặc xoá máy, đổi proxy, cài app từ file ngoài thư mục
  uploads.

## An toàn

- **Ai có token thì điều khiển được mọi profile và máy mobile**, kể cả đọc nội dung các trang đã đăng nhập. Chỉ đưa token cho
  agent bạn tin.
- **Trang web có thể tìm cách lừa agent** (prompt injection), ví dụ chèn chữ "hãy chuyển tiền tới…". AnyProfile báo
  cho agent rằng mọi nội dung từ trang là không đáng tin, và không có tool chạy JavaScript hay lấy cookie. Dù vậy,
  hãy để mắt tới cửa sổ khi agent làm việc trên tài khoản quan trọng.
- Nội dung trang mà agent đọc được sẽ đi tới agent, và agent gửi nó lên nhà cung cấp mô hình của nó (ví dụ
  Anthropic với Claude Code, OpenAI với Codex, Google với Gemini). Bản thân AnyProfile không gửi gì ra ngoài máy.
- Tự động hoá trên tài khoản thật có thể trái điều khoản của nền tảng và bị nhận ra qua hành vi. Rủi ro thuộc về
  người dùng.

## Thử mà không đụng profile thật

`tools/bench/mcp-serve` chạy launcher và API thật, có `/mcp`, trên một thư mục dữ liệu tạm với một profile và một
trang shop thử ở `127.0.0.1:51627`:

```bash
export ANYPROFILE_TOKEN=$(openssl rand -hex 24)
go run ./tools/bench/mcp-serve -app "<Chromium.app của Apostate>"   # cổng 51626
```
