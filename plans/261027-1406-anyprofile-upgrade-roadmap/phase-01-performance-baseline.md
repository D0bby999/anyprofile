---
title: "Phase 01: Đo baseline hiệu năng"
status: in_progress
effort: 1d
---

# Phase 01: Đo baseline hiệu năng

## Context Links

- Script đo sẵn có: `tools/bench/` (README: script đo cho developer, không nằm trong app bundle).
- Đường mở profile: `internal/launcher/`, `internal/service/`.
- Bảng profile: `frontend/src/components/profile-table.tsx`.
- Kiểm thử tay hiện tại: `docs/manual-testing.md`.

## Goal

Có một bộ số đo gốc, lặp lại được, cho bốn trục: thời gian mở một profile tới khi dùng được,
RAM của một profile đang mở, thời gian render và cuộn bảng profile khi danh sách lớn, và
thời gian truy vấn danh sách profile trong SQLite. Mọi phase hiệu năng sau so với bộ số này.

## Hợp đồng hiện hành

Theo [performance](contracts/performance.md) và [matrix](acceptance-matrix.md):
ngưỡng PERF-01..09 là mục tiêu chưa đo đầy đủ, không phải kết quả đã đạt. Query/UI
p50/p95 có >=30 mẫu; cold/warm browser >=30 mẫu mỗi mode/engine/lane, không suy từ
mean benchmark. Host Mac đã xác nhận M2/16 GiB/macOS 26.7.2; Windows/32 GiB cần
môi trường thật và artifact riêng. Mốc 100 dòng trong baseline cũ là đề xuất tạm,
chưa được dùng như bằng chứng render. Chưa có dữ liệu RAM thì không đóng default
guard; thiếu RAM không ngăn sửa cleanup ở Phase 02.

## Tasks

### Task 01.1: Đo thời gian mở và RAM

- **Target files:** `tools/bench/` (thêm script nếu thiếu), kết quả ghi vào `reports/baseline.md` trong thư mục plan này.
- **Steps:**
  1. Đo từng engine/platform được hỗ trợ, cold/warm theo định nghĩa hợp đồng, >=30 mẫu/mode; dùng profile test đã chuẩn bị và local fixture.
  2. Tách enqueue acceptance, queue wait, browser ready và fingerprint complete; ghi p50/p95/max, raw timing và không trộn queue wait vào startup.
  3. Đo RAM cây process giây 60 ở N=1/3/5 và 10 nếu an toàn; >=5 lượt/N và >=20 mẫu RAM hợp lệ/engine/lane, ghi memory pressure/app RSS.
- **Verify:** Đủ metadata/hash/script/raw artifact theo matrix, hai lượt đo p50 lệch <=15% hoặc có blocker điều tra; Windows không có Fury là N/A, thiếu Fury đúng hash trên Mac là BLOCKED.

### Task 01.2: Đo bảng và truy vấn danh sách

- **Target files:** `tools/bench/`, `internal/service/` (hàm liệt kê profile), `internal/store/`.
- **Steps:**
  1. Sinh dữ liệu giả lập 100, 300, 500 profile trong một thư mục dữ liệu riêng, không đụng dữ liệu thật.
  2. Đo store riêng với service+bridge/page50+total; UI native đo first data paint, input paint, final search paint và frame interval, không dùng JSDOM thay render timing.
  3. Bật `EXPLAIN QUERY PLAN` cho truy vấn danh sách, ghi rõ có dùng index hay quét toàn bảng.
- **Verify:** `reports/baseline.md` có bảng số cho 3 mốc; phần truy vấn ghi rõ index đang dùng hoặc ghi "chưa có index" kèm câu truy vấn.

### Task 01.3: Chốt ngưỡng cho Phase 02 và 03

- **Steps:**
  1. Từ số đo RAM/profile, đề xuất trần số profile mở đồng thời mặc định cho máy 16 GB và 32 GB.
  2. Từ số đo bảng, chốt mốc bắt đầu ảo hoá (số dòng) và ngân sách render (mục tiêu: gõ tìm kiếm không giật khung hình ở 500 dòng).
  3. Ghi ngưỡng vào cuối `reports/baseline.md` để Phase 02/03 trích dẫn.
- **Verify:** Baseline ghi R95/B/hệ số/công thức trần 16/32 GiB và evidence thực, ngân sách UI theo PERF-03..06; không gọi fallback cap 2 provisional là trần đã đo. Các metric chưa đạt/chưa đo giữ phase mở.

## Cook note (2026-10-27)

Cook bằng Muse (ak-cook 2.5.1, files-first; không có `ak` CLI và không có shell trên máy đích nên trạng thái phase giữ nguyên ở frontmatter, tiến độ ghi tại đây):

- Phần tĩnh của Task 01.2 đã xong, ghi ở `reports/baseline.md` mục 1.
- Phần đo truy vấn của Task 01.2 đã chạy thật trên clone GitHub của repo (máy Linux của Muse, Go 1.26.10): benchmark `ListProfiles` ở 100/300/500 profile và `EXPLAIN QUERY PLAN` — số và plan ghi ở `reports/baseline.md` mục 3.2. Test sẵn có của `internal/store` đạt trên clone.
- Còn chưa đo: Task 01.1 (thời gian mở và RAM trên macOS — máy Linux không chạy được engine macOS) và render frontend trên Mac. Vì vậy Verify của phase chưa đạt trọn, phase chưa đóng; ngưỡng ở Task 01.3 mới chốt được phần danh sách, phần trần đồng thời chờ số RAM/profile từ Mac.

## Cập nhật sau review (2026-10-27)

Đã bổ sung harness benchmark trong repo và chạy trên Mac; số liệu/lệnh tái lập ở
`reports/baseline.md` mục 5. Chưa đo RAM/profile, mở nguội/nóng và render UI, nên
Task 01.1 và exit gate vẫn chưa đạt.

## Phase exit gate

- Mọi Verify và ACC-09/05 liên quan đạt trên required lanes; `reports/baseline.md` có raw artifacts/script/hash và đánh dấu từng metric mean/p50/p95 đúng loại.
- Thêm generator app fixture 100/300/500 vào repo trước khi đo native UI. Fixture Go/test SQL hiện có không tự thay generator này.

## Failure Protocol

Giữ phase mở khi Verify không đạt. Ghi case/lane/revision/fixture, lệnh, raw output,
expected/actual, blocker owner và next action theo matrix. Không rerun mù hoặc đổi
ngưỡng để làm xanh; môi trường invalid cần có bằng chứng trước khi đo lại. Có thể
nhờ reviewer độc lập nếu runtime có, nhưng không giả tool/subagent tồn tại. Thiếu
máy/engine/quyết định thì báo user, không suy số đo hoặc đóng gate bị chặn.
