# AI agent (MCP)

Claude Code và các AI agent khác nói chuyện được với AnyProfile qua MCP. Agent mở profile rồi làm việc ngay
trong trình duyệt của profile đó: đọc trang, bấm, gõ, chụp màn hình, xem request mạng và body response. Trình
duyệt dùng đúng tài khoản đã đăng nhập, cookie, proxy và fingerprint của profile. Cửa sổ vẫn hiện trên màn hình,
nên bạn nhìn thấy agent đang làm gì.

MCP nằm trong [API trên máy](local-api.md), dùng chung công tắc và token. API tắt thì MCP cũng tắt.

## Kết nối Claude Code

1. Settings → **Local API** → **On**.
2. Ở mục **AI agent (MCP)**, bấm **Chép lệnh cho Claude Code**, rồi dán lệnh vào terminal. Lệnh có dạng:

   ```bash
   claude mcp add --transport http anyprofile http://127.0.0.1:51525/mcp --header "Authorization: Bearer <token>"
   ```

   Lệnh chứa token và tự xoá khỏi bộ nhớ tạm sau 90 giây. Claude Code lưu token trong cấu hình của nó. Muốn thu
   hồi thì bấm **Token mới**, rồi chép lại lệnh.
3. Trong Claude Code, gõ `/mcp` để kiểm tra rằng `anyprofile` đã kết nối, rồi giao việc. Ví dụ: "Mở profile Shop
   VN 01, vào trang đơn hàng, đọc 10 đơn mới nhất."

Client khác (Cursor, Codex, Claude Desktop…) dùng địa chỉ `http://127.0.0.1:<cổng>/mcp`, kiểu **Streamable HTTP**,
và header `Authorization: Bearer <token>`. Ví dụ:

```json
{ "mcpServers": { "anyprofile": { "type": "http", "url": "http://127.0.0.1:51525/mcp",
  "headers": { "Authorization": "Bearer ${ANYPROFILE_TOKEN}" } } } }
```

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

Agent chỉ upload được file nằm trong **thư mục uploads** (Settings → Local API → Mở thư mục uploads; trên đĩa là
`uploads` trong thư mục dữ liệu của app). Đường dẫn tuyệt đối, `..` và symlink trỏ ra ngoài đều bị từ chối, vì một
trang độc có thể lừa agent tải file riêng tư của bạn lên. Thư mục con được phép (`clips/a.mp4`).

`upload_file` nhận hai kiểu `ref`: một ô `<input type=file>`, hoặc nút mở hộp chọn file (như "Select files" của
YouTube Studio, nơi ô chọn file bị ẩn). Với kiểu thứ hai, AnyProfile chặn hộp chọn file của hệ điều hành, bấm nút
bằng chuột thật, rồi đặt file vào ô mà trang vừa mở. Trong lúc đó miền `Page` được bật rồi tắt ngay; nó không để
lại dấu vết mà script của trang thấy được.

## Recipe

Recipe là các bước làm một việc trên một nền tảng, viết cho agent: một file `.md` trong **thư mục recipes**
(Settings → Local API → Mở thư mục recipe).

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
- Claude Code hiện mỗi recipe thành lệnh `/mcp__anyprofile__<name>`. Agent cũng tự tìm được bằng tool `recipe`.
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
  Focus ở dòng của profile. Nếu Chromium vẫn tự nhảy lên lúc khởi động, AnyProfile ẩn nó một nhịp để macOS trả focus
  cho app trước đó; có thể thấy cửa sổ loé lên một chút.
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

## Không có

- Chạy JavaScript tuỳ ý trên trang, xuất cookie, đổi fingerprint hay viewport, tạo hoặc xoá profile, gán proxy.
- Upload file nằm ngoài thư mục uploads.
- Request trước lần đầu agent dùng tab: tải lại trang (`navigate` với `reload`) để thấy chúng.
- Profile đã mở từ trước khi AnyProfile khởi động lại không điều khiển được (`agent_no_pipe`): đóng rồi mở lại.

## An toàn

- **Ai có token thì điều khiển được mọi profile**, kể cả đọc nội dung các trang đã đăng nhập. Chỉ đưa token cho
  agent bạn tin.
- **Trang web có thể tìm cách lừa agent** (prompt injection), ví dụ chèn chữ "hãy chuyển tiền tới…". AnyProfile báo
  cho agent rằng mọi nội dung từ trang là không đáng tin, và không có tool chạy JavaScript hay lấy cookie. Dù vậy,
  hãy để mắt tới cửa sổ khi agent làm việc trên tài khoản quan trọng.
- Nội dung trang mà agent đọc được sẽ đi tới agent, và agent gửi nó lên nhà cung cấp mô hình của nó (ví dụ
  Anthropic với Claude Code). Bản thân AnyProfile không gửi gì ra ngoài máy.
- Tự động hoá trên tài khoản thật có thể trái điều khoản của nền tảng và bị nhận ra qua hành vi. Rủi ro thuộc về
  người dùng.

## Thử mà không đụng profile thật

`tools/bench/mcp-serve` chạy launcher và API thật, có `/mcp`, trên một thư mục dữ liệu tạm với một profile và một
trang shop thử ở `127.0.0.1:51627`:

```bash
export ANYPROFILE_TOKEN=$(openssl rand -hex 24)
go run ./tools/bench/mcp-serve -app "<Chromium.app của Apostate>"   # cổng 51626
```
