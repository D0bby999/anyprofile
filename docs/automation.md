# Automation API cho script

API trên máy (Settings → Local API) phục vụ cả MCP cho agent và HTTP cho script.
Bật API, chép token, rồi dùng một trong ba cách dưới đây.

```sh
API=http://127.0.0.1:51525
AUTH="Authorization: Bearer $ANYPROFILE_TOKEN"
```

## Một hành động

```sh
curl -s -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"profile":"Shop VN 01","action":"find","target":{"text":"Đơn hàng"}}' \
  $API/v1/act
```

Kết quả: `{"ok":true,"result":[{"ref":"e41","role":"button","name":"Đơn hàng","enabled":true}]}`.
`click` nhận cùng `target`; nếu không chắc, gọi `find` trước.

## Một flow

```sh
curl -s -H "$AUTH" -H 'Content-Type: application/json' \
  -d @examples/automation/flow.json $API/v1/run
```

`/v1/run` trả `{"ok":true,"steps":[{"ok":true,"result":…},…]}`; dừng ở bước lỗi đầu
tiên trừ khi `"stopOnError": false`.

## CLI

```sh
export ANYPROFILE_TOKEN=…
sdk/typescript/bin/anyprofile.mjs run examples/automation/flow.json
sdk/typescript/bin/anyprofile.mjs run flow.json --var ORDER_ID=123
```

`--dry-run` in ra flow sau khi thay biến, không gọi API. Biến lấy từ `--var` hoặc môi trường;
flow dùng `{{ORDER_ID}}`.

## TypeScript

```ts
import { session } from './sdk/typescript/src/client.ts'

const s = await session({ profile: 'Shop VN 01' })
await s.goto('https://seller.shopee.vn/portal/order')
await s.click({ text: 'Đơn hàng' })
const rows = await s.find({ text: 'Chờ lấy hàng' }, 10)
console.log(await s.read())
await s.close()
```

## Python

```python
from anyprofile.client import Session, Target

s = Session(profile="Shop VN 01")
s.goto("https://seller.shopee.vn/portal/order")
s.click(Target(text="Đơn hàng"))
print(s.find(Target(text="Chờ lấy hàng")))
```

## Hành động

| Nhóm | Hành động |
|---|---|
| Browser | `navigate`/`goto`, `find`, `click`, `type`, `read`, `press`, `scroll`, `wait`, `screenshot`, `upload`, `network`, `body`, `hover`, `double_click`, `right_click`, `drag` |
| Mobile | `find`, `tap`, `tap_text`, `type`, `key`, `swipe`, `long_press`, `drag`, `launch`, `open_url`, `screenshot`, `pull` |

Mọi hành động nhận `target` (xem `docs/mcp.md`); `click`/`tap`/`type` cũng nhận `ref`.
Không cần `sleep`: target được chờ tới 5 giây, phải ổn định và bật.

## Lỗi

`400` cho target/action sai (`target_not_found`, `target_ambiguous`, `target_disabled`,
`api_action`, …), `409` cho trạng thái app (`agent_not_open`, `mobile_not_running`),
`401`/`403` cho token/Origin/Host. Dạng:

```json
{"error":{"code":"target_not_found","message":"no element matches text \"Lưu\" after 5s"}}
```

## Session (khoá profile/device)

```sh
curl -s -H "$AUTH" -H 'Content-Type: application/json'   -d '{"profile":"Shop VN 01","ttlSeconds":60}' $API/v1/sessions
# {"id":"s_…","key":"profile:…","expiresAt":"…"}

curl -s -X POST -H "$AUTH" $API/v1/sessions/s_…/heartbeat
curl -s -X DELETE -H "$AUTH" $API/v1/sessions/s_…
```

Gửi `"session":"s_…"` trong `/v1/act` hoặc `/v1/run` để giữ khoá. Session thứ hai cho cùng profile nhận
`409 profile_busy` kèm id đang giữ. Không gửi `session` thì API vẫn chạy như trước (tương thích ngược), nhưng
hai script có thể giành tab — nên dùng session khi chạy song song.

## Ghi lại thành flow

```sh
anyprofile record --profile "Shop VN 01" --out flow.json
# in ra session id; gửi các bước với "session": "<id>"
# Ctrl-C để lưu
```

Server ghi các hành động có ý định (`navigate`, `click`, `type`, `tap`, …), **bỏ qua** `find`/`read`/`screenshot`
vì đó là quan sát. Bước có `"secret": true` chỉ lưu `{{SECRET}}`, không lưu chữ đã gõ. File lưu là flow JSON chạy
lại được bằng `anyprofile run`.

## Đo hiệu quả

```sh
ANYPROFILE_TOKEN=… scripts/automation-eval.sh build/automation-eval
# build/automation-eval/results.csv: flow,result,seconds
```

Bộ 10 task dùng để đánh giá agent (xem `plans/261101-1301-agent-automation-first/phase-07-docs-acceptance.md`):
đọc trang, bấm theo chữ, điền form, đọc 10 dòng, upload, wait, dialog, hai tab, mobile mở app, mobile gõ tiếng Việt.
Mục tiêu: ≤ 6 tool call cho task "mở → đọc 10 dòng", 9/10 task không cần gợi ý lại, script ≤ 20 dòng và 0 `sleep`.

**Chưa có số đo thật** trong môi trường build này: cần một agent đang kết nối MCP, token Local API và tài khoản
thật cho các site. Đừng ghi số ước lượng vào báo cáo.

## Trace

Mỗi lượt gọi MCP ghi `logs/traces/mcp-YYYY-MM-DD.jsonl`; xem 50 dòng cuối:

```sh
curl -s -H "$AUTH" "$API/v1/trace?lines=50"
```

Chưa có session/lease (Phase 5) nên chưa có `/v1/sessions`.
