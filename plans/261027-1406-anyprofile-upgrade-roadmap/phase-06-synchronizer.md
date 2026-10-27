---
title: "Phase 06: Spike và Synchronizer có ranh giới an toàn"
status: pending
effort: 5d
---

# Phase 06: Spike và Synchronizer có ranh giới an toàn

## Hiện trạng và hợp đồng

Chưa có code Synchronizer. Primitive hiện có là `internal/cdppipe` Call/Subscribe;
đó không phải bằng chứng đã capture được input tay. Dùng
[synchronizer contract](contracts/synchronizer.md),
[performance](contracts/performance.md) và [matrix](acceptance-matrix.md).

Estimate 5d cũ chưa tính độ khó capture/iframe/IME/barrier. Task 06.0 timebox
0,5–1d, GO mới re-estimate và cook production; BLOCKED thì báo user, không tự
chuyển sang OS-wide hooks, cắt safety hoặc thêm port để làm cho chạy.

## Task 06.0 — Feasibility spike (gate cứng)

**Files dự kiến:** harness/fixture trong `tools/bench/`, test integration và
report `reports/synchronizer-spike.md` (chưa có). Không bật feature production.

1. Dùng **1 chính + 2 đích = 3 browser**, profile test riêng, origins local.
2. Thử isolated-world injection/binding + CDP replay; kiểm capability từng engine,
   trusted input, loop, event duplication, page nav/tab/frame và Conn ownership.
3. Thử viewport/DPR khác, DOM lệch, iframe/OOPIF, shadow DOM, Unicode/IME/paste;
   field nhạy cảm/page spoofing/rate overflow phải được chặn mặc định.
4. Chứng minh Stop fence/drain, healthy ack <=1 giây và không post-ack input;
   đích treo không được báo Stopped giả hay đóng pipe chung để che lỗi.
5. Đo PERF-09 >=1.000 event mỗi đích; ghi support/capability matrix và raw output.

**Verify:** report có GO/BLOCKED từng required engine/OS, code tái lập, zero loss/
duplicate/order error ở fixture xác định, safety PASS và đủ latency/barrier.
Chỉ có Mac prototype không cho phép tuyên bố Windows đã hỗ trợ. Unsupported cần
đề xuất và user quyết định scope, không tự đánh dấu N/A.

## Task 06.1 — Session service và quyền điều khiển

**Files dự kiến:** `internal/service/`, gói sync riêng nếu cần, launcher/agent
ownership adapters, tests. Chỉ làm sau 06.0 GO và hợp đồng queue Phase 02 sẵn.

1. Session generation, primary/target tab/origin explicit, one active session/app;
   mọi profile đang chạy qua cùng resource ceiling. Không tự mở target.
2. Từ chối profile chưa chạy/adopt không có Conn, target trùng, hai controller
   hoặc agent muốn takeover khi đang sync. Không lén tắt fingerprint capture.
3. Chính đóng/app quit -> fence session; đích đóng -> remove riêng và không auto-
   join khi mở lại. Không resume session sau app restart.
4. Metadata lỗi/counts redacted; payload queue chỉ memory, bounded, không DB/log.

**Verify:** membership/control-ownership/close/restart/spoofed-generation tests;
profile ngoài phiên event count luôn 0. Mọi path cleanup giải phóng lease đúng một lần.

## Task 06.2 — Capture, replay và Stop barrier

**Files dự kiến:** gói sync, `internal/cdppipe/` chỉ khi cần adapter tương thích,
fixture/test integration. Không thêm control listener.

1. Thực thi cơ chế đã chứng minh ở spike; validate source frame/context, allowlist
   origin/input type và hit-test/preconditions ở từng đích trước replay.
2. Sequence và queue riêng/đích; chậm/lỗi một đích không chặn đích khác. Không retry
   key/click chưa rõ kết quả hoặc âm thầm drop input. Overflow pause + resync.
3. Sensitive-field/clipboard/file/OAuth navigation mặc định chặn; custom input
   không phân loại được là unsupported. Không hứa nhận diện mọi secret trong text thường.
4. Stop <=50 ms fence dispatch; ack healthy <=1s sau drain. Timeout giữ trạng thái
   stopping với lỗi rõ; phiên mới không dùng queue/generation cũ. Late replies bị bỏ.

**Verify:** ACC-10 + PERF-09 trên required lanes, tổng 3 browser; kiểm cả kết quả
DOM/input thực, không chỉ CDP ack. Stop khi typing/backlog/navigation/unresponsive;
zero post-ack events, canary credentials không có trong logs/artifacts.

## Task 06.3 — Consent UI và kiểm tay

**Files dự kiến:** profile page/table, dialog/status bar, i18n en/vi, tests.

1. Chọn chính/từng đích/tab/origin, giải thích supported/blocked input và rủi ro
   text thường. Không bật sẵn hoặc thêm profile ngoài danh sách xác nhận.
2. Thanh trạng thái luôn thấy: chính/đích, per-target error count, pause/resync,
   Stop và keyboard shortcut trong app; chip role không làm tràn bảng.
3. UI distinguishes stopped/stopping/target-unresponsive; lỗi đích không được
   biến mất mà tạo cảm giác cả nhóm đã đồng bộ thành công.
4. Kiểm local fixture trước; tài khoản phụ thật sau safety PASS, không quay/chụp
   payload nhạy cảm. Không dùng account thật thay unit/integration tests.

**Verify:** ACC-10/11, en/vi sáng/tối, keyboard/resize, consent/blocked inputs,
Stop luôn dùng được kể cả có backlog. Thử tab/profile chưa mở phải báo đúng.

## Exit gate và failure protocol

- 06.0 GO và mọi Verify production PASS theo matrix, `make test`/`make lint` đạt.
  UI và browser thật bắt buộc; chưa có host/binary đúng thì BLOCKED, không completed.
- Mỗi fail ghi case/lane/revision/capability/input đã redacted + expected/actual,
  owner và next step. Dừng phần phụ thuộc spike; xin user khi cần đổi scope.
- Các test/harness mới chưa có lệnh runnable: phải author và ghi lệnh vào report
  trước khi nghiệm thu, không invent command hay giả `Call/Subscribe` đã đủ.
