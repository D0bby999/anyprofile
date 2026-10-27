---
title: "Phase 02: Giới hạn tài nguyên khi mở nhiều profile"
status: in_progress
effort: 2d
---

# Phase 02: Giới hạn tài nguyên khi mở nhiều profile

## Context Links

- Ngưỡng trần đồng thời: `reports/baseline.md` của Phase 01 (bắt buộc xong trước).
- Mở/đóng profile: `internal/launcher/`, `internal/service/`.
- Trạng thái đang chạy đã có ở sidebar: View `running` và status bar (`frontend/src/components/sidebar.tsx`, `App.tsx`).

## Goal

App không để người dùng (hoặc một lần mở hàng loạt) mở quá số profile mà máy chịu được.
Profile vượt trần vào hàng đợi minh bạch, đóng một profile là profile đang chờ tự chạy,
và tiến trình browser con luôn được dọn sạch khi profile đóng.

## Tasks

### Task 02.1: Trần đồng thời và hàng đợi ở service

- **Target files:** `internal/service/`, `internal/launcher/`, test đi kèm.
- **Steps:**
  1. Thêm cài đặt trần số profile mở đồng thời, mặc định theo ngưỡng Phase 01, người dùng sửa được trong Settings.
  2. Yêu cầu mở vượt trần trả trạng thái "đang chờ" kèm vị trí trong hàng đợi, không báo lỗi.
  3. Khi một profile đóng, profile đầu hàng đợi tự mở; huỷ chờ được từ giao diện.
  4. Mở hàng loạt (bulk) tôn trọng cùng trần và hàng đợi này, không có đường vòng.
- **Verify:** Test Go: đặt trần 2, mở 4 profile → đúng 2 chạy, 2 chờ; đóng 1 → đúng 1 profile chờ tự chạy; `go test ./internal/service/ ./internal/launcher/ -count=1` in `ok`.

### Task 02.2: Dọn tiến trình khi đóng

- **Target files:** `internal/launcher/`.
- **Steps:**
  1. Kiểm kê đường đóng hiện tại: đóng thường, app thoát, profile crash.
  2. Đảm bảo cây tiến trình browser và pipe DevTools của profile đều kết thúc ở cả ba đường; app thoát giữa chừng vẫn fail-closed như hiện tại.
- **Verify:** Test tích hợp mở rồi đóng profile, sau 2 giây không còn tiến trình con của profile đó (kiểm bằng danh sách tiến trình trong test); chạy 3 lần liên tiếp đều đạt.

### Task 02.3: Giao diện hàng đợi và mức dùng

- **Target files:** `frontend/src/App.tsx` (status bar), `frontend/src/components/profile-table.tsx`, i18n.
- **Steps:**
  1. Status bar hiện "đang mở X / trần Y" và tổng RAM ước tính của các profile đang mở (số đo Phase 01).
  2. Profile đang chờ hiện chip "Đang chờ" kèm nút huỷ chờ trong bảng.
  3. Khi trần bị chạm lúc mở tay, toast nói rõ profile đã vào hàng đợi thay vì im lặng.
- **Verify:** Test frontend: trạng thái chờ render đúng chip và nút huỷ; bấm huỷ gọi đúng API; `FE_TEST` đạt.

## Cook note (2026-10-27)

Cook bằng Muse (ak-cook 2.5.1, files-first; trạng thái phase giữ ở frontmatter, tiến độ ghi tại đây):

- **Task 02.1 phần lõi đã xong trong `internal/launcher`:** thêm `gate.go` (bộ giới hạn số browser đang mở/đang khởi động, trần 0 = không giới hạn như hành vi cũ) và nối vào `launcher.go`/`stop.go` — `open()` lấy một slot trước khi tạo tiến trình, nhả slot đúng một lần khi tiến trình rời khỏi launcher (mở lỗi hoặc browser thoát). API công khai: `SetMaxRunning`, `MaxRunning`, `WaitingLaunches`. Browser được adopt từ lần chạy trước không qua cổng và không nhả slot.
- Kiểm chứng trên clone GitHub của repo: biên dịch chéo `GOOS=windows` cho toàn bộ package launcher đạt; 5 test của gate (không giới hạn không chặn, chặn ở trần tới khi nhả slot, huỷ theo context, nâng trần đánh thức hàng đợi, nhả thừa không âm) chạy đạt độc lập trên Linux. Package launcher không biên dịch được trên Linux (thiếu bản `workArea` cho Linux) và bản macOS cần cgo nên chưa chạy được ở máy Muse.
- **Chưa xong:** khoá cài đặt + nối `SetMaxRunning` lúc app khởi động (phần còn lại của Task 02.1 — trần hiện chưa được app đặt nên hành vi app chưa đổi), Task 02.2 (RAM) và Task 02.3 (chip hàng đợi trên UI). Cần chạy trên Mac: `go test ./internal/launcher/ -run Gate -v` và `make build` để xác nhận bản macOS.

## Cook note bổ sung (2026-10-27, lượt 2)

- **Task 02.1 hoàn tất phần backend:** khoá cài đặt `max_concurrent_launches` (`internal/store/settings.go`), `Service.MaxConcurrentLaunches/SetMaxConcurrentLaunches/ApplyLaunchCeiling` (`internal/service/settings.go`) — đặt trần là áp dụng ngay vào launcher, giá trị lưu hỏng đọc thành 0 (không giới hạn) để không bao giờ khoá người dùng ngoài app; binding `App.MaxConcurrentLaunches/SetMaxConcurrentLaunches` (`app_bindings.go`); `app.go` gọi `ApplyLaunchCeiling()` lúc khởi động. Mặc định chưa đặt = 0 = không giới hạn, hành vi hiện tại không đổi cho tới khi có UI/ người dùng đặt trần. Test mới: `internal/service/settings_launch_test.go` (3 test).
- Kiểm chứng trên clone: `GOOS=windows go build ./...` đạt toàn repo (bản clone cần file giữ chỗ `frontend/dist`), `go vet` đạt cho store/service/launcher/root, gofmt sạch. Test service chưa chạy được ở máy Muse vì package launcher không có bản Linux; cần chạy trên Mac: `go test ./internal/service/ -run 'MaxConcurrentLaunches|ApplyLaunchCeiling' -v` và `go test ./internal/launcher/ -run Gate -v`, cộng `make build`.
- Khi đối chiếu, phát hiện `app.go` trên Mac có một dòng local chưa lên GitHub (`a.svc.OfferMobileRuntime()`); dòng này được giữ nguyên, phần sửa chỉ thêm đúng một dòng gọi `ApplyLaunchCeiling()`.
- Còn lại của phase: Task 02.2 (cảnh báo RAM — nên chờ số RAM/profile ở Phase 01 Task 01.1 để đặt ngưỡng) và Task 02.3 (hàng đợi trên UI: chip "đang chờ" + nút huỷ + mục trần trong trang Cài đặt — backend đã sẵn binding).

## Cook note (2026-10-27, cook tự động — trạng thái 02.2/02.3)

- **Task 02.2 (cảnh báo RAM) chưa cook:** chờ số RAM/profile từ Phase 01 Task 01.1 để đặt ngưỡng có căn cứ; ghi lại ở `reports/cook-auto-2026-10-27.md`.
- **Task 02.3 (chip hàng đợi + mục trần trong Settings) chưa cook:** phần frontend tạm hoãn theo chỉ đạo bỏ qua phần khó; backend đã sẵn `WaitingLaunches()` và binding trần để UI dùng.

## Cập nhật sau review (2026-10-27)

Các Cook note cũ gọi Task 02.2 là "cảnh báo RAM" là nhầm: nhiệm vụ gốc là **dọn
cây tiến trình khi đóng**. Không được hoãn nhiệm vụ này vì thiếu số RAM.

Đã sửa FIFO, hủy theo context, đăng ký profile đang chờ để Stop/Trash tìm được,
chặn yêu cầu mở trùng, tính cả browser Adopt vào trần, và hủy mọi yêu cầu chờ trước
khi StopAll nhả slot. Bảng và drawer có nút hủy chờ (en/vi).

Integration Chromium thật đã kiểm mở 4/trần 2, hủy một yêu cầu chờ, đóng browser
để nhường slot và kiểm không còn cây tiến trình sau StopAll; chạy 3 lần đạt.
Lifecycle kiểm pipe/cookie/fingerprint cũng đạt 3 lần. Chưa đủ để đóng cả phase:
chưa có setting trần trên UI, vị trí hàng đợi/status bar/RAM estimate, trần mặc
định dựa trên số đo và kiểm UI trên app thật. Binding mở vẫn chờ hoàn tất/hủy,
chưa phải API enqueue trả ngay kèm vị trí như spec.

## Phase exit gate

- Mọi Verify đạt; `make test` đạt.

## Failure Protocol

If any Verify step does not meet its stated pass condition, STOP this phase.
Do not improvise a fix, retry blindly, or reason around the failure.
Spawn the `kongming` subagent for next-step counsel and pass:
- the phase and task id,
- what you attempted (the steps you ran),
- the exact command and its full output,
- the pass condition it failed to meet.
Apply kongming's guidance, then re-run the Verify step.
If `kongming` cannot be spawned in this environment, STOP and report the same failure evidence to the user. Never continue by self-reasoning.
