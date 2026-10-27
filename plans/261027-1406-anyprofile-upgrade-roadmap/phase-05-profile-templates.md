---
title: "Phase 05: Templates hồ sơ theo nền tảng"
status: in_progress
effort: 2d
---

# Phase 05: Templates hồ sơ theo nền tảng

## Context Links

- Site có sẵn: `internal/site/`, `frontend/src/components/site-picker.tsx`, `frontend/src/lib/sites`.
- Mặc định tạo profile: `frontend/src/components/defaults-settings.tsx`, `new-profile-dialog.tsx`.
- Persona và seed: README mục Identity; kiểm tra nhất quán: plan `261014-0001` phase 10.

## Goal

Tạo một profile đúng chuẩn cho một nền tảng trong một lần chọn: template gói sẵn site,
loại proxy khuyên dùng, ngôn ngữ và các mặc định liên quan, để profile mới sinh ra đã
nhất quán thay vì người dùng tự ráp từng mục rồi kiểm tra nhất quán mới phát hiện lệch.

## Hợp đồng liên phase (contract hardening 2026-10-27)

- Dùng folder/query/selection DTO ở Phase 03.0 và folder UI Phase 04, không tự
  cache/filter cả danh sách khác với bảng. Bulk template áp cho IDs/profile mới
  được user xác nhận, proxy là lựa chọn explicit riêng, không chứa credential.
- Validate template trước khi lưu; tạo profile áp defaults trước insert/notify.
  Không tuyên bố launch tự re-derive locale để che mismatch; test thực với proxy
  VN và proxy khác thị trường phải giải thích warning, không sửa fingerprint lén.
- Theo ACC-02/08/11 trong [matrix](acceptance-matrix.md): backup/restore template
  có folder collision/rollback, builtin immutable, create/edit/duplicate/delete,
  tạo bulk 3 profile cùng defaults và UI en/vi sáng/tối/keyboard.
- Template trỏ folder đã bị xóa: UI báo folder không còn và cho chọn lại/unfiled;
  không hiển thị tên folder cũ như còn tồn tại. Không làm hỏng tạo profile cũ.

## Tasks

### Task 05.1: Model template

- **Target files:** `internal/service/`, `internal/store/`, test.
- **Steps:**
  1. Template gồm: tên, site đích, ghi chú loại proxy (chỉ là khuyến nghị hiển thị, không tự mua hay tự gán proxy), ngôn ngữ ưu tiên, folder đích (nếu Phase 04 đã xong), và các mặc định tạo profile đã có.
  2. Template đóng gói sẵn cho TikTok, YouTube, Facebook và một template trống; người dùng sửa và nhân bản được template đóng gói, xoá thì chỉ xoá bản của mình.
  3. Template không chứa secret, không chứa proxy cụ thể và không chứa cookie.
- **Verify:** Test Go: tạo profile từ template đóng gói cho ra đúng site và ngôn ngữ đã khai; xuất template ra JSON không có trường secret nào.

### Task 05.2: Tạo nhanh từ template

- **Target files:** `frontend/src/components/new-profile-dialog.tsx` và test, trang Settings (quản lý template), i18n.
- **Steps:**
  1. Đầu dialog tạo profile có hàng chọn template; chọn xong các mục bên dưới tự điền và vẫn sửa tay được từng mục.
  2. Tạo hàng loạt áp dụng một template cho cả đợt.
  3. Settings có mục quản lý template: tạo, sửa, nhân bản, xoá template của người dùng.
- **Verify:** Test frontend: chọn template TikTok thì site và ngôn ngữ đổi đúng, sửa tay một mục không bị template ghi đè lại; tạo hàng loạt 3 profile từ template cho 3 profile cùng chuẩn.

### Task 05.3: Nối với kiểm tra nhất quán

- **Steps:** Profile tạo từ template, khi mở lần đầu, kết quả kiểm tra nhất quán (đã có) không có mục danger nào do lệch ngôn ngữ hay múi giờ từ chính template gây ra; nếu proxy người dùng gán làm lệch, câu cảnh báo nói rõ lệch ở proxy, không đổ cho template.
- **Verify:** Kiểm thử tay theo `docs/manual-testing.md`: tạo profile từ template TikTok, gán proxy Việt Nam, mở lần đầu và chụp lại kết quả checks vào nhật ký phase.

## Cook note (2026-10-27, cook tự động)

- **Task 05.1 xong (backend):** migration `0019_templates.sql` + `internal/store/templates.go` (CRUD template người dùng); template đóng gói sẵn TikTok/YouTube/Facebook/Trống nằm trong code (`internal/service/templates.go`, cờ `Builtin`), sửa = nhân bản thành bản của mình, xoá template đóng gói bị từ chối. Template không có trường secret/proxy cụ thể/cookie theo thiết kế; `ProxyNote` chỉ là khuyến nghị hiển thị. `CreateProfileFromTemplate` tạo profile đúng site, trang đầu, ngôn ngữ, tag và folder đích (nếu folder còn tồn tại).
- Test: store CRUD đạt trên clone; test service (tạo profile từ template TikTok cho đúng site + vi-VN, JSON không chứa trường secret, template đóng gói chỉ đọc nhưng nhân bản được) đã viết và biên dịch đạt qua vet — chưa chạy được ở máy Muse vì package service phụ thuộc launcher không có bản Linux; cần chạy trên Mac: `go test ./internal/service/ -run Template -v`.
- **Chưa làm:** Task 05.2 (hàng chọn template trong dialog tạo profile, quản lý template ở Settings) — frontend, tạm hoãn; Task 05.3 là kiểm thử tay — kịch bản đã thêm ở `docs/manual-testing.md` mục D4, chưa chạy.

## Cập nhật sau review (2026-10-27)

Đã thêm validation engine, URL http(s), folder, site và language tag; bảo vệ
built-in theo ID thay vì tin cờ do client gửi. Profile từ template áp dụng toàn
bộ defaults trước lần insert/notify duy nhất, tránh UI đọc trạng thái dở dang.
Backup/restore giữ template và remap folder. Test service đã chạy trên Mac, không
còn chỉ biên dịch/vet. UI template và nghiệm thu với proxy thật vẫn chưa làm.

## Phase exit gate

- Mọi Verify và ACC-02/08/11 liên quan PASS theo required lanes; `make test`/`make lint` đạt. Backend test không thay nghiệm thu UI/proxy thật.

## Failure Protocol

Giữ phase mở, ghi case/lane/revision/fixture, command/output đã redacted và
expected/actual; sửa nguyên nhân rồi rerun case ảnh hưởng. Không sửa fingerprint
hoặc đổi proxy sang direct để che mismatch. Chỉ nhờ reviewer/subagent có thật;
thiếu môi trường/quyết định thì báo user, không coi test chưa chạy là PASS.
