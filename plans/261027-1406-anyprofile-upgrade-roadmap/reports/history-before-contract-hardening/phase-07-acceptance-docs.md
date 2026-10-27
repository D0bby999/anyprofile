---
title: "Phase 07: Kiểm thử chấp nhận, tài liệu và CHANGELOG"
status: in_progress
effort: 1d
---

# Phase 07: Kiểm thử chấp nhận, tài liệu và CHANGELOG

## Context Links

- Kịch bản kiểm thử tay: `docs/manual-testing.md`.
- Nhật ký thay đổi: `CHANGELOG.md`.
- Số đo trước/sau: `reports/baseline.md` của Phase 01 và mục bổ sung của Phase 03.

## Goal

Toàn bộ đợt nâng cấp được kiểm chứng đầu cuối trên dữ liệu giả lập lớn và trên một bộ
tài khoản phụ thật, tài liệu khớp hành vi mới, và plan này được đóng với kết quả đo
được thay vì cảm tính.

## Tasks

### Task 07.1: Chạy tiêu chí của plan

- **Steps:**
  1. Lần lượt kiểm từng dòng trong mục "Tiêu chí" của `plan.md` và ghi đạt/không đạt kèm bằng chứng (số đo, ảnh chụp, đầu ra test).
  2. Dòng nào không đạt thì mở lại đúng phase sở hữu nó; không sửa plan để hạ tiêu chí cho khớp kết quả.
- **Verify:** Mọi dòng tiêu chí có trạng thái và bằng chứng; không còn dòng chưa kiểm.

### Task 07.2: Bổ sung kiểm thử tay và tài liệu

- **Target files:** `docs/manual-testing.md`, README nếu hành vi người dùng thấy được đã đổi.
- **Steps:**
  1. Thêm kịch bản: mở vượt trần và hàng đợi, cuộn danh sách 500 profile, tạo và xoá folder có profile, tạo profile từ template, một phiên đồng bộ hai profile.
  2. Viết lại các đoạn mô tả tổ chức profile trong tài liệu để nhắc folder cạnh site và tag.
- **Verify:** Từng kịch bản mới chạy qua ít nhất một lần trên bản dựng hiện tại và cho kết quả như văn bản mô tả.

### Task 07.3: CHANGELOG và đóng plan

- **Target files:** `CHANGELOG.md`, `plan.md` của thư mục này.
- **Steps:**
  1. Thêm mục cho đợt nâng cấp: hiệu năng, Folders, Templates, Synchronizer, kèm số đo trước/sau chính.
  2. Cập nhật trạng thái từng phase trong `plan.md`, thêm mục "Kết quả" với số đo cuối và những gì chưa kiểm được (ví dụ máy cấu hình khác).
  3. `make test` và `make lint` lần cuối; `git status --short` rỗng sau commit đóng plan.
- **Verify:** CHANGELOG có mục mới; `plan.md` ở trạng thái completed với mục Kết quả không rỗng.

## Cook note (2026-10-27, cook tự động)

- **Task 07.2 xong phần văn bản:** `docs/manual-testing.md` thêm Phần D (4 kịch bản: trần + hàng đợi, danh sách 500, Folders, Templates) — đã ghi rõ là kịch bản chưa chạy.
- **Task 07.3 xong phần CHANGELOG:** mục `Unreleased` cho đợt 261027 với số đo chính. Chưa cập nhật trạng thái completed trong `plan.md` vì nhiều Verify chưa đạt (đúng quy tắc của Task 07.1/07.3: không hạ tiêu chí cho khớp kết quả).
- **Task 07.1 chưa làm trọn:** bảng kiểm từng dòng tiêu chí nằm ở `reports/cook-auto-2026-10-27.md` với trạng thái đạt/chưa kèm bằng chứng; các dòng chưa đạt trỏ về đúng phase sở hữu.
- Chưa chạy được ở phía Muse: `make test`/`make lint` toàn repo trên macOS và commit đóng plan (không có shell trên Mac; code chưa được user duyệt để commit).

## Cập nhật sau review (2026-10-27)

Bằng chứng hồi quy hiện hành ở `reports/review-fixes-2026-10-27.md`. Các phase còn
thiếu UI/Synchronizer/đo RAM/render vẫn mở; không đánh dấu toàn plan completed chỉ
vì unit test đạt. Không tự commit hoặc đòi worktree sạch khi user chưa yêu cầu.

## Phase exit gate

- Mọi Verify đạt.

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
