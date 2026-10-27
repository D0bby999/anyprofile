---
title: "Phase 06: Synchronizer cho thao tác tay"
status: pending
effort: 5d
---

# Phase 06: Synchronizer cho thao tác tay

## Context Links

- Pipe DevTools sẵn có: `internal/cdppipe/`; agent lái trình duyệt qua cùng pipe này (`docs/mcp.md`).
- Recipes và automation cho agent đã có (`internal/recipe/`) — phase này chỉ lo thao tác tay của người dùng, không thay thế recipes.
- Trần profile đồng thời: Phase 02 (phiên đồng bộ bị giới hạn bởi cùng trần đó).

## Goal

Người dùng thao tác trên một profile chính — bấm, gõ, cuộn, chuyển trang — và các thao
tác đó lặp lại tức thì trên một nhóm profile đích đang mở, để làm việc song song trên
nhiều tài khoản giống nhau mà không cần agent hay script.

## Ranh giới an toàn

- Tắt mặc định; bật đồng bộ là một hành động đích danh cho một phiên, liệt kê rõ profile chính và từng profile đích trước khi bắt đầu.
- Chỉ đi qua pipe DevTools sẵn có; không mở port mới, không phơi sự kiện ra ngoài `127.0.0.1`.
- Không mirror vào profile không nằm trong phiên; đóng một profile đích thì nó rời phiên, không tự quay lại.
- Dừng đồng bộ bằng một nút luôn thấy được và bằng phím tắt; dừng là dừng hẳn, sự kiện đến sau không được phát lại.

## Tasks

### Task 06.1: Phiên đồng bộ ở service

- **Target files:** `internal/service/` (gói đồng bộ mới nếu cần), `internal/launcher/`, test.
- **Steps:**
  1. Kiểu phiên: profile chính, tập profile đích, trạng thái bật/tắt; một profile chính chỉ thuộc một phiên tại một thời điểm.
  2. Bắt đầu phiên kiểm tra mọi profile đích đang mở và cùng trần tài nguyên của Phase 02; profile chưa mở thì báo rõ, không tự mở lén.
  3. Kết thúc phiên khi người dùng dừng, khi profile chính đóng, hoặc khi app đóng.
- **Verify:** Test Go: bắt đầu phiên với một đích chưa mở bị từ chối kèm lý do; đóng profile chính thì phiên kết thúc và sự kiện sau đó không tới đích nào.

### Task 06.2: Bắt và phát lại sự kiện qua pipe

- **Target files:** `internal/cdppipe/`, gói đồng bộ, test.
- **Steps:**
  1. Bắt ở profile chính: click theo toạ độ tương đối theo kích thước trang, gõ phím và văn bản, cuộn, điều hướng.
  2. Phát lại ở từng đích bằng sự kiện đầu vào thật qua pipe (cùng cơ chế agent đang dùng), quy đổi toạ độ theo kích thước trang của đích.
  3. Sự kiện không phát lại được ở một đích (trang khác cấu trúc, phần tử không có) được đếm và báo theo đích, không làm nghẽn các đích còn lại.
- **Verify:** Test tích hợp với hai profile mở trang kiểm thử cục bộ: gõ một chuỗi ở chính thì cả hai đích có đúng chuỗi đó; click một nút ở chính thì đích bấm đúng nút; tắt phiên rồi gõ tiếp thì đích không đổi.

### Task 06.3: Giao diện phiên đồng bộ

- **Target files:** `frontend/src/pages/profiles-page.tsx`, `frontend/src/components/profile-table.tsx`, i18n.
- **Steps:**
  1. Chọn nhiều profile đang mở → hành động "Đồng bộ thao tác" mở dialog chốt profile chính và các đích.
  2. Trong phiên, thanh trạng thái dính ở đầu trang liệt kê chính và các đích, số sự kiện lỗi theo đích, và nút Dừng luôn thấy được.
  3. Profile trong phiên có chip phân biệt chính/đích trong bảng.
- **Verify:** Test frontend: mở dialog từ lựa chọn có một profile chưa mở thì profile đó bị loại kèm lý do; bấm Dừng gọi đúng API và thanh trạng thái biến mất.

## Cook note (2026-10-27, cook tự động)

- **Chưa cook — bỏ qua toàn bộ phase theo chỉ đạo "phần khó tạm bỏ qua".** Lý do: Synchronizer phát thao tác sang nhiều browser thật qua cdppipe; không thể kiểm chứng đúng/sai nếu không chạy browser thật trên macOS, và code viết mù trong vùng nhạy cảm này (điều khiển mọi profile đang mở) rủi ro hơn giá trị của việc làm vội. Backend chưa chạm vào file nào cho phase này.
- Điều kiện để cook tiếp: có phiên làm việc trên Mac chạy được app (dev build) với 2 profile rác mở sẵn để kiểm chứng từng loại thao tác (click, gõ, cuộn) và cơ chế tắt khẩn cấp.

## Phase exit gate

- Mọi Verify đạt, gồm test tích hợp hai profile ở Task 06.2; `make test` và `make lint` đạt.
- Kiểm thử tay một phiên thật trên hai tài khoản phụ trước khi đóng phase.

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
