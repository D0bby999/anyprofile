---
title: "Phase 07: Nghiệm thu theo ma trận và tài liệu đúng hành vi"
status: in_progress
effort: 1d
---

# Phase 07: Nghiệm thu theo ma trận và tài liệu đúng hành vi

## Nguồn chuẩn và trạng thái

Dùng [acceptance matrix](acceptance-matrix.md), các
[performance](contracts/performance.md), [queue](contracts/launch-queue.md),
[list](contracts/profile-list.md), [sync](contracts/synchronizer.md) contracts.
[Review fixes](reports/review-fixes-2026-10-27.md) và [baseline](reports/baseline.md)
là bằng chứng cũ có phạm vi; chưa chứng minh các gate hợp đồng mới PASS.

Task này không được bỏ feature hoặc giảm support matrix để đổi pending thành
completed; `1d` chỉ là estimate nghiệm thu gốc khi các dependency đã sẵn sàng.

## Task 07.1 — Evidence ledger và kiểm từng required cell

**Output khi chạy:** `reports/acceptance-results.md` và raw artifacts có hash.
File kết quả chưa tồn tại không phải bằng chứng test đã chạy.

1. Lập record cho ACC-01..12 theo từng lane R: build/engine/fixture/revision,
   command hoặc steps, số mẫu, expected/actual và artifact truy cập được.
2. Gán PASS/FAIL/BLOCKED/NOT_RUN/N/A đúng định nghĩa; lỗi engine hash/M-F hoặc
   thiếu Windows/RAM-32 giữ BLOCKED/NOT_RUN, không ghi PASS từ cross-compile.
3. Kiểm PERF-01..09 với cách đo/sampleCount đã chốt; mean SQL không được gọi p95
   service/UI. Nếu môi trường tranh tải invalid thì giữ raw log/lý do trước rerun.
4. Với case fail, mở lại phase sở hữu và giao remediation; rerun case ảnh hưởng
   trên revision mới. Scope/product change chỉ sau user duyệt.

**Verify:** mỗi required cell có evidence PASS đúng revision; không còn NOT_RUN/
BLOCKED/FAIL trước khi đóng phase. Người khác chạy lại được bằng fixture/script.

## Task 07.2 — Manual docs và compatibility

**Files:** `docs/manual-testing.md`, README theo tính năng đã thực sự có.

1. Ghi cách async enqueue/cancel/position, Settings modes và warning provisional/
   unlimited; không hướng dẫn nút/UI chưa ship như đã có.
2. Ghi kịch bản 500 rows, query parity, snapshot stale/refresh, range/select-all,
   drag qua page/view lọc và preservation của hidden rows.
3. Ghi Folders/Templates/bulk và migration schema 17, backup mã hóa/restore vào
   destination có dữ liệu; rollback/vault/Trash không mất profile.
4. Ghi Synchronizer tổng 3 browser (1 chính + 2 đích), consent/tab/origin/input
   unsupported, privacy warning và Stop healthy/unresponsive đúng contract.
5. Test giao diện bằng text/attributes trước screenshot element khi cần; en/vi,
   sáng/tối, compact/normal, keyboard và native WebView theo matrix.

**Verify:** từng hướng dẫn chạy ít nhất một lần trên build đúng lane; documents
khớp actual. Không coi frontend unit tests là kiểm layout native đã đạt.

## Task 07.3 — Regression, changelog và đóng plan có điều kiện

1. Chạy các lệnh hiện có trong matrix; viết tests/harness mới ở phase sở hữu và
   ghi command thực, không dùng tên test tương lai như bằng chứng.
2. Ghi CHANGELOG chính xác tính năng đã ship, số đo cùng host/fixture và giới hạn;
   không ghi Synchronizer đã có khi spike/production chưa hoàn tất.
3. So field dữ liệu cũ sau migration/backup/rollback; verify support Mac/Windows.
4. Reconcile `plan.md`, phase statuses, results và tất cả contract links. Chỉ
   completed khi mọi gate required PASS. Commit/push cần user yêu cầu riêng;
   không reset hay stage thay đổi ngoài phạm vi để làm worktree sạch.

**Verify:** build/test/lint/typecheck/race và acceptance matrix PASS; report ghi
commit/hash/limitations. Nếu lane required thiếu evidence thì phase/plan vẫn mở.

## Tách khỏi public release

Nghiệm thu roadmap không thay gate ký/notarize Windows/macOS, cập nhật catalog,
AnyBrowser migration/rename, tải public, Pro entitlement/runtime delivery hay
acceptance tài khoản thật dài hạn. Chỉ ghi các dependency đó, không tự deploy/
publish hoặc tuyên bố production-ready từ test feature này.

## Failure protocol

Giữ tiêu chí, ghi sự thật và blocker owner/next action; không chạy lại bộ đã biết
fail môi trường khi chưa sửa nguyên nhân. Dừng task bị chặn và báo user nếu cần
máy/engine/quyết định ngoài quyền hiện có. Có thể tiếp tục task độc lập theo graph,
nhưng tuyệt đối không dùng kết quả đó đóng required cell đang bị chặn.
