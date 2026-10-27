# Sửa lỗi sau review — 2026-10-27

Phạm vi: các lỗi đã xác nhận trong phần backend đang cook, test hồi quy, đường hủy
hàng đợi trên UI và bằng chứng/tài liệu còn thiếu. Không coi đây là việc hoàn thành
mọi tính năng của roadmap. Không commit/push; không sửa dữ liệu profile thật.

## Đã sửa

| Vấn đề | Sửa ở đâu | Bằng chứng |
|---|---|---|
| Restore profile có folder lỗi khóa ngoại | `internal/store/merge.go`, `merge_organization.go` | Merge folder trước profile/template; remap folder trùng tên Unicode; test rollback, import lặp và backup mã hóa đầu cuối |
| Template người dùng bị bỏ lại khi restore | `merge_organization.go`, `internal/service/backup.go` | Template được merge và remap folder; phát notification settings sau restore |
| Stop báo thành công nhưng yêu cầu vẫn chờ | `internal/launcher/launcher.go`, `stop.go` | Đăng ký proc ở trạng thái queued trước khi chờ; Stop hủy và đợi cleanup, chặn yêu cầu mở trùng |
| Hàng đợi không FIFO, context đã hủy vẫn nhận slot | `internal/launcher/gate.go` | FIFO theo thứ tự đăng ký ở gate, loại waiter đã hủy; test head bị hủy, tăng trần, slot adopt |
| Adopt không tính vào trần | `adopt.go`, `gate.go` | Browser adopt nhận một slot và nhả đúng một lần; integration Chromium thật xác nhận trần có hiệu lực |
| StopAll nhả slot rồi vô tình mở yêu cầu chờ | `stop.go`, `start.go` | Hủy queued/starting trước khi đóng browser; ngăn Start mới trong StopAll; các StopAll đồng thời cùng đợi hoàn tất |
| Hủy startup để lại cây tiến trình/pipe | `start.go`, `launcher.go` | Startup theo context, cleanup process group/pipe/record/session khi lỗi; test macOS dùng shell + child, không browser thật |
| Không có thao tác hủy chờ trên bảng | `profile-table.tsx`, `profile-drawer.tsx`, i18n en/vi | Hiện Queued/Đang chờ và nút hủy; test nút gọi Stop, không gọi Start/double-open |
| Tìm kiếm backend không bỏ dấu | `internal/store/search.go`, `profiles.go` | Hàm SQL deterministic cùng quy tắc NFD/đ/lowercase với frontend; test name/notes/tags/account và literal wildcard |
| Phân trang không có tie-breaker/index | `profiles.go`, migration `0020` | ID làm khóa cuối; test 500 profile cùng timestamp; EXPLAIN dùng `profiles_list_order`, không TEMP B-TREE |
| Reorder một phần danh sách sai | `internal/store/folders.go` | Gán lại toàn bộ thứ tự, phần chưa liệt kê nằm sau; từ chối ID lạ/trùng và rollback |
| Tạo folder trùng tên khi ghi đồng thời | `folders.go` | Check tên + insert/rename trong cùng transaction; test 16 create đồng thời chỉ 1 thành công |
| Bulk move lỗi giữa chừng, UI không được thông báo | `folders.go`, `internal/service/folders.go` | Transaction toàn batch, deduplicate, lỗi thì rollback và trả 0; notification chỉ sau commit thành công |
| Template chấp nhận default không dùng được | `internal/service/templates.go` | Validate engine, site, URL http(s), folder và BCP 47; built-in được bảo vệ theo ID, không tin cờ client |
| Profile từ template notify trước khi ghi xong | `templates.go`, `profiles.go` | Áp dụng defaults trước lần insert/notify duy nhất; test observer đọc đúng folder/ngôn ngữ |
| Không có harness benchmark trong repo | `internal/store/profiles_bench_test.go` | Chạy trên Mac; lệnh và số liệu ở `baseline.md` mục 5 |
| Trạng thái plan/Task 02.2 bị ghi sai | `plan.md`, các phase và manual-testing | Phase còn thiếu giữ in_progress; sửa rõ Task 02.2 là dọn tiến trình, không phải cảnh báo RAM |

## Kiểm chứng trên Mac

- `GOFLAGS=-p=2 make test`: PASS, toàn bộ Go packages; frontend 46 files / 212 tests;
  bao gồm TypeScript typecheck.
- `make lint`: PASS (gofmt/SPDX, Go vet macOS + Windows cả integration tags,
  govulncheck, npm production audit, Rust clippy). Không phát hiện lỗ hổng ảnh
  hưởng code theo govulncheck.
- `go test -race ./internal/launcher ./internal/store ./internal/service -count=1`:
  PASS.
- `make build`: PASS; `codesign --verify --deep --strict` PASS.
- Chromium thật 155.0.8059.31, headless, profile tạm + mock keychain:
  `TestApostateLifecycle`, `TestApostateAdoptedBrowserCountsTowardCeiling`,
  `TestApostateLaunchCeilingQueue`: mỗi test chạy 3 lần, PASS (9 lượt tổng).
  Có kiểm fingerprint/cookie/pipe, trần 2/mở 4, cancel chờ, adopt và không còn
  cây tiến trình theo thư mục profile test sau StopAll.
- `TestCancelStartupKillsProcessTree` và unit gate/queue chạy 3 lần: PASS.
- Benchmark ở Apple M2, Go 1.26.10: trang 50 dòng ~0,438 ms tại 500 profile;
  trang kèm count ~0,469 ms; tìm kiếm ~4,012 ms. Đây chỉ là backend SQL, chưa
  phải service/render end-to-end.

### Lỗi môi trường đã gặp — không giấu bằng cách hạ kiểm tra

- Lượt `make test` chạy đồng thời với race/vet/browser đã làm `fury-config`
  chạm timeout 5 giây (`signal: killed`). Sau khi dừng các batch nặng đồng thời,
  test `TestDeriveIsDeterministic` chạy riêng PASS trong ~0,01 giây. Chạy toàn
  suite với `GOFLAGS=-p=2` PASS; không sửa/giãn timeout production hoặc bỏ test.
- Integration Fury `TestLifecycleFiveProfiles` chưa chạy thành công: engine
  code yêu cầu Fury 0.4.5, máy chỉ có 0.3.5. Integrity check từ chối đúng với
  `engine_missing`; không bỏ kiểm hash, không tải/đổi engine của user. Test
  integration được mở rộng để kiểm Adopt nhưng cần bản Fury đúng trước khi
  chạy lại. Đường Adopt đã được kiểm bằng Chromium thật như trên.

## Còn thiếu — không đánh dấu completed

- Baseline browser: mở nguội/nóng, RAM sau 60 giây; baseline render 500 dòng.
- Trần mặc định có căn cứ RAM; UI setting, vị trí hàng đợi/status bar/RAM estimate;
  API enqueue trả ngay (binding Start hiện đợi mở/hủy).
- Frontend phân trang/debounce/ảo hóa, tương thích drag + shift-range và sort.
- Sidebar/dialog/bulk UI Folders; UI quản lý/chọn Templates; kiểm với proxy thật.
- Toàn bộ Synchronizer và nghiệm thu cuối cùng.
- Kiểm giao diện app bằng trình duyệt/WebView thật chưa làm. Headless Chromium
  ở trên kiểm engine/launcher, không thay thế kiểm UI AnyProfile.

## Tự kiểm tra giao diện

1. Mở app trên bộ dữ liệu test riêng, đặt trần 2 bằng binding, mở 4 profile: tối
   đa 2 chạy, các dòng còn lại hiện Đang chờ/Queued.
2. Bấm Hủy ở bảng hoặc drawer của một dòng chờ, rồi đóng browser đang chạy:
   profile đã hủy không tự mở; chỉ yêu cầu FIFO còn lại được mở.
3. Kiểm tiếng Việt/Anh và theme sáng/tối: nhãn và nút không tràn/chồng lấp.

Log chi tiết nằm ở `/tmp/anyprofile-roadmap-*` của phiên này, không phải artifact
lâu dài để thay thế các lệnh/test trong repo.
