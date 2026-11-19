# Ứng dụng chạy được trên máy mobile

Cập nhật: 19/11/2026. Máy: macOS Apple Silicon, image LineageOS 23.2.20261019 + lớp nền MindTheGapps & danh tính thiết bị (mặc định bật cho máy mới ở preset Cân bằng / Tương tác).

Cách thử: cài từ Google Play Store / Aurora Store (hoặc APK), mở, đăng nhập bằng email/số điện thoại, dùng 10 phút, thoát rồi mở lại.

## Kết quả

| App | Trạng thái | Ghi chú |
|---|---|---|
| TikTok | Hỗ trợ (cần proxy dân cư/4G sạch) | Đã ẩn CPU Apple (`midr`), áp danh tính thiết bị thật (`release-keys`, `user`, `verifiedbootstate=green`), SIM/nhà mạng theo quốc gia proxy, pin thực tế và Magisk DenyList. Vẫn cần proxy dân cư/4G sạch khi đăng ký mới |
| YouTube (app) | Hỗ trợ (trên máy bật Google Play / GMS) | Máy mới tạo (Cân bằng / Tương tác) cài sẵn MindTheGapps (Google Play Services + Play Store) và bật âm thanh scrcpy opus. Máy tạo trước bản nâng cấp không có GMS vẫn dùng `m.youtube.com` |
| Chrome | Chạy | Dùng bình thường; Google có thể hỏi captcha theo IP |
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
