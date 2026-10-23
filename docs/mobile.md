# Máy mobile

Mỗi máy mobile là một máy Android riêng, có proxy riêng: app trong máy chỉ ra internet qua proxy đó, và mất proxy
thì mất mạng chứ không lộ IP thật. Agent AI điều khiển được máy qua MCP (`docs/mcp.md`, mục "Máy mobile").

Máy mobile thuộc **AnyProfile Pro**. Không có key Pro thì trang Mobile vẫn hiện, nhưng không tạo hay mở được máy.

## Chạy trên gì

| | macOS (Apple Silicon) | Windows 10/11 x64 |
|---|---|---|
| Máy Android | Máy ảo LineageOS 23 (Android 16) trên QEMU, dùng Hypervisor của macOS | Một instance LDPlayer 9 (Android 9) |
| Cần cài | Không. Lần đầu bấm **Tải về** trên trang Mobile: tổng khoảng 1,1 GB gồm bộ chạy máy ảo (20 MB) và image Android | LDPlayer 9 cài sẵn, bật ảo hoá (VT-x / AMD-V) trong BIOS |
| Google Play | Không có. Cài app bằng file APK (mục "Cài app" bên dưới) | Có store của LDPlayer; cài APK cũng được |
| Mạng đi qua proxy bằng | Mạng ảo trong AnyProfile: mọi kết nối TCP của máy đi qua proxy | App tunnel (SocksTun) trong máy, VPN luôn bật và chặn khi VPN tắt |

Image Android lấy từ bản dựng LineageOS của jqssun (`android-lineage-qemu`), đã ghim bằng SHA256 và kiểm chứng nhận
build. AnyProfile không lưu trữ image.

## Cần bao nhiêu máy tính

- **RAM:** mỗi máy mặc định 3 GB (2–6 GB). AnyProfile cảnh báo khi mở thêm một máy vượt quá: 1 máy với RAM ≤ 8 GB,
  2 máy với ≤ 16 GB, 3 máy với ≤ 24 GB, 4 máy khi nhiều hơn. Cảnh báo không chặn; máy chậm hẳn khi phải swap.
- **Đĩa:** tải image cần trống 4 GB một lúc; mỗi máy mới cần trống 3 GB. Trên macOS, máy mới là bản sao APFS của
  image nên lúc đầu gần như không tốn chỗ, rồi lớn dần theo app và dữ liệu.
- **Thời gian (đo trên MacBook M2):** tạo máy 35–45 giây, mở 27–33 giây, đóng khoảng 5 giây.

## Cài app

Máy đang chạy có nút **Cài app** trên trang Mobile: chọn một hay nhiều file `.apk` trên máy tính. Kéo file APK thả
vào cửa sổ của máy cũng cài được.

- Máy không có Google Play. Lấy APK ở [APKMirror](https://www.apkmirror.com/) (file gốc do chính hãng ký) hoặc
  [F-Droid](https://f-droid.org/) (app mã nguồn mở).
- Trên Mac chọn bản **arm64-v8a**. Bản chỉ cho x86 báo "APK này không dành cho điện thoại ARM".
- Gói chia nhỏ (`.apks`, `.xapk`, `.apkm`) chưa cài được: tải bản APK đơn nếu có.

## Màn hình và độ mượt (macOS)

Máy ảo không có GPU: Android vẽ màn hình bằng CPU, và cửa sổ của máy (scrcpy) nén hình bằng CPU. Vì vậy màn hình máy
là 720×1600, như điện thoại tầm trung. Đo trên M2 khi cuộn trang Cài đặt: ở 1080×2400 có 42% khung hình bị giật và
máy ảo dùng 3,2 nhân CPU; ở 720×1600 còn 16% và 1,6 nhân. Đóng cửa sổ của máy (máy vẫn chạy cho agent) thì gần như
hết giật.

## Google hỏi "Bạn không phải robot"

Trang kiểm tra của Google (`google.com/sorry`) phụ thuộc vào **IP**. Máy không có proxy dùng IP của máy tính; nếu
Google đang nghi IP đó thì trình duyệt thường trên máy tính cũng bị hỏi y như vậy. Gắn proxy riêng cho máy (tốt nhất
là proxy dân cư hoặc 4G) và đăng nhập tài khoản Google trong máy thì ít bị hỏi hơn. AnyProfile không vượt captcha.

## Proxy, múi giờ, ngôn ngữ

- Chọn proxy khi tạo máy hoặc sửa sau (máy phải đóng). Không chọn proxy thì máy dùng IP của máy tính.
- **Mạng đóng khi lỗi:** proxy chết, sai mật khẩu hay bị tắt thì máy không có mạng. Không có đường nào đi thẳng ra
  internet.
- **DNS** cũng đi qua proxy (DNS-over-HTTPS tới 1.1.1.1, dự phòng DNS qua TCP), nên nhà mạng của bạn không thấy tên
  miền máy hỏi.
- **UDP, ICMP (ping) và IPv6 bị chặn.** Proxy HTTP/SOCKS chỉ chở TCP; để UDP đi thẳng sẽ lộ IP thật. App gọi video
  hay game cần UDP sẽ chạy kém hoặc không chạy. QUIC tự lùi về TCP.
- **Múi giờ và ngôn ngữ** đặt theo quốc gia của IP ra của proxy mỗi lần mở máy.
  - macOS: múi giờ cho cả máy; ngôn ngữ đặt cho từng app (tính năng của Android 13 trở lên), vì đổi ngôn ngữ cả máy
    cần quyền root mà image không cho.
  - LDPlayer: múi giờ có ngay; ngôn ngữ có hiệu lực từ lần mở thứ hai, vì Android 9 chỉ đọc nó lúc khởi động.
- **Đồng hồ** được chỉnh theo giờ máy tính mỗi lần mở. Máy ảo không đồng bộ giờ qua mạng được (NTP là UDP); lệch
  giờ làm HTTPS hỏng.

## Dùng hằng ngày

- **Mở** máy chạy ở chế độ nền; bấm **Hiện cửa sổ** để xem và thao tác bằng chuột, bàn phím (cửa sổ dùng scrcpy).
  Đóng cửa sổ không đóng máy.
- **Đóng** máy tắt Android đàng hoàng. Thoát AnyProfile cũng đóng mọi máy đang chạy.
- AnyProfile bị tắt ngang (crash, `kill`): mở lại thì máy đang chạy được nhận lại, không phải mở lại.
- **Sao lưu / Khôi phục** (máy phải đóng): sao lưu chép đĩa của máy; khôi phục đưa máy về đúng bản đó, mất mọi thứ
  sau lúc sao lưu.
- **Xoá** đưa máy vào Thùng rác như profile; xoá hẳn mới xoá đĩa. Dung lượng máy mobile hiện ở Cài đặt → Lưu trữ.
- Windows: đừng đổi tên hay xoá instance `ap-…` trong LDPlayer. Instance bị xoá ở ngoài thì khi mở, AnyProfile
  báo máy không còn trong LDPlayer.

## Ứng dụng vẫn biết đây là máy ảo

Nói thẳng: AnyProfile **không giả một chiếc điện thoại thật và không giấu dấu vết máy ảo**. App có thể nhận ra qua
nhiều thứ, ví dụ:

- tên model, hãng, phần cứng (`ro.product.*`, `ro.hardware`), bản dựng `userdebug` hoặc của LDPlayer;
- không có cảm biến thật (gia tốc, con quay), pin luôn đầy, không có SIM, không có sóng di động;
- GPU giả lập, CPU khác điện thoại, Play Integrity không đạt (máy ảo không có Google Play);
- trên LDPlayer: file, process và app riêng của LDPlayer.

Thứ AnyProfile cho: mỗi máy là một thiết bị Android riêng với dữ liệu riêng, IP riêng, múi giờ và ngôn ngữ khớp IP,
không rò IP qua UDP, DNS hay IPv6. Nền tảng vẫn có thể hạn chế tài khoản dùng trên máy ảo hoặc giả lập; rủi ro đó
thuộc về người dùng.

## An toàn trên máy tính

- Mỗi máy mở một cổng adb chỉ trên `127.0.0.1`, **không có mật khẩu**: chương trình khác chạy trên máy tính của bạn
  có thể điều khiển máy mobile qua cổng đó. Máy tính có phần mềm lạ thì đừng để tài khoản quan trọng trong máy
  mobile.
- Trên Windows, proxy của máy đi qua một relay SOCKS5 trên `127.0.0.1`, cũng không mật khẩu; relay chỉ nhận
  CONNECT tới đúng proxy của máy đó.
- AnyProfile dùng server adb riêng (cổng 5137), không đụng adb bạn đang dùng cho điện thoại thật.

## Khi có lỗi

| Thấy | Làm |
|---|---|
| Máy mở nhưng không vào được mạng | Kiểm tra proxy ở trang Proxy (Check). Đây là cách máy báo proxy hỏng, không phải lỗi máy |
| "Không đủ chỗ trống" khi tạo hay tải | Dọn đĩa: tải image cần 4 GB, máy mới 3 GB |
| Windows: "Máy tính này chưa cài LDPlayer 9" | Cài LDPlayer 9 từ ldplayer.net, rồi mở lại AnyProfile |
| Windows: máy không mở, LDPlayer báo VT | Bật ảo hoá trong BIOS; tắt Hyper-V nếu LDPlayer yêu cầu |
| App hiện sai ngôn ngữ trên LDPlayer | Đóng rồi mở máy lần nữa |

`logs/anyprofile.log` ghi việc tạo, mở, đóng máy và lỗi kèm theo.
