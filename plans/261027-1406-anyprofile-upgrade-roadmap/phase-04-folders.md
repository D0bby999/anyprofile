---
title: "Phase 04: Folders"
status: in_progress
effort: 4d
---

# Phase 04: Folders

## Context Links

- Spec gốc đã soạn 2026-10-16 (P1, khoảng trống tổ chức lớn nhất) — plan này kế thừa các quyết định trong spec đó.
- Sidebar và kiểu View hiện tại: `frontend/src/components/sidebar.tsx` (View: `all | running | attention | site: | tag:`).
- Lưu trữ: `internal/store/`, `internal/service/`; thùng rác: `frontend/src/pages/trash-page.tsx`, `internal/snapshot/` không liên quan.

## Goal

Mỗi profile thuộc về một nơi rõ ràng: người dùng tạo folder theo kênh, khách hàng hoặc
mục đích, lọc và chuyển profile hàng loạt theo folder, và xoá folder không bao giờ làm
mất profile.

## Quyết định đã chốt trong spec

- Một profile thuộc tối đa một folder; chưa xếp folder là "Chưa phân loại".
- Folder phẳng ở MVP, không folder con.
- Tag giữ nguyên vai trò nhãn ngang; nhóm theo site giữ nguyên.
- Profile trong Trash giữ `folder_id`; khôi phục về đúng folder cũ.

## Hợp đồng liên phase (contract hardening 2026-10-27)

- UI filter/count/selection/bulk phải theo [profile-list](contracts/profile-list.md),
  không lọc một trang rồi trình bày như toàn bộ folder. Chốt DTO Phase 03.0 trước
  khi nối sidebar/dialog; bulk gửi tập ID đã xác nhận, không chọn lại theo query mới.
- Nghiệm thu ACC-01/02/08/11 trong [matrix](acceptance-matrix.md): migration từ
  schema 17 có dữ liệu, backup mã hóa có template/folder, name/ID collision,
  rollback/file/vault/Trash và UI trên lane OS required.
- Sidebar giữ count **toàn bộ live profile/folder**, đặt field `globalLiveCounts`
  và nhãn phạm vi rõ; `total` của bảng là tập đã lọc theo snapshot query. Không
  lấy số rows cache làm count, không trộn hai revision hoặc gọi global count là
  số kết quả tìm kiếm.
- Test Go đạt không đóng thay gate UI en/vi sáng/tối/keyboard/native WebView.

## Tasks

### Task 04.1: Model và migration

- **Target files:** `internal/store/` (schema và migration), `internal/service/`, test.
- **Steps:**
  1. Bảng `folders`: `id` ổn định, `name` bắt buộc và không trùng sau khi trim (không phân biệt hoa thường), `color` tuỳ chọn từ bảng màu cố định, `sort_order`, `created_at`.
  2. Profile thêm `folder_id` nullable; migration đặt toàn bộ profile cũ thành chưa phân loại, không đụng tag hay dữ liệu khác.
  3. API: tạo, đổi tên, đổi màu, sắp xếp, xoá folder; gán và gỡ folder cho một profile.
- **Verify:** Test Go: migration trên database mẫu có profile cũ cho `folder_id` rỗng toàn bộ; tạo trùng tên bị từ chối; `go test ./internal/store/ ./internal/service/ -count=1` in `ok`.

### Task 04.2: Sidebar và lọc theo folder

- **Target files:** `frontend/src/components/sidebar.tsx` và test, `frontend/src/App.tsx`, i18n.
- **Steps:**
  1. View thêm dạng `folder:<id>` và mục "Chưa phân loại"; sidebar thêm section Folders: Tất cả, từng folder kèm số profile, Chưa phân loại.
  2. Lọc theo folder kết hợp được với tìm kiếm, tag và sort hiện tại; khi đang lọc phải thấy rõ đang ở folder nào và bỏ lọc được bằng một bấm.
  3. Menu trên folder: đổi tên, đổi màu, xoá (flow ở Task 04.4).
- **Verify:** Test frontend: chọn folder chỉ hiện đúng tập đã lọc; `total` khớp snapshot, sidebar global live count khớp toàn folder và không giả thay đổi theo page/query; bỏ lọc trở về đủ danh sách.

### Task 04.3: Gán folder khi tạo và chuyển hàng loạt

- **Target files:** `frontend/src/components/new-profile-dialog.tsx`, `bulk-dialogs.tsx`, menu dòng trong `profile-table.tsx`, API bindings.
- **Steps:**
  1. Dialog tạo/sửa profile có dropdown Folder, mặc định là folder đang xem (hoặc Chưa phân loại nếu đang ở Tất cả), kèm tạo folder nhanh ngay trong dialog.
  2. Thanh bulk thêm "Chuyển vào folder" cho các profile đã chọn; menu từng dòng có "Chuyển vào folder" và "Gỡ khỏi folder".
  3. Tạo hàng loạt chọn một folder đích cho cả đợt.
- **Verify:** Test frontend: tạo profile trong lúc đang xem folder X thì profile mang folder X; chuyển 5 profile bằng bulk thì cả 5 đổi folder và có toast xác nhận số lượng.

### Task 04.4: Xoá folder an toàn và Trash

- **Target files:** `internal/service/`, `frontend/src/components/confirm.tsx` (dùng lại), trang Trash.
- **Steps:**
  1. Xoá folder trống: xác nhận một lần. Xoá folder có profile: bắt buộc chọn chuyển sang folder khác hoặc về Chưa phân loại trước khi nút xoá bật.
  2. Xoá và khôi phục profile giữ nguyên `folder_id`; Empty Trash không đụng folder.
- **Verify:** Test: xoá folder có 3 profile chọn chuyển sang folder B thì cả 3 sang B, folder cũ biến mất, không mất profile; khôi phục từ Trash về đúng folder cũ.

## Cook note (2026-10-27, cook tự động)

- **Task 04.1 xong (backend):** migration `0018_folders.sql` (bảng `folders`, cột `profiles.folder_id`, index theo folder); `internal/store/folders.go` đủ CRUD + xoá folder chuyển profile trong một transaction; `Profile.FolderID` đi qua `profileCols`/scan/INSERT; lọc danh sách theo `FolderID`/`Unfiled`; service `internal/service/folders.go` (bảng màu cố định, `FolderRow` kèm số đếm, chuyển hàng loạt) và bindings trong `app_bindings.go`.
- Quyết định kỹ thuật đáng chú ý: trùng tên folder kiểm tra ở Go bằng `strings.EqualFold` sau khi trim — bản nháp đầu dùng `lower()` của SQLite và test bắt được lỗi: `lower()` chỉ gập ASCII nên "KHÁCH A"/"khách a" lọt qua. Đã sửa và có test phủ đúng ca tiếng Việt này.
- **Task 04.4 phần backend xong:** xoá folder có profile bắt buộc có folder đích hoặc gỡ về Chưa phân loại (tham số `moveTo`); Trash giữ `folder_id` — test gồm cả ca profile đang ở Trash khi xoá folder, khôi phục về đúng folder đích.
- Test store mới (`folders_test.go`, 4 test): migration để profile cũ chưa phân loại, trùng tên bị từ chối, xoá folder chuyển đúng 3 profile, lọc + đếm đúng. Toàn bộ test store đạt trên clone; build Windows + vet đạt.
- **Chưa làm:** Task 04.2 (sidebar + lọc trên UI) và Task 04.3 (dropdown folder trong dialog tạo/sửa, bulk bar, menu dòng) — phần frontend, tạm hoãn theo yêu cầu bỏ qua phần khó; backend và binding đã sẵn.

## Cập nhật sau review (2026-10-27)

Đã sửa restore backup có folder (merge folder trước profile, remap ID khi trùng
tên Unicode), sắp xếp một phần danh sách, uniqueness trong transaction và chuyển
bulk nguyên tử có deduplicate. Migration được test từ schema 17 có profile cũ
thật, không chỉ tạo profile sau migration. Backup mã hóa đầu cuối có folder và
template đã có test. Sidebar/dialog/bulk UI vẫn chưa làm.

## Phase exit gate

- Mọi Verify, ACC-01/02/08/11 liên quan PASS theo required lanes; `make test` và `make lint` đạt. Không bỏ migration/backup/UI vì backend CRUD đã có.

## Failure Protocol

Giữ phase mở, ghi case/lane/revision/fixture, command/output đã redacted và
expected/actual; giao owner xử lý rồi rerun case ảnh hưởng. Không reset dữ liệu,
bỏ migration/backup hoặc hạ tiêu chí để làm xanh. Reviewer độc lập chỉ dùng khi
runtime có thật; thiếu môi trường/quyết định thì báo user, không đóng gate bị chặn.
