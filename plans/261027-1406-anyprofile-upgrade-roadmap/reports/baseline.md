# Baseline hiệu năng — Phase 01

Trạng thái: **phần tĩnh xong; phần truy vấn danh sách đã đo thật** (2026-10-27, trên
clone GitHub của repo chạy ở máy Linux của Muse — môi trường ghi ở mục 3.2). **Chưa
đo:** thời gian mở profile và RAM trên macOS (máy Linux không chạy được engine macOS)
và render frontend trên Mac. Ô chưa đo ghi rõ "chưa đo", không suy đoán thành số.

## 1. Phát hiện tĩnh (đã kiểm chứng từ code)

Đọc trực tiếp: `internal/store/profiles.go`, `internal/store/store.go`,
`internal/store/migrations/0001_init.sql`, `0004_serial.sql`, `0013_accounts.sql`,
`tools/bench/README.md`, `tools/seed-demo/main.go`.

### Danh sách profile (`Store.ListProfiles`)

- Truy vấn lấy toàn bộ cột của mọi profile khớp bộ lọc, `ORDER BY pinned DESC,
  sort_order, created_at`. Không có `LIMIT` — service luôn trả toàn bộ danh sách.
- Index hiện có trên `profiles`: khoá chính `id`, `profiles_deleted (deleted_at)`,
  `profiles_serial` duy nhất `(serial)`. Không có index phục vụ thứ tự sort của danh
  sách — đã xác nhận bằng EXPLAIN ở mục 3.2 (SQLite dùng TEMP B-TREE để sort).
- Tìm kiếm theo chữ dùng `LIKE '%…%'` trên `name`, `notes`, `tags` và `EXISTS` trên
  `accounts.login`.
- Sau khi đọc profile, hàm này gọi tiếp `accountsByProfile("")` và
  `extensionsByProfile("")` — nạp **toàn bộ** accounts và extensions của mọi profile
  rồi nối trong bộ nhớ, bất kể bộ lọc chọn bao nhiêu profile. EXPLAIN xác nhận bước
  này là `SCAN accounts` (quét toàn bộ).
- SQLite đã bật WAL (`journal_mode(WAL)`, `busy_timeout(5000)`, một kết nối ghi) trong
  `store.go` — phần này không cần sửa.

Hệ quả cho plan: Phase 03 có ít nhất ba việc cụ thể, theo thứ tự nghi ngờ tác động:
(1) chỉ nạp accounts/extensions cho các profile trong kết quả, (2) phân trang bằng
`LIMIT/OFFSET`, (3) cân nhắc index cho thứ tự sort.

### Công cụ đo đã có (không phải viết mới)

- `tools/bench/chromium-bench.mjs`: mở N profile cùng lúc, báo RAM theo loại tiến trình
  — đủ cho Task 01.1 phần RAM, chạy trên Mac. Các script bench mở browser bằng cờ bị
  cấm trong app (`--use-mock-keychain`, `--remote-allow-origins=*`), chỉ dùng với
  profile rác.
- `tools/bench/launch-one/`: mở một profile qua launcher thật của app, in cổng chẩn
  đoán — dùng để đo thời gian mở trên Mac.
- `tools/seed-demo`: nạp khoảng 60 profile, 40 proxy, tag, ghi chú, lịch sử mở vào một
  thư mục dữ liệu riêng, không đụng dữ liệu thật, không ghi Keychain. Số lượng cố định
  khoảng 60 — muốn dữ liệu demo lớn hơn cần thêm cờ số lượng (việc nhỏ, làm khi cần).

## 2. Cách đo phần còn lại (trên Mac)

```sh
# Thời gian mở (Task 01.1): launch-one cho 1 profile Apostate và 1 profile Fury,
# mỗi engine 5 lần mở nguội và 5 lần mở nóng, lấy trung vị.
# RAM (Task 01.1): chromium-bench.mjs với N = 1, 3, 5, 10 (xem tools/bench/README.md).
# Render bảng (Task 01.2 phần frontend): mở app với thư mục demo, ghi thời gian
# render lần đầu và độ trễ gõ tìm kiếm ở 500 profile.
```

## 3. Số đo

### 3.1 Thời gian mở và RAM trên macOS (Task 01.1) — chưa đo

| Engine | Mở nguội (trung vị) | Mở nóng (trung vị) | RAM/profile sau 60s |
|---|---|---|---|
| Apostate | chưa đo | chưa đo | chưa đo |
| Fury | chưa đo | chưa đo | chưa đo |

### 3.2 Danh sách và truy vấn (Task 01.2) — đã đo phần backend

Môi trường đo: clone `github.com/D0bby999/anyprofile` (master tại 2026-10-27) trên máy
Linux của Muse — AMD EPYC 9D25 (2 vCPU được cấp), Go 1.26.10, benchmark Go
(`-benchtime 20x`), mỗi profile có đúng 1 account. Harness đo là test tạm trong
`internal/store` trên clone, không nằm trong repo. Test sẵn có của `internal/store`
đạt (`ok`) trên cùng clone. Số tuyệt đối trên Mac sẽ khác; xu hướng theo quy mô và
EXPLAIN là phần dùng để quyết định.

| Mốc | ListProfiles (tất cả) | Tìm kiếm "kenh" | Lọc tag |
|---|---|---|---|
| 100 | 1,71 ms — 414 KB, 7.228 allocs | 1,77 ms | 1,74 ms |
| 300 | 5,01 ms — 1,38 MB, 21.679 allocs | 5,25 ms | 5,24 ms |
| 500 | 8,49 ms — 1,96 MB, 36.281 allocs | 9,19 ms | 8,87 ms |

- Tăng tuyến tính, khoảng 17 µs và 72 allocs cho mỗi profile thêm vào. Ở 500 profile
  mỗi lần liệt kê tốn khoảng 9 ms và cấp phát khoảng 2 MB — gọi một lần thì nhẹ, nhưng
  app gọi lại sau mỗi thay đổi dữ liệu, và frontend còn phải render toàn bộ kết quả
  (chưa đo phần render).
- `EXPLAIN QUERY PLAN` (300 profile):
  - Danh sách: `SEARCH profiles USING INDEX profiles_deleted (deleted_at=?)` +
    `USE TEMP B-TREE FOR ORDER BY` — lọc được bằng index, sort phải tự sắp xếp.
  - Tìm kiếm: như trên, cộng `CORRELATED SCALAR SUBQUERY` chạy EXISTS trên accounts
    cho từng dòng (dùng `sqlite_autoindex_accounts_1` theo `profile_id`).
  - Nạp accounts: `SCAN accounts` — quét toàn bộ, đúng như đọc code dự đoán.

## 3.3 Sau Phase 03 — backend (đo lại 2026-10-27, cùng máy đo của mục 3.2)

Phase 03 đã sửa backend: danh sách chỉ nạp accounts/extensions của đúng các dòng trả về (trước đây nạp toàn bộ), và có phân trang `LIMIT/OFFSET` kèm `CountProfiles`. Đo lại bằng cùng harness (số tuyệt đối giữa hai lượt chạy trên máy ảo có dao động theo tải máy, nên so theo xu hướng):

| Cách gọi | 100 profile | 300 profile | 500 profile |
|---|---|---|---|
| Toàn bộ danh sách | 2,39 ms — 440 KB | 6,18 ms — 1,46 MB | 11,42 ms — 2,09 MB |
| Một trang 50 dòng | 1,49 ms — 222 KB, 3.801 allocs | 1,66 ms — 222 KB, 3.801 allocs | 1,40 ms — 222 KB, 3.801 allocs |

Trang 50 dòng tốn hằng số theo quy mô dữ liệu — đúng mục tiêu của phân trang. Phần render frontend (Task 03.1 ảo hoá) chưa làm, xem Cook note ở file phase-03.

## 4. Ngưỡng chốt (Task 01.3) — chốt một phần

- **Mốc ảo hoá bảng: từ 100 dòng.** Lý do nằm ở render (mọi dòng đều vào DOM) chứ không
  phải backend — backend ở 500 dòng mới khoảng 9 ms. Phase 03 giữ mốc này; số render
  trên Mac sẽ kiểm chứng lại.
- **Ngân sách backend danh sách: dưới 20 ms ở 500 profile** sau Phase 03 (hiện 9,19 ms
  cho tìm kiếm — đã dưới ngân sách, nên việc Phase 03 ưu tiên là cắt nạp thừa
  accounts/extensions và phân trang để giữ tuyến tính này khi lên 2.000 profile,
  không phải cứu cháy ở 500).
- **Trần profile mở đồng thời: chưa chốt** — chờ RAM/profile ở mục 3.1. Công thức đã
  sẵn: trần = phần RAM dành cho browser (gợi ý 50% RAM máy) chia cho RAM/profile đo
  được, làm tròn xuống.

## 5. Kiểm chứng lại trên Mac sau sửa review (2026-10-27)

Harness nay nằm trong repo: `internal/store/profiles_bench_test.go`, fixture ở
`list_page_test.go`. Mỗi profile có 1 account và 1 extension; database nằm trong
thư mục tạm. Không đụng dữ liệu thật. Các số Linux ở trên là lịch sử, không thể
so trực tiếp với lượt này do khác máy và fixture.

Máy đo: Apple M2, darwin/arm64, Go 1.26.10. Lệnh tái lập:

```sh
go test ./internal/store -run '^TestProfileListUsesOrderIndex$' \
  -bench '^BenchmarkProfileList$' -benchmem -benchtime=100x -count=1 -v
```

| Cách gọi | 100 profile | 300 profile | 500 profile |
|---|---|---|---|
| Toàn bộ | 0,753 ms | 2,570 ms | 3,579 ms |
| Trang 50 dòng | 0,433 ms | 0,470 ms | 0,438 ms |
| Tìm kiếm bỏ dấu `kenh` | 0,883 ms | 2,511 ms | 4,012 ms |
| Trang 50 + count | 0,442 ms | 0,460 ms | 0,469 ms |

Trang 50 cấp phát khoảng 230.544 byte/lần (4.060 allocations); trang kèm count
khoảng 231.072 byte. EXPLAIN xác nhận `profiles_list_order` được sử dụng, không
cần TEMP B-TREE cho thứ tự phân trang mặc định. Test 500 profile cùng timestamp
xác nhận thứ tự ổn định, không trùng/thiếu dòng; test tìm kiếm phủ tiếng Việt có
dấu, `đ`, dạng dấu tổ hợp và ký tự `%`/`_` được hiểu như chữ thường.

Đây là backend benchmark, không phải số đo end-to-end service/render, không
chứng minh UI đã ảo hoá. Thời gian mở nguội/nóng, RAM sau 60 giây và render bảng
vẫn chưa đo; không tự suy ra trần RAM mặc định từ các số SQL này.
