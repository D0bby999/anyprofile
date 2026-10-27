# Hợp đồng thử nghiệm và an toàn Synchronizer

**Chưa có implementation.** Chỉ các primitive `cdppipe.Conn.Call/Subscribe` đã
được xác nhận. CDP không có sẵn một stream đầy đủ mọi click/phím người dùng;
không được suy ra rằng có pipe là đã giải được bài toán capture.

## Spike bắt buộc trước production (Task 06.0)

Timebox 0,5–1 ngày làm việc để chứng minh cơ chế trên 1 profile chính + **2 đích**
(tổng 3 browser), bằng local fixture. Spike pass mới ước lượng lại Task 06.1–06.3;
fail/unsupported -> ghi blocker và xin quyết định, không tự cắt tính năng hoặc
chuyển sang OS-wide input hook/control port.

Các ứng viên phải thử chứ chưa phải API được bảo đảm:

1. `Page.addScriptToEvaluateOnNewDocument` trong isolated world +
   `Runtime.addBinding` gắn execution context riêng để nhận DOM input ở chính.
   Kiểm hỗ trợ thực của từng protocol version; scope binding không được phơi cho
   page world. `Target.setAutoAttach`/frame session cần đánh giá cho iframe/OOPIF.
2. Capture trusted user input, tọa độ theo viewport **CSS pixel** và text đã commit;
   replay bằng CDP Input/Page trên pipe đích. Sự kiện replay có thể trusted, nên
   chống loop bằng role primary/target + session generation, không chỉ isTrusted.
3. Chứng minh key/text không nhân đôi (keydown + input + composition), wheel/
   scroll không replay hai lần, điều hướng không lặp vô hạn và không giành focus.
4. Chứng minh không xung đột agent/capture đang dùng chung Conn/Subscribe. Chạy
   test unsubscribe/cancel và renderer/page navigation, không đóng Conn chung chỉ
   để dừng Synchronizer.
5. Đo trên layout giống nhau và khác viewport/DPR; page lệch DOM, iframe cùng/
   khác origin, shadow DOM, tab mới, redirect và field nhạy cảm. Unsupported phải
   có tín hiệu rõ và fail-closed cho event/đích đó; không âm thầm báo thành công.

Output: `reports/synchronizer-spike.md` gồm engine/hash/OS, API capability, sample
code/harness trong repo, support matrix, raw latency, safety tests và quyết định
GO/BLOCKED. Protocol capability/coverage chưa chứng minh phải ghi unverified.

## Phạm vi input và tương thích

- Phiên gắn một tab chính và một tab đích đã chọn ở từng profile. Không chọn tab
  tự động theo vị trí và không broadcast tới mọi tab. Tab mới cần user map trước;
  tab/frame bị thay ngoài allowlist thì pause đích/phiên, không chọn thay lén.
- Bấm/gõ/scroll/điều hướng cùng tab là mục tiêu MVP. Có hỗ trợ hay chưa cho IME,
  paste, cross-origin frame, closed shadow/custom widgets phải có bằng chứng spike;
  chưa hỗ trợ phải báo ngay trước Start. Không gọi phase completed nếu case bắt
  buộc trong matrix chưa chạy hoặc bị bỏ qua mà chưa có user duyệt đổi scope.
- Tọa độ tương đối không bảo đảm bấm đúng nút khi DOM khác. Trước replay cần kiểm
  URL/origin/frame, viewport mapping và hit-test/precondition semantic (tag/role/
  input type) tương thích. Lệch thì block event ở đích + tăng error count, không
  click mù. Không ghi label, DOM text hoặc giá trị input vào log.
- Mọi điều hướng chỉ HTTP(S), không URL có userinfo, file/data/javascript hoặc
  endpoint ngoài origins user đã duyệt. URL có query/fragment chứa token/login/
  OAuth state không được mirror tự động; fail-closed và báo blocked navigation.
  Redirect ra ngoài origins đã duyệt suspend đích cho đến khi user chọn lại.

## Consent và dữ liệu nhạy cảm

- Tắt mặc định; user chọn chính/từng đích/từng tab/origin trước Start. Không target
  nào được tự mở, tự gia nhập hoặc tự quay lại sau close/reopen. Một profile không
  thuộc hai phiên hay vừa là chính vừa là đích; MVP một phiên active/app.
- Chặn mặc định input password, autocomplete current/new-password, one-time-code,
  username/email/tel/payment, file upload, autofill và Clipboard shortcuts. Ctrl/
  Meta hotkeys có thể điều khiển app/clipboard/OS bị chặn; không capture toàn bàn
  phím ngoài tab/browser đã chọn. Không đọc password manager/clipboard để mirror.
- Trường không phân loại được (canvas input, closed shadow, custom secure widget)
  phải coi là unsupported, không đoán là text thường. Paste/IME cần policy spike
  riêng, không bật chỉ vì gõ ASCII đã pass.
- Không thể nhận diện mọi bí mật user gõ vào ô text thường. UI phải nói rõ rủi ro
  đó và chỉ khuyến nghị phiên test/tài khoản phụ; không hứa "không bao giờ truyền
  secret". Consent không cho phép bỏ bảo vệ field nhạy cảm mặc định.
- Payload input/URLs chỉ tồn tại tạm trong bộ nhớ, không persist DB/recipe/telemetry,
  không đưa vào log/error dump/screenshot của acceptance với dữ liệu thật. Report
  chỉ có IDs, type, sequence, counts, mã lỗi và latency đã redacted.
- Binding receiver kiểm role/session generation/frame/execution context đã đăng
  ký và giới hạn schema/size/rate. Page script giả binding/event, navigation đổi
  context hoặc input malformed phải bị từ chối; không nhận command tùy ý từ page.

## Vòng đời, thứ tự và Stop barrier

- Mọi event có session generation, sequence và capture timestamp. Controller chỉ
  phát từ chính, có queue riêng từng đích; đích chậm không chặn đích khác. Queue
  tối đa 256 events/đích, một lệnh input in-flight/đích để giữ thứ tự.
- Không drop/reorder/retry key/text/click âm thầm. Overflow -> pause đích + thông
  báo resync. Chỉ coalesce scroll/move khi đã chứng minh không đổi semantics;
  không retry command input đã gửi mà chưa biết kết quả (tránh gõ/bấm hai lần).
- Agent cần quyền điều khiển trên profile thuộc phiên phải bị từ chối hoặc user
  dừng phiên trước; không hai controller tranh cùng tab. Capture fingerprint read
  only không bị tắt lén. Khóa điều khiển giải phóng khi Stop/crash, không ảnh hưởng
  các profile ngoài phiên.
- Nút Stop luôn thấy, phím tắt trong app có thể focus được; user đóng chính/app
  cũng fence session. Trong <=50 ms từ yêu cầu Stop, controller ngừng enqueue/
  dispatch event mới, gỡ receiver/subscription và invalidates generation.
- Ack **Stopped** chỉ sau drain/ack mọi lệnh in-flight (đã gửi không thể thu hồi).
  Case healthy cần ack <=1 giây, sau ack không có input mới tại đích. Không undo
  thao tác đã thực hiện trước barrier. Spike phải chứng minh CDP ack + quan sát
  kết quả đủ bảo đảm barrier, không giả định từ timeout Call.
- Đích treo: trong <=1 giây báo `stopping/target_unresponsive`, giữ fence và không
  báo Stopped giả hay cho phiên mới chạy trên đích chưa drain. User có lựa chọn
  đóng browser đích để chấm dứt. Không kill browser/đóng pipe chung tự động.
- Event/response đến muộn của generation cũ bị bỏ; phiên mới không dùng lại queue,
  target mapping hoặc buffer cũ. Crash app không tự bật lại phiên khi restart.

## Gate đo và safety

- >=1.000 event test trên mỗi đích của tổng 3 browser; key/text/click xác định
  không mất/nhân đôi/đảo thứ tự, latency theo PERF-09 của [performance](performance.md).
- Với Stop ở lúc idle/typing/backlog/navigation/đích treo: fence đúng, healthy ack
  đúng deadline, zero post-ack input trên fixture; late generation không replay.
- Chính đóng -> stop toàn phiên; một đích đóng -> remove đúng đích, đích khác tiếp
  tục; reopen không auto-join. Profile không được chọn luôn giữ event count 0.
- Credential/OTP/file/clipboard/page binding spoofing/rate overflow đều có test
  từ chối; log scan không có canary input nhạy cảm. Fixture account không dùng secret thật.
- Kiểm capability và an toàn trên từng lane OS/engine bắt buộc. Lane thiếu binary
  đúng hash hoặc không có môi trường thật là BLOCKED, không lấy cross-compile thay.
