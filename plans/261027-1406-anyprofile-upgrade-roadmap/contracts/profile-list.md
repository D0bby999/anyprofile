# Hợp đồng danh sách, phân trang, chọn nhiều và kéo-thả

**Đích cần triển khai.** `ListProfilesPage(f, limit, offset)` hiện tại chỉ là bước
backend đầu tiên; chưa đủ để nối thẳng UI. Không chuyển sang page-local filter/sort.
Tên DTO/API dưới đây là thiết kế đề xuất; giữ API cũ cho caller hiện tại.

## Một truy vấn chuẩn cho toàn bộ UI

`ProfileQuery` chứa `query`, `folderId|unfiled`, `tag`, `siteId`, `view`, `trashed`,
`sortKey`, `sortDesc`, `language`. Chỉ nhận key/enum allowlist, giá trị đi qua
parameters; không ghép SQL theo chuỗi từ client.

- `view = all | running | attention`; giữ nghĩa view running hiện có là active
  (`queued/starting/running/stopping`), không chỉ PID running. Các counter hiển thị
  running thực/queued tách riêng; đổi nhãn view phải qua i18n.
- Folder/unfiled, site, tag, query và view kết hợp bằng AND. Folder + unfiled là
  input invalid, không silently chọn một nhánh. Trash không trộn live profiles.
- Attention dùng cùng predicate đã có ở frontend/health, gồm fingerprint, checks,
  flags, proxy expiry; test parity để không có hai định nghĩa khác nhau. Capture
  runtime map và mốc `now` một lần cho một snapshot; không tính lại từng trang.
- Query bỏ dấu đúng `fold` hiện tại (NFD, bỏ marks, đ -> d, lowercase), tìm substring
  ở name, account.login, notes, tags, **proxy exit IP và tên site hiển thị**. Query
  dạng `#123` chỉ khớp serial. Kiểm literal `%`, `_`, backslash; không coi là wildcard.
  Backend hiện chưa đủ IP/site-name parity: đó là phần cần bổ sung, không ghi đã xong.
- Site-name lookup gồm built-in/custom catalog tại revision snapshot, không suy
  từ slug thay tên. Tìm accounts không đọc password/TOTP/cookie để lọc.

## Sort toàn cục và snapshot

- Pinned luôn đứng trước. Sort `no` mặc định dùng thứ tự kéo-thả đã lưu ở
  `SettingProfileOrder`, ID chưa có trong đó xếp tiếp theo serial rồi ID; không
  thay bằng `sort_order/created_at` SQL nếu nó chưa phản ánh thứ tự user.
- Sort name giữ hành vi localeCompare tiếng Việt/Anh hiện tại: service dùng
  collation tương đương, có fixtures kiểm parity. Sort lastOpen theo timestamp,
  null nhất quán với hành vi hiện tại. Cuối mọi sort có serial/ID tie-breaker.
  Desc đảo khóa sort đã chọn, không đảo quy tắc pinned.
- `OpenProfileList(query)` tạo snapshot opaque và trả `snapshotId`, `dataRevision`,
  `orderRevision`, `total`, `globalLiveCounts` và trang đầu. Lọc + sort **trước** cắt trang.
- `ReadProfilePage(snapshotId, offset, limit)` trả offset/total/revision giống
  snapshot. Page size mặc định 50, max 200; negative/invalid input bị từ chối.
- Snapshot là danh sách ID đã lọc/sort trên DB consistent read cùng runtime map/
  clock captured. Service kiểm revision trước/sau materialization; nếu dữ liệu đổi
  giữa chừng thì retry bounded hoặc trả `list_stale`, không trộn hai thời điểm.
- Snapshot cần dữ liệu row nhất quán: nếu mutation sau đó ảnh hưởng filter/order/
  visible fields/health, đánh dấu stale và yêu cầu mở snapshot mới trước khi trả
  trang tiếp. Không lấy IDs cũ rồi ghép row data mới mà vẫn gọi là snapshot nhất quán.
- UI giữ tối đa 3 trang rows trong cache; selection giữ IDs, không giữ mọi row.
  Snapshots server chỉ giữ IDs + metadata cần thiết, tối đa 2/client, 10/app,
  idle expiry 60 giây và absolute expiry 5 phút. LRU/expiry trả `list_expired`;
  request đang dùng không đọc vùng cache đã giải phóng. Không persist/log query.
- Mỗi thay đổi query/sort/view tăng request generation; response cũ không được
  ghi đè mới dù transport không hỗ trợ abort. Snapshot stale -> refresh, giữ
  selection theo ID còn thuộc kết quả và scroll anchor nếu còn trong kết quả.
  Chỉ mutation ảnh hưởng query/order/visible data mới invalidate; tick đồng hồ
  không liên quan không làm snapshot churn. Boundary đổi expiry/attention đặt
  valid-until tương ứng, không giữ trạng thái sai tới hết TTL.
- Debounce trailing 200 ms; input vẫn phản hồi ngay. Counter total dùng cùng query
  và snapshot, không count trước rồi list sau ở hai revision khác nhau. Sidebar
  `globalLiveCounts` là count toàn bộ live profile theo folder/site/tag ở cùng
  data revision, không chịu query/paging và có nhãn phạm vi rõ; không lấy nó thay
  `total` của bảng. Live counts không gồm Trash.

Snapshot không được coi là bằng chứng user đã đồng ý thao tác sau đó. Mọi mutation
revalidate ID/state/quyền trước khi thực hiện; lỗi trả rõ, không áp vào profile mới
thay thế do index thay đổi. Nếu snapshot refresh quá thường xuyên vì runtime
churn, đo lại PERF-02/04; không né bằng cách bỏ qua mutation liên quan.

## Chọn dòng qua vùng chưa tải

- Selection và anchor theo ID; anchor thuộc snapshot. Đổi query/filter/view/trash
  thì clear selection + anchor; đổi sort giữ các selected IDs còn khớp query nhưng
  reset range anchor. Data refresh giữ giao của selected IDs và kết quả mới; order
  revision đổi thì reset anchor, lần click kế tiếp chốt anchor mới. Dialog bulk
  đang confirm phải refresh/reconfirm khi tập chọn bị thay đổi, không execute lén.
- Checkbox header chọn toàn
  bộ **kết quả đã lọc**, không chỉ DOM/window/page; nhãn ghi đúng số lượng.
- `ResolveSelection(snapshotId, all|range(anchorId,targetId))` trả IDs từ danh
  sách đã sort. Shift-range giữa trang chưa tải không cần fetch full rows.
- Snapshot stale/expired thì refresh và yêu cầu chọn lại range; không suy khoảng
  theo index của một snapshot khác. Profile mới xuất hiện sau snapshot không tự
  lọt vào bulk action đã xác nhận.
- Bulk gửi IDs đã chốt, không gửi "all matching current query" để server tự chọn
  một tập mới. Bulk move folder nguyên tử; bulk open trả per-item request như
  [queue contract](launch-queue.md), không giữ UI busy đến khi queue cạn.

## Kéo-thả và bàn phím trên danh sách ảo

- Cho phép reorder chỉ ở sort no tăng dần, như hiện tại; vẫn hỗ trợ khi lọc.
  Không lặng lẽ tắt drag/shift-select vì virtualization khó.
- `MoveProfile(profileId, beforeId|afterId, expectedOrderRevision, snapshotId)`
  thay thao tác gửi toàn bộ rows đang tải làm thứ tự mới. Neo là ID, không pixel
  index của DOM. Không chấp nhận cả before/after cùng lúc hoặc neo chính nó.
- Di chuyển trong nhóm pinned/unpinned hiện tại; không vượt nhóm hoặc tự đổi pin.
  Trong view lọc, remove ID rồi insert sát trước/sau neo hiển thị trong **thứ tự
  toàn cục**; giữ nguyên thứ tự tương đối mọi ID ẩn/các ID không di chuyển.
- Server kiểm snapshot/order revision và neo còn hợp lệ rồi commit transaction;
  stale -> reject + refresh, không tự đoán vị trí mới. ArrowUp/Down cùng semantics.
- Auto-scroll kéo qua viewport/page, drop ở row chưa render, Escape rollback và
  focus sau reorder phải được test. Keyboard focus neo theo ID, không mất vào
  spacer row; spacer không selectable và có aria-hidden phù hợp.
- Không giả định chiều cao cố định từ code hiện tại: density/tag wrap/nội dung
  dài phải đo. Chọn fixed-height có quy tắc truncation + drawer đầy đủ hoặc
  measured heights; ghi quyết định trước khi viết. Không làm mất thông tin/a11y.

## Test contract

- Fixture 500: Unicode/đ, accounts, custom site tên khác slug, proxy IP, folder,
  tag, trashed, pin, health và timestamp trùng; so IDs + total với reference query
  đầy đủ cho mọi filter/sort rồi với nhiều tổ hợp AND.
- Đọc hết từng trang: đúng tập, đúng thứ tự, không trùng/thiếu. Chèn/xóa/rename,
  pin/reorder/expiry hoặc launch state đổi giữa hai trang -> stale đúng, không
  response cũ ghi đè sau khi đổi query nhanh.
- Range từ index 5 tới 250 khi chỉ có 50 rows cached; select-all/clear; bulk không
  chọn row mới thêm; deselect một ID và filter đổi có quy tắc UI rõ.
- Drag qua biên trang, qua nhóm pin, view lọc có hàng ẩn, keyboard/Escape và
  concurrent reorder; xác nhận hidden IDs không bị đảo ngoài ý định.
- Đo PERF-01..06 trên WebView thật; unit DOM đếm rows không thay số đo frame.
