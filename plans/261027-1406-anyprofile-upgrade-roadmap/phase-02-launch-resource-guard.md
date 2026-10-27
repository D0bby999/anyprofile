---
title: "Phase 02: Launch queue bất đồng bộ và giới hạn tài nguyên"
status: in_progress
effort: 2d
---

# Phase 02: Launch queue bất đồng bộ và giới hạn tài nguyên

## Hợp đồng và hiện trạng

Bắt buộc theo [launch-queue](contracts/launch-queue.md),
[performance](contracts/performance.md) và [matrix](acceptance-matrix.md).
Ước lượng 2d là lịch sử; re-estimate khi chốt async adapter/migration.

Đã có: gate FIFO/cancel, dedupe pending, tính Adopt, StopAll và queued/cancel trên
bảng/drawer. Chưa có: request API trả ngay, queue position/revision, Settings mode,
status bar/RAM estimate và migration mode. Bằng chứng cũ ở
[review fixes](reports/review-fixes-2026-10-27.md), không chứng minh hợp đồng mới.

## Task 02.1 — Scheduler/request API và compatibility

**Files:** `internal/launcher/`, `internal/service/`, `app_bindings.go`,
`frontend/src/api.ts`, bindings generated; tests cùng package.

1. Viết regression contract trước: acceptance sequence FIFO, submit trùng, stale
   cancel ID, cancel racing admit/spawn/running và caller context hết hạn.
2. Thêm request identity/generation/state/revision và positions; async Submit nhận
   việc rồi trả ngay. Lifecycle app sở hữu accepted jobs; history bounded/ephemeral.
3. Legacy Start/StartBehind/API/MCP wait-ready đi qua cùng scheduler, không đổi
   semantics âm thầm. Hủy waiter chỉ tác động job nó sở hữu, không browser đã chạy.
4. Bulk trả per-item ack, UI không await toàn bộ browser trước khi cho phép hủy/đóng.
5. Đo PERF-07; các error chỉ mã redacted, không có credentials/args nhạy cảm.

**Verify:** Submit 10/trần 2, cancel đầu/giữa, release/tăng/giảm trần, duplicate,
legacy callers và crash/restart đạt ACC-03; p95 service <=100 ms. Phải có test
positions/revisions/late events, không chỉ test semaphore. Test mới chưa tồn tại.

## Task 02.2 — Cleanup và shutdown, không phải cảnh báo RAM

**Files:** `internal/launcher/` + tests macOS/Windows integration.

1. Giữ slot qua cleanup, nhả đúng một lần; Adopt vượt trần không kill browser cũ.
2. Stop/Trash/Purge chờ pending cancellation; app quit fence submissions, hủy
   pending trước khi đóng running; concurrent StopAll chung barrier.
3. Kiểm normal close, startup failure/cancel, browser crash, app crash/restart,
   PID reuse và pipe teardown. Không đóng/kill profile ngoài fixture.

**Verify:** ACC-04, cây process/pipe/launch record không còn sau 2 giây tính từ
ack cleanup, mỗi case 3 lần/lane required. Integrity mismatch là BLOCKED, không
bỏ check để chạy Fury cũ. Thiếu số RAM không chặn làm đúng cleanup.

## Task 02.3 — Mode/default migration và UI

**Files:** `internal/store/settings.go` + migration nếu cần,
`internal/service/settings.go`, `app.go`, Settings/status bar/table/drawer, i18n/tests.

1. Migrate explicit old 0 -> unlimited, positive -> manual, missing -> auto,
   corrupt -> auto + cảnh báo theo hợp đồng; không làm mất lựa chọn cũ của user.
2. Auto dùng baseline Phase 01. Chưa có R95 thì cap 2 provisional có nhãn rõ,
   user vẫn đổi được; không gọi provisional là đã đạt default nghiệm thu.
3. Settings cho mode/trần và lời cảnh báo; status bar tách running/starting/queued,
   over-limit do Adopt và RAM **ước tính**. Không bịa RAM live.
4. Bảng/drawer có position + lý do chờ + hủy; toast acceptance không chờ ready.
   Cancel khi browser vừa running chỉ refresh/báo user chọn Stop, không đóng lén.
5. Không dùng global busy của bulk-open để khóa nút hủy/Stop các request khác.

**Verify:** ACC-05/11: migration missing/corrupt/0/positive; en/vi sáng/tối,
keyboard, async errors/late events/reconnect. Default 16/32 GiB có số RAM thật,
không suy từ SQL benchmark. Các test fallback cũ ->0 phải đổi theo policy mới.

## Gate và lệnh

- ACC-03/04/05 cùng phần liên quan ACC-09/11/12 PASS trên required lanes.
- Lệnh hiện có: `go test ./internal/launcher ./internal/service -count=1`,
  `go test -race ./internal/launcher ./internal/service -count=1`, `make test`,
  `make lint`; integration đúng env/engine theo matrix. Tests async mới phải được
  viết/ghi lệnh runnable trước khi coi Task 02.1 đạt.
- Chưa đóng phase khi còn provisional baseline, UI chưa kiểm hoặc legacy adapter
  bypass scheduler. Không đòi worktree sạch nếu user chưa yêu cầu commit.

## Failure protocol

Fail Verify thì giữ phase mở, ghi case/lane/revision/command/output/expected và
owner + next action. Chỉ rerun sau khi đã xử lý nguyên nhân; không tăng timeout,
bỏ hash check hay đổi tiêu chí để làm xanh. Nếu cần thay sản phẩm/scope, dừng task
liên quan và xin user quyết định; phase độc lập chỉ tiếp tục khi dependency cho phép.
