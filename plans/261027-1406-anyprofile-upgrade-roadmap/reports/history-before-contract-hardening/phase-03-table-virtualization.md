---
title: "Phase 03: Ảo hoá bảng profile và phân trang danh sách"
status: in_progress
effort: 2d
---

# Phase 03: Ảo hoá bảng profile và phân trang danh sách

## Context Links

- Ngân sách render và mốc ảo hoá: `reports/baseline.md` của Phase 01.
- Bảng hiện tại: `frontend/src/components/profile-table.tsx` và test của nó; kéo thả dòng: `frontend/src/components/use-row-drag.ts`; chọn nhiều và shift-range đã có.
- Liệt kê profile: `internal/service/`, `frontend/src/api.ts`.

## Goal

Danh sách 500 profile cuộn và tìm kiếm mượt như danh sách 20: chỉ render các dòng đang
thấy, tìm kiếm có debounce và không ép frontend giữ toàn bộ danh sách khi service đã có
thể phân trang.

## Tasks

### Task 03.1: Ảo hoá dòng trong bảng

- **Target files:** `frontend/src/components/profile-table.tsx`, test của nó, `frontend/src/styles/shell.css` nếu cần.
- **Steps:**
  1. Chỉ render cửa sổ dòng đang thấy cộng vùng đệm; chiều cao dòng cố định như hiện tại để tính cửa sổ không cần đo DOM.
  2. Giữ nguyên: chọn nhiều bằng checkbox và shift-range (chọn theo index trong danh sách đã lọc, không theo dòng đang render), sort bằng bàn phím, menu dòng, kéo thả sắp xếp.
  3. Không thêm thư viện ảo hoá nếu tự cài đặt cửa sổ dòng đủ đơn giản; nếu cần thư viện, ghi lý do vào plan trước khi thêm dependency.
- **Verify:** Test frontend với 500 dòng giả lập: số phần tử dòng trong DOM dưới 60 tại mọi vị trí cuộn; chọn shift-range qua vùng chưa render vẫn chọn đúng khoảng; `FE_TEST` đạt.

### Task 03.2: Phân trang và tìm kiếm phía service

- **Target files:** `internal/service/`, `internal/store/`, `frontend/src/api.ts`, `frontend/src/pages/profiles-page.tsx`.
- **Steps:**
  1. API danh sách nhận `limit/offset` và từ khoá tìm kiếm; tìm kiếm không phân biệt hoa thường và bỏ dấu, khớp hành vi tìm kiếm hiện tại ở frontend.
  2. Thêm index cho các cột lọc/sort thường dùng nếu Phase 01 cho thấy đang quét toàn bảng.
  3. Frontend debounce 200 ms cho ô tìm kiếm; huỷ request cũ khi gõ tiếp.
- **Verify:** Test Go: tìm kiếm trả đúng tập con ở 500 profile, phân trang không trùng và không sót dòng; thời gian truy vấn đo lại tốt hơn baseline Phase 01.

### Task 03.3: Đo lại so với baseline

- **Steps:** Chạy lại đúng kịch bản đo bảng của Phase 01 ở 500 profile và ghi số mới cạnh số cũ trong `reports/baseline.md` (thêm mục "Sau Phase 03").
- **Verify:** Độ trễ gõ tìm kiếm và thời gian render lần đầu đều dưới ngân sách đã chốt ở Phase 01; nếu một số không đạt, ghi rõ số và lý do thay vì bỏ qua.

## Cook note (2026-10-27, cook tự động)

- **Task 03.2 phần backend xong:** `ProfileFilter` có `Limit/Offset`; `ListProfiles` dùng chung bộ lọc với `CountProfiles` mới; accounts/extensions chỉ nạp cho đúng các dòng trả về (`internal/store/profile_scoped.go`, chia cụm 400 id); service có `ListProfilesPage` + binding. Test store mới (`list_page_test.go`): phân trang không trùng/không sót ở 25 profile, tìm kiếm trả đúng tập con và count khớp — đạt cùng toàn bộ test cũ của store.
- **Task 03.3 phần backend xong:** số đo lại ghi ở `reports/baseline.md` mục 3.3 — trang 50 dòng ~1,4 ms và 222 KB không đổi theo quy mô 100→500.
- **Chưa làm (ghi lại theo yêu cầu bỏ qua phần khó):** Task 03.1 ảo hoá dòng ở frontend và debounce ô tìm kiếm (phần frontend của 03.2). Lý do tạm hoãn: ảo hoá va chạm với kéo-thả sắp xếp dòng và chọn shift-range — cần kiểm chứng trực quan trên app Mac, không nên viết mù. Backend phân trang đã sẵn để frontend dùng.
- Kiểm chứng: `go test ./internal/store/ -count=1` đạt trên clone; `GOOS=windows go build ./...` và `go vet` đạt. Chưa chạy trên Mac.

## Cập nhật sau review (2026-10-27)

Backend đã thêm tìm kiếm Unicode bỏ dấu (kể cả đ), thứ tự phân trang có ID làm
tie-breaker, index phù hợp và test 500 profile cùng timestamp. Benchmark trong
repo đã chạy trên Mac (`reports/baseline.md` mục 5). Chưa nối frontend, chưa đo
render và chưa ảo hóa nên không tuyên bố Task 03.1/03.3 hoàn tất.

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
