# Kiểm trình duyệt và fingerprint

Danh sách trang web để kiểm một profile: IP, rò rỉ mạng, fingerprint và độ nhất quán. Mở các trang này
**bên trong cửa sổ của profile**, không mở bằng Chrome thường. Làm theo thứ tự từ trên xuống: phần 1
nhanh nhất và bắt được lỗi nặng nhất.

Trong các bảng dưới, "IP proxy" là IP ra mà trang Proxies hiện cho proxy của profile.

## 1. IP và mạng

| Trang | Điều phải thấy |
|---|---|
| `https://ipinfo.io/json` | IP proxy; quốc gia và múi giờ của IP đó |
| `https://browserleaks.com/ip` | Cùng IP proxy. **Không** thấy IP thật ở bất kỳ dòng nào |
| `https://browserleaks.com/webrtc` | Mục "WebRTC Leak Test" không lộ IP thật hay IP mạng nội bộ (192.168.x.x) |
| `https://dnsleaktest.com` (bấm Standard test) | DNS server ở quốc gia của proxy hoặc thuộc nhà cung cấp proxy. Thấy DNS của nhà mạng Việt Nam là bị lộ |

## 2. Chấm điểm tổng thể

| Trang | Điều phải thấy |
|---|---|
| `https://www.browserscan.net` | Điểm cao. **Không** có "Canvas Tampering" hay "WebGL exception". Múi giờ và ngôn ngữ khớp IP |
| `https://pixelscan.net/fingerprint-check` | "Your Browser Fingerprint is consistent"; không báo masking hay automation |
| `https://abrahamjuliot.github.io/creepjs/` | "like headless" và "stealth" thấp, ít "lies". Trang nặng, chờ 10–20 giây |
| `https://deviceandbrowserinfo.com/are_you_a_bot` | "You are human!" |
| `https://iphey.com` | "Trustworthy"; múi giờ, vị trí và IP khớp nhau |

"Canvas Tampering" và "WebGL exception" là hai lỗi engine Fury bị BrowserScan bắt ở mọi lượt đo;
Apostate không bị. Profile Apostate mà thấy hai dòng này là có vấn đề cần báo lại.

## 3. Kiểm từng phần

Dùng khi một trang ở phần 2 báo lỗi, để tìm đúng chỗ hỏng.

| Trang | Kiểm |
|---|---|
| `https://browserleaks.com/canvas` | Canvas có mã riêng; đóng rồi mở lại profile thì mã **giữ nguyên** |
| `https://browserleaks.com/webgl` | GPU hợp với máy Mac (Apple M-series), không phải SwiftShader hay llvmpipe |
| `https://browserleaks.com/fonts` | Danh sách font là font của macOS |
| `https://browserleaks.com/javascript` | Ngôn ngữ, múi giờ, độ phân giải màn hình hợp lý |
| `https://amiunique.org/fingerprint` | Chi tiết từng thuộc tính so với dữ liệu lớn |
| `https://bot.sannysoft.com` | Mọi dòng xanh; `webdriver` là `false` |
| `https://fingerprint.com/demo` | Visitor ID: hai profile khác nhau ra hai ID khác nhau; cùng profile mở lại ra cùng ID |

## 4. Ba phép so quan trọng nhất

1. **Hai profile với nhau.** Mở 2 profile cùng lúc, vào `fingerprint.com/demo` và
   `browserleaks.com/canvas`. Mã của hai profile phải **khác** nhau. Giống nhau nghĩa là các trang có
   thể liên kết hai tài khoản.
2. **Một profile qua hai lần mở.** Đóng rồi mở lại cùng profile. Mã canvas và Visitor ID phải **giữ
   nguyên**. Đổi nghĩa là nền tảng thấy "thiết bị lạ" mỗi lần đăng nhập.
3. **Với kết quả của app.** Ngăn chi tiết của profile, nhóm Checks, phải khớp với BrowserScan và
   Pixelscan. App báo đạt mà trang báo lỗi (hoặc ngược lại) là điều cần báo lại.

## 5. Đọc kết quả cho đúng

- **Proxy chậm** làm CreepJS và BrowserScan tải lâu hoặc lỗi giữa chừng. Thử lại một lần trước khi kết
  luận.
- **IP datacenter** (DigitalOcean, AWS, Vultr…) bị iphey, pixelscan ghi "datacenter / hosting IP" hoặc
  trừ điểm. Đó là do proxy, không phải lỗi trình duyệt. Để chấm phần trình duyệt, nhìn canvas, WebGL,
  múi giờ và WebRTC, bỏ qua loại IP. Tài khoản thật nên đi qua proxy residential sticky.
- **"Different time zones"** trên BrowserScan có thể xuất hiện dù đúng, khi ipinfo và BrowserScan dùng
  hai cơ sở dữ liệu IP khác nhau. So múi giờ của profile với ipinfo là đủ.
- **Không có proxy:** mọi trang đều thấy IP và vị trí thật của bạn. Chỉ dùng để thử chức năng.

## 6. Chạy tự động

`tools/bench/detect-sites.mjs` mở lần lượt ipinfo, CreepJS, BrowserScan, deviceandbrowserinfo và
Pixelscan rồi in các dòng kết luận. Công cụ nối vào profile qua cổng chẩn đoán:

1. Ngăn chi tiết của profile: bật "Open a diagnostics port on the next launch" (tiếng Việt: "Mở cổng
   chẩn đoán ở lần mở tới").
2. Mở profile. Ngăn chi tiết hiện "Port <số> open".
3. Chạy (thay `<số>` bằng số cổng; `SITES` để chọn trang, bỏ đi thì chạy hết):

   ```sh
   cd ~/Heroin/anyprofile
   CDP_PORT=<số> SITES=ipinfo,creepjs,browserscan node tools/bench/detect-sites.mjs x x -
   ```

4. Đóng profile. Cổng tự tắt sau lần mở đó.

Khi cổng chẩn đoán mở, mọi chương trình trên máy đều điều khiển được trình duyệt, kể cả đọc cookie.
Không bật cổng này trên profile đang đăng nhập tài khoản thật.

Hướng dẫn test các chức năng khác của app nằm ở `docs/manual-testing.md`.
