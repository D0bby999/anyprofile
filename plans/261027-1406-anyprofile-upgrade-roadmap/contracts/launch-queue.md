# Hợp đồng hàng đợi mở browser

**Thiết kế cần triển khai thêm.** Code ở `b6f7b78` đã có FIFO/cancel/adopt/StopAll,
nhưng `StartBehind` và binding mở vẫn đợi browser. Các tên API dưới đây là đề
xuất nội bộ, chưa phải endpoint đã tồn tại. Không thêm control port.

## API và quyền sở hữu

- `SubmitLaunch(profileId)` trả `LaunchRequest` ngay sau validate/reserve; không
  đợi engine I/O, proxy check hoặc browser ready. Schema: `requestId`, `profileId`,
  `state`, `queuePosition`, `reason`, `createdAt`, `errorCode`, `revision`.
- `SubmitLaunchMany(profileIds)` nhận <=100 ID/lần, deduplicate và trả kết quả
  từng ID (accepted/existing/rejected cùng mã lỗi). Yêu cầu vượt trần CPU không
  là rejected: nó được accepted ở trạng thái queued. UI batch lớn chia đợt có
  hiển thị tiến độ, không khóa các nút hủy/đóng khi còn request pending.
- `GetLaunchRequests()` trả snapshot có revision; event chứa cùng requestId và
  revision tăng. UI bỏ event cũ, có thể refresh snapshot khi mất event/reconnect.
- `CancelLaunch(requestId)` idempotent cho queued/starting/cancelled. Nếu đã
  running, trả `already_running` và state mới; **không** đổi ngầm thành Stop browser.
  ID generation cũ không bao giờ hủy một lần mở mới của cùng profile.
- StopProfile tương thích: hủy pending hiện tại hoặc đóng browser hiện tại.
  Trash/Purge chờ cancel/cleanup thành công trước khi sửa/xóa dữ liệu profile.
- Legacy `Start/StartBehind`, Local API và MCP vẫn có ngữ nghĩa chờ ready hiện tại,
  nhưng dùng chung scheduler/admission; không có đường mở vượt trần. Adapter wait
  chỉ hủy request nó sở hữu khi context caller hết hạn; không hủy browser đã chạy
  hay request existing của caller khác. UI chuyển sang Submit + events.
- Sau acceptance, request thuộc app, không thuộc vòng đời HTTP/bridge call. Caller
  ngắt kết nối không tự hủy job UI đã được nhận. Trước acceptance, context bị hủy
  thì không có request/slot/process nào được tạo.

## State machine và thứ tự

`queued -> starting -> running -> stopping -> stopped`; nhánh queued/starting có
thể về `cancelled` hoặc `failed`. `stopping`/cleanup vẫn giữ slot đến khi process
kết thúc; running là thành công của thao tác mở, không có nghĩa browser sống mãi.

- Một request active/profile; submit trùng trả request existing ở queued/starting/
  running. Profile đang stopping trả conflict rõ ràng, không tự xếp lần mở sau.
- FIFO theo thứ tự accept ở scheduler, không theo thứ tự goroutine được CPU chạy.
  `queuePosition` là 1-based trong toàn app và 0 nếu không queued. Cancel/tăng trần/
  release cập nhật position + revision trước khi phát event mới.
- Chỉ một nơi sở hữu slot và nhả đúng một lần cho mỗi launch/adopt. Starting và
  browser adopt tính vào trần; queued không tính. Adopt vượt trần thì giữ browser
  đang chạy và chặn mở mới, không kill browser để ép số đẹp.
- Giảm trần không kill browser đang chạy; hàng đợi chờ đủ slot. Tăng trần đánh thức
  FIFO. Mọi nguồn UI/bulk/API/MCP chia cùng pool/trần, không lane đặc quyền.
- Cancel racing admission: hoặc cancelled và không spawn, hoặc đã starting thì
  cancel cleanup cây process/pipe trước ack. Nếu đã running, Cancel trả conflict
  để user quyết định Stop; không báo thành công hủy một browser đang chạy.
- StopAll: fence không nhận submit mới; hủy queued/starting trước khi nhả slot,
  đợi cleanup; nhiều StopAll đồng thời cùng đợi cùng barrier. Không spawn lén lúc
  thoát. Nếu cleanup lỗi phải trả lỗi/trạng thái còn sống, không giả stopped.

## Restart, thông tin lưu trữ và lỗi

- Queue/request history chỉ ở bộ nhớ, không chứa proxy credentials, args nhạy cảm
  hoặc nội dung nhập. Sau restart, queued/starting không tự resume/retry; browser
  sống được Adopt theo PID + start time/lock và có request generation mới.
- Terminal history tối đa 500 entries và TTL 15 phút; chỉ mã lỗi đã redacted.
  UI đang giữ request cũ sau restart phải refresh, không dùng cancel ID cũ.
- FIFO không có nghĩa một request lỗi giữ chỗ vô hạn: startup có deadline, cleanup
  rồi failed; user chủ động Retry tạo generation mới, không retry ẩn.
- Kiểm hash/engine missing vẫn fail-closed. Proxy lỗi không đổi sang direct để
  làm queue chạy tiếp; fail request đó và cho waiter hợp lệ tiếp theo.

## Cài đặt trần và migration

Thiết kế mới thêm mode `auto | manual | unlimited` thay vì diễn giải mọi giá trị
lỗi thành 0. Giữ khả năng unlimited có lựa chọn, không coi nó là resource protection.

| Dữ liệu cũ | Chuyển đổi |
|---|---|
| `max_concurrent_launches > 0` hợp lệ | manual, giữ nguyên số |
| `max_concurrent_launches = 0` được lưu rõ ràng | unlimited, giữ ý định cũ |
| Chưa có setting | auto |
| Setting/mode hỏng | auto + cảnh báo sửa setting, không âm thầm unlimited |

Auto dùng công thức đã đo ở [performance](performance.md). Nếu chưa có baseline
phù hợp engine/platform: fallback tạm cap 2, ghi rõ **provisional**, user vẫn sửa
manual hoặc chọn unlimited có cảnh báo. Fallback không thay thế gate đo RAM và
không được dùng để đóng Phase 02. Thay đổi fallback này phải được migration/test
phủ; test cũ `bad stored value -> 0` sẽ phải đổi, không giữ cả hai hợp đồng trái nhau.

Settings giải thích mode, trần, tác dụng giảm/tăng và unlimited. Status bar tách
`running`, `starting`, `queued`, `ceiling`, `overLimitFromAdopt`; RAM chỉ ghi
**ước tính** với phiên bản mẫu, không giả số đo live. Không auto-kill theo RAM.

## Kiểm chứng bắt buộc

- Submit 10/trần 2: ack đạt PERF-07; tổng starting+running+stopping <=2; thứ tự
  admit khớp acceptance sequence. Cancel #3, release một slot -> #4 được admit.
- Duplicate submit, cancel head/middle, cancel trước accept/racing spawn/đã running,
  stale request ID, context timeout của waiter, tăng/giảm trần và corrupt settings.
- Adopt 3/trần 2: không mở mới cho tới khi đủ slot; StopAll không mở waiter nào;
  crash/restart không resume queue. PID reuse không được adopt/kill nhầm.
- Đóng thường, kill/crash browser, cancel startup và quit app: không còn process
  thuộc profile/pipe/launch record sau 2 giây tính từ ack cleanup; mỗi ca 3 lượt
  trên các lane engine/OS bắt buộc trong [matrix](../acceptance-matrix.md).
- UI: positions/revisions cập nhật đúng; hủy được cả khi batch mở đang pending;
  stale Cancel khi browser vừa chạy chỉ báo refresh/Stop tùy user chọn.
