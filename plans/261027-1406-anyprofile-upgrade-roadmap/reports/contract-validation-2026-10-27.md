# Validation của plan sau contract hardening — 2026-10-27

Lượt này chỉ chỉnh plan. Không sửa code production, không chạy browser/build/test
nghiệm thu mới, không commit/push và không đổi trạng thái feature thành completed.

## Quyết định đã ghi rõ

1. Dùng PERF-01..09 thay tiêu chí cảm tính; mẫu/timing boundary/host/fixture và
   p50/p95/p99 tách khỏi mean SQL. M2/16 GiB/macOS 26.7.2 đã được kiểm bằng sysctl/
   sw_vers. Windows và RAM-32 là lane cần chạy, không giả là môi trường đã có.
2. Queue async có request generation/position/revision; cancel không đóng lén
   browser đã running; caller disconnect không hủy accepted UI job. Legacy API/MCP
   vẫn chờ ready nhưng không bypass pool. Cap fallback provisional 2 là policy
   thiết kế mới, chưa áp vào production; migration giữ explicit unlimited cũ.
3. Snapshot query/sort/total là toàn cục trước paging; search thêm proxy IP/site
   name parity, order theo SettingProfileOrder. Header chọn toàn filtered result;
   range/reorder dùng ID/revision, giữ hidden-row relative order.
4. Sidebar count giữ toàn live theo folder/site/tag và có field/phạm vi riêng,
   không tráo thành filtered total. Row cache 3 pages, snapshots bounded/expired,
   mutation/stale response có quy tắc thay vì trộn hai thời điểm.
5. Synchronizer production phải qua spike; tổng 3 browser, capture feasibility
   chưa chứng minh từ Call/Subscribe. Explicit tab/origin, sensitive fields mặc
   định chặn; không hứa phát hiện mọi secret user gõ vào text thường. Stop ack chỉ
   sau drain, không gọi timeout là thành công hoặc đóng Conn chung để làm xanh.
6. Matrix ACC-01..12 phân lane required/N/A và evidence statuses; missing engine/
   Windows host giữ BLOCKED/NOT_RUN. Public release/AnyBrowser/deploy ngoài scope.

Đây là thiết kế cụ thể cho đề xuất user đã chấp thuận, không phải bằng chứng đã
triển khai. Nếu spike/baseline chứng minh cần đổi sản phẩm/ngưỡng/support, hỏi user
trước khi thay; không dùng report validation này để bỏ qua thực thi các gate.

## Đối chiếu source hiện hành

- `b6f7b78` là HEAD đã xác nhận; root worktree sạch trước sửa plan.
- CDP có Call/Subscribe; không có bằng chứng native full user-input event stream.
- Profiles UI lọc client-side name/account/notes/proxy IP/site names/tags và sort
  locale/name/lastOpen/manual order; view running hiện là state != stopped.
- `SettingProfileOrder` là JSON IDs; SQL default order chưa tương đương order đó.
- `ListProfilesPage` chỉ page/count, chưa có snapshot/generation/query DTO mới.
- Launcher có StartBehind/Adopt và key max_concurrent_launches; async API/mode mới
  trong tài liệu là future work, không được gọi như endpoint đã có.
- Fury đúng catalog còn thiếu trên máy theo report integration trước; không coi
  cross-vet Windows là Windows native acceptance.

Đây là review scoped trong phiên, không phải 4 reviewer độc lập hay audit runtime.

## Structural và consistency checks

Đã chạy validator cục bộ `/tmp/validate-anyprofile-upgrade-plan.py`:

- 14 tài liệu active, đủ 7 phase; 46 link tương đối hợp lệ.
- Đủ 9 định nghĩa PERF và 12 ACC; không có reference ID chưa định nghĩa.
- Frontmatter/status hợp lệ, không phase nào bị ghi completed sai.
- Không trailing whitespace/BOM/zero-width junk được kiểm; không còn placeholder
  `FE_TEST`, quy tắc bắt buộc spawn subagent không có hay câu spike 2 browser/2 đích.
- Archive/history không được dùng làm execution authority. Đã sweep dependency:
  Phase 01 chặn default/perf claim, 03.0 chốt DTO trước UI 04/05, 06.0 GO chặn
  production sync, 07 chỉ đóng khi matrix required PASS.
- `git status --short` của repo chính vẫn rỗng: không sửa production code.

Đây là PASS về cấu trúc và sweep nội dung trong phiên, **không phải** PASS của
nghiệm thu runtime, không có reviewer độc lập hay test mới được chạy.

## Whole-plan sweep

- Main plan đã bỏ mô tả "store chưa có folder" lỗi thời, thay bằng trạng thái
  backend/UI thực và commit evidence. Estimate 17d vẫn là lịch sử, cần re-estimate.
- Phases 02/03/06/07 viết lại theo hợp đồng; lịch sử trước đó được sao lưu riêng.
  Phases 01/04/05 thêm gate/perf/matrix và sửa failure protocol không phụ thuộc
  subagent không có trong runtime. Task 02.2 luôn là cleanup, không phải RAM warning.
- Không còn tiêu chí active "git status sạch bằng mọi giá", 5 lần cold/warm hay
  2 browser nhưng 2 đích cho Synchronizer. Archive giữ nguyên ghi chú cũ, không
  dùng làm execution authority và không sửa report lịch sử thành kết quả mới.
- Các file hợp đồng là nguồn chính của thresholds/API semantics; phase chỉ dẫn
  task/ownership và Verify, tránh hai bản DTO/SQL trái nhau.

## Điều kiện làm tiếp

Có thể triển khai task độc lập: Phase 01 đo/generator, Phase 02.1 async contract,
Phase 03.0 reference query/DTO và Phase 06.0 feasibility spike. Chưa được kết luận
full plan ready-to-close hoặc tự chạy cook toàn bộ: default/performance cần dữ
liệu, sync production đợi GO và acceptance cần các lane hiện chưa có bằng chứng.

Tại thời điểm validation, plan còn bị ignore và chưa commit. Sau đó user yêu cầu
`cm` và xác nhận plan đã chuyển vào `plans/` của repo chính: `.gitignore` được mở
riêng cho roadmap này để theo dõi bình thường, không dùng force-add; các plan
khác/journals vẫn local, `pro` không nằm trong commit. Không push ngoài yêu cầu.
Không có thay đổi giao diện production nên không cần mở browser để kiểm tài liệu.
