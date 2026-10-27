# Hợp đồng đo hiệu năng và môi trường

Các ngân sách dưới đây là **tiêu chí phải đo**, không phải kết quả đã đạt. Số SQL
trung bình ở [baseline](../reports/baseline.md) không chứng minh p95 hay UI đạt.
Không hạ ngân sách chỉ để khớp kết quả; muốn đổi phải ghi trade-off và xin user duyệt.

## Môi trường chuẩn

- Mac tham chiếu đã xác nhận: Apple M2, RAM 16 GiB, macOS 26.7.2, arm64.
- Windows tham chiếu cần chuẩn bị: Windows 11 x64, 16 GiB RAM, >=4 physical cores,
  SSD; ghi model CPU/GPU/WebView2 thực tế. Windows 10 x64 có lane compatibility riêng.
- Máy 32 GiB: đo thêm để xác nhận công thức trần, không giả số từ máy 16 GiB.
- Ghi commit, engine version/hash/catalog, OS, runtime WebView, chế độ build,
  viewport, theme, density, tải nền, test fixture/hash và lệnh trước mỗi lượt.
  Dùng app release build; đo UI native WebView, không lấy JSDOM thay số render.
- Không chạy đồng thời build/race/vet/benchmark/browser acceptance. Lượt bị swap,
  update OS hoặc timeout do tranh tài nguyên phải đánh dấu invalid, giữ log và lý
  do; không âm thầm bỏ mẫu chậm hợp lệ. Báo cả số invalid và dữ liệu thô hợp lệ.
- Dùng thư mục test có marker, proxy mock/local fixture và tài khoản rác. Không
  tái sử dụng dữ liệu thật, không tắt kiểm hash/chữ ký để ép engine chạy.

## Phương pháp

- Query/UI: 5 warm-up rồi >=30 mẫu hợp lệ/case, p50/p95/max và raw timing. SQL
  microbench thêm `-benchmem`, >=100 iterations; ns/op trung bình phải ghi đúng là
  mean, không gọi là p95. Chạy hai lượt độc lập, chênh p50 <=15%; lệch thì điều tra.
- Fixture 100/300/500 profile; tối thiểu một account và một extension/profile,
  thêm tags, proxy exits, Unicode, pin, folder, health states và timestamp trùng.
  Generator cho app phải được lưu trong repo và tách data dir thật. Fixture Go
  không tự coi là dữ liệu đã seed vào app. 2.000 profile là stress, chưa là gate MVP.
- Browser cold: không còn process của fixture và mỗi mẫu dùng bản sao profile test
  chuẩn chưa mở trong lượt đó. Warm: đóng hẳn rồi mở lại cùng profile test đã mở
  ít nhất một lần. Không gọi đây là cold OS cache; không purge cache hệ thống.
- Mỗi engine/platform có >=30 mẫu cold và >=30 warm trên local test page. Đo
  riêng enqueue acceptance, queue wait, Starting -> pipe/page ready và tới lúc
  capture/check fingerprint hoàn tất. Không trừ queue wait khỏi số end-to-end
  nhưng không trộn nó vào thời gian browser startup.
- RAM: tổng RSS của cây browser theo PID + start time tại giây 60, N=1/3/5 và
  min(10, trần đủ an toàn để đo). >=5 lượt/N và >=20 mẫu hợp lệ/engine/lane;
  ghi cả app RSS và host memory pressure.
  RSS có thể đếm trùng shared pages: dùng như ước lượng bảo thủ, không cam kết RAM
  chính xác hay chống OOM. Dừng lượt nếu có pressure nghiêm trọng.

## Ngân sách nghiệm thu

| ID | Đo ở đâu | Điều kiện đạt |
|---|---|---|
| PERF-01 | Store, 500 profile, list/search bỏ dấu | p95 <=20 ms, đúng tập kết quả; báo mean/alloc riêng |
| PERF-02 | Service + bridge, trang 50 dòng kèm total | p95 <=50 ms, gồm enrichment/health/proxy, không chỉ SQL |
| PERF-03 | Profiles UI, 500 profile, từ yêu cầu dữ liệu tới paint đầu có dòng | p95 <=250 ms; không tính thời gian boot toàn app |
| PERF-04 | Gõ ô tìm kiếm | Input paint p95 <=50 ms; debounce trailing 200 ms; từ phím cuối tới kết quả đúng p95 <=350 ms |
| PERF-05 | Cuộn scripted 10 giây trong bảng | Frame interval p95 <=32 ms và frame >50 ms <1%; 3 lượt/fixture/theme/density |
| PERF-06 | Virtual rows | Viewport bảng 720 px: <60 data rows ở mọi vị trí cuộn; viewport khác theo `ceil(height/rowHeight)+2*overscan+2`, overscan <=8 |
| PERF-07 | SubmitLaunch/SubmitMany phía service | p95 <=100 ms cho 1/10 profile, không chờ browser ready; ack trên UI p95 <=250 ms |
| PERF-08 | Browser startup sau thay đổi scheduler | p50 không tăng >15% và p95 không tăng >20% so với baseline cùng engine/host/fixture; không dùng số Mac suy ra Windows |
| PERF-09 | Synchronizer, 1 chính + 2 đích, fixture local | capture -> kết quả input/DOM đích p95 <=100 ms, p99 <=250 ms, >=1.000 sự kiện có kết quả trên mỗi đích |

Ngưỡng PERF-09 không gồm network/site response. Đo đồng hồ ở controller và quan
sát kết quả thực ở đích; CDP Call trả về đơn thuần không được gọi là input đã hiện.
Mất/nhân đôi/đảo thứ tự key/text ở fixture xác định phải bằng 0, độc lập độ trễ.
Các field nhạy cảm bị chặn là kết quả safety đúng, không tính thành event thành công.

## RAM và trần mặc định

Sau khi đủ dữ liệu từng platform, lấy `R95` bảo thủ từ RSS/profile đo ở các N,
chọn giá trị lớn nhất trong engine hỗ trợ trên platform đó. Ngân sách browser
`B = 0,5 * RAM vật lý`; auto ceiling `max(1, floor(B / (1,25 * R95)))`.
Ghi B, R95, hệ số, kết quả 16/32 GiB, giả định và cảnh báo tab nặng. Không dùng
RAM trống nhất thời để làm trần dao động sau mỗi lần mở.

Đây là đề xuất công thức phải được kiểm thực nghiệm, không phải số đã chốt.
Chưa có R95 thì chưa được đóng Phase 01/02 hoặc gọi default là RAM-derived.
Không âm thầm bật unlimited khi không đo được; policy fallback ở hợp đồng queue.

## Bằng chứng bắt buộc

Lưu raw timing CSV, script và summary vào artifact test có đường dẫn/hash. Report
chỉ ghi metadata, mẫu lỗi đã redacted và kết quả; không đưa cookie, login, nội dung
nhập hay query chứa dữ liệu thật vào log. UI đọc text/attributes/computed style
trước; chỉ chụp element cần nhìn khi kiểm layout thực sự.
