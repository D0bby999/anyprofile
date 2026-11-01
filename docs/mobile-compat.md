# Ứng dụng chạy được trên máy mobile

Cập nhật: 01/11/2026. Máy: macOS Apple Silicon, image LineageOS 23.2.20261019.

Cách thử: cài APK (hoặc Aurora Store), mở, đăng nhập bằng email/số điện thoại, dùng 10 phút, thoát rồi mở lại.
Ghi lại app tự đóng, hộp thoại đòi Google Play, hay mất mạng khi gọi video.

## Kết quả

| App | Trạng thái | Ghi chú |
|---|---|---|
| TikTok | Chạy | Cài và mở được; đăng nhập bằng email/số điện thoại |
| Chrome | Chạy | Dùng bình thường; Google có thể hỏi captcha theo IP |
| YouTube (app) | Không chạy | Mở rồi tự đóng vì cần dịch vụ Google Play; dùng `m.youtube.com` |
| YouTube (web) | Chạy | Trong Chrome |
| Google Search | Chạy, có captcha | Captcha theo IP; proxy dân cư/4G giảm bị hỏi |
| Shopee | Chưa kiểm | Cần tài khoản thật để thử |
| Facebook | Chưa kiểm | Cần tài khoản thật để thử |
| Instagram | Chưa kiểm | Cần tài khoản thật để thử |
| Zalo | Chưa kiểm | Cần số điện thoại thật để thử |
| Telegram | Chưa kiểm | Có thể dùng số điện thoại |
| Techcombank | Chưa kiểm | Nhiều app ngân hàng đòi Play Integrity; dự kiến không chạy |
| Liên Quân Mobile | Chưa kiểm | Cần UDP; dự kiến chạy kém hoặc không chạy |

Chỉ ghi "Chạy" khi đã thử đúng cách trên máy này. Đừng suy đoán từ app tương tự.

## Giới hạn không vượt được

- **Không có Google Play / Play Integrity.** App ngân hàng, DRM (Netflix HD), và app bắt buộc Play Services sẽ không chạy.
- **Không có UDP, ICMP, IPv6.** Gọi video, game online, và một số app livestream sẽ hỏng hoặc chậm; QUIC tự lùi về TCP.
- **Không có cảm biến thật** (gia tốc, con quay), không SIM, pin luôn đầy. App kiểm tra phần cứng sẽ nhận ra máy ảo.
- **Không vượt captcha.** Captcha phụ thuộc IP; proxy riêng là cách giảm.

## Cách cài

- Nút **Cài app** trên trang Mobile, hoặc kéo file `.apk` vào cửa sổ máy.
- **Aurora Store** để tải app từ Google Play mà không cần tài khoản Google.
- **F-Droid** cho app mã nguồn mở.

Chi tiết: [mobile-apps.md](mobile-apps.md).
