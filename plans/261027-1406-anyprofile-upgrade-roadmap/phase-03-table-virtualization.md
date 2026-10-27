---
title: "Phase 03: Query toàn cục, phân trang snapshot và bảng ảo"
status: in_progress
effort: 2d
---

# Phase 03: Query toàn cục, phân trang snapshot và bảng ảo

## Hợp đồng và hiện trạng

Nguồn chính: [profile-list](contracts/profile-list.md),
[performance](contracts/performance.md), [matrix](acceptance-matrix.md).
2d là estimate cũ; snapshot/drag/query parity cần re-estimate trước cook.

Có store LIMIT/OFFSET/count/scoped accounts/extensions, Unicode fold, stable ID
sort và index; test 500 dòng/SQL mean Mac đã có ở [baseline](reports/baseline.md).
Chưa có service snapshot/query parity đầy đủ, frontend paging/debounce/ảo hóa.
Không coi backend paging hiện tại là đủ để thay danh sách UI.

## Task 03.0 — Chốt DTO và reference semantics trước UI

**Files:** `internal/service/profiles_page.go`, `profiles.go`, `internal/store/`,
`frontend/src/lib/health.ts`, `pages/profiles-page.tsx`, API/models/tests.

1. Lập fixtures reference cho query trên toàn dataset: running/attention, folder,
   unfiled, tag, site, text và trash; giữ nghĩa active-view hiện tại.
2. Bổ sung search parity proxy exit IP/tên site built-in/custom, Unicode/#serial/
   literal wildcard. Không lấy slug thay tên hiển thị hay bỏ trường để query nhanh.
3. Sort pinned + no/locale name/lastOpen theo contract, dùng thứ tự user lưu ở
   SettingProfileOrder; kiểm parity en/vi với frontend hiện tại trước khi bỏ local sort.
4. Thiết kế snapshot IDs/revisions/consistent row read, expiry/cache bounds và DTO
   để Phase 04/05 dùng chung. Lọc/sort trước paging; total không khác revision.

**Verify:** ACC-06 backend: reference IDs/order/total khớp từng filter và tổ hợp
AND tại 500 profile; data mutation giữa pages trả stale, không trộn cũ/mới. Đây
là test mới cần viết, không gọi test paging hiện có là full query-contract coverage.

## Task 03.1 — Service snapshot, cancellation và frontend data flow

**Files:** service/store, `frontend/src/api.ts`, `hooks/use-profiles.ts`,
`pages/profiles-page.tsx`, bindings/tests. Giữ API cũ cho caller chưa migrate.

1. Open/read page snapshot; response có total, data/order revision và counters.
2. Cache frontend tối đa 3 page rows, selection chỉ IDs; server snapshots bounded.
   Không cache mọi enriched row để gọi là "đã phân trang".
3. Debounce 200 ms; input paint ngay; request generation bỏ response lỗi thời kể
   cả khi bridge không abort được request transport. Invalidate/refresh có anchor.
4. Chuyển sidebar/counters/filter/sort sang cùng query contract; không lọc trang
   đã tải rồi trình bày total như danh sách đầy đủ.

**Verify:** đổi query nhanh, responses đảo thứ tự, snapshot expired/stale, mutation
runtime/expiry và API error không hiện rows/total sai. PERF-01/02/04 đạt với service
có enrichment; test SQL microbench không thay cho số service + bridge.

## Task 03.2 — Virtual rows, selection và reorder theo ID

**Files:** `profile-table.tsx`, `use-row-drag.ts`, page selection/bulk hooks,
styles/i18n/tests; service reorder/range endpoints và bindings liên quan.

1. Chọn row-height strategy dựa trên đo normal/compact/tag wrap; ghi fixed-height
   truncation/a11y hoặc measured-height trước khi code. Thư viện mới phải có lý do.
2. Render window + overscan, không mất menu, keyboard focus, pin hay drawer.
3. Range/Select all dùng snapshot IDs toàn bộ filtered result, không DOM/page
   indices. Mutation/expiry giữa range -> refresh rõ, không chọn nhầm profile mới.
4. Reorder bằng ID + before/after anchor + expected order revision; không gửi
   toàn rows đang tải làm thứ tự global. Hàng ẩn giữ relative order; không qua nhóm pin.
5. Auto-scroll qua page, arrow reorder, Escape rollback và focus restoration;
   không tắt drag/shift-range chỉ vì virtualized.

**Verify:** ACC-07: range index 5..250 khi cache chỉ 50 rows; select-all/deselect,
bulk frozen IDs, drag qua page và view lọc có hàng ẩn, concurrent reorder/stale.
Viewport bảng 720 px có <60 data rows ở mọi vị trí; viewport khác theo bound
PERF-06. Test native WebView cho layout/focus/scroll, JSDOM chỉ kiểm logic DOM.

## Task 03.3 — Đo end-to-end và regression

Chạy lại đúng fixture/host/build đã ghi ở Phase 01; >=30 samples và raw artifact
cho p95. Ghi số trước/sau cùng môi trường, không so mean Linux với p95 Mac.

**Verify:** PERF-01..06 và ACC-06/07/11 đạt; nếu một metric chưa đạt thì phase
chưa đóng. Đo first paint, input paint, final search paint và frame interval;
không dùng số DOM rows hay screenshot để suy ra latency.

## Gate, dependency và lệnh

- Folder schema hiện có là prerequisite; Phase 04/05 UI cần DTO 03.0 trước khi
  nối sidebar/dialog/bulk. Khi sửa cùng app_bindings/api/models phải phối hợp tuần tự.
- Lệnh hiện có: `go test ./internal/store ./internal/service -count=1`, benchmark
  trong matrix, `cd frontend && npm run typecheck && npm test`; test WebView/script
  performance mới phải lưu vào repo và có lệnh trước khi chạy nghiệm thu.
- ACC-06/07 và PERF-01..06 PASS theo lanes required, `make test`/`make lint` đạt.
  Snapshot query/virtual UI chưa code thì không coi phase này completed.

## Failure protocol

Giữ phase mở, lưu expected/actual/revision/fixture/raw artifact; sửa nguyên nhân
rồi rerun case ảnh hưởng. Không bỏ hidden-row ordering, privacy hay browser test
để giữ estimate 2d. Thay contract/scope cần user duyệt, không giảm tiêu chí theo kết quả.
