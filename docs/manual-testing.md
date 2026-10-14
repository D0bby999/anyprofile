# Hướng dẫn test bằng tay

Có hai bộ dữ liệu riêng biệt, không bao giờ trộn vào nhau:

| Bộ dữ liệu | Thư mục | Dùng để |
|---|---|---|
| Mẫu | `~/Library/Application Support/AnyProfile-Demo` | Xem và bấm thử mọi màn hình với khoảng 60 profile |
| Thật | `~/Library/Application Support/AnyProfile` | Đăng nhập tài khoản thật qua proxy thật |

AnyProfile chỉ chạy **một cửa sổ** một lúc. Nếu app đang mở, lệnh mở bộ kia chỉ đưa cửa sổ cũ lên trước.
Vì vậy luôn **thoát hẳn app (⌘Q)** trước khi đổi bộ dữ liệu. Nhìn số profile ở cột trái để biết mình đang ở bộ nào:
62 là bộ mẫu, 2 là bộ thật.

## Phần A: bộ dữ liệu mẫu

### Mở, làm lại, xoá

```sh
cd ~/Heroin/anyprofile
# Mở app với bộ mẫu
open -n build/bin/AnyProfile.app --env ANYPROFILE_DATA_DIR="$HOME/Library/Application Support/AnyProfile-Demo"
# Làm lại từ đầu (thoát app trước)
go run ./tools/seed-demo -dir "$HOME/Library/Application Support/AnyProfile-Demo" -reset
# Xoá hẳn bộ mẫu
rm -rf "$HOME/Library/Application Support/AnyProfile-Demo"
```

Công cụ seed từ chối ghi vào thư mục thật và không ghi gì vào Keychain. Engine được nhân bản
từ bộ thật (bản sao APFS, gần như không tốn thêm ổ đĩa), nên không phải tải lại.

### Trong bộ mẫu có gì

| Nhóm | Số lượng | Để thử |
|---|---|---|
| TikTok US 01–10, TikTok VN 01–06 | 16 | Tag "Kênh chính" và "Nuôi"; 3 kênh US được ghim |
| YouTube Studio, Facebook Ads, Seller, Instagram, X, Amazon… | 27 | Nhiều nền tảng, tag "Khách A", "Khách B", "Shop" |
| Partner 1–3 | 3 | Nền tảng tự tạo "Partner portal" |
| Fury cũ 1–5 | 5 | Engine Fury; dùng để thử nhân bản sang Apostate |
| Thử trực tiếp 1–4 | 4 | **Không có proxy, mở được thật**; dùng cho ảnh chụp, cookie, mật khẩu, 2FA |
| Mẫu: … | 10 | Mỗi profile minh hoạ một trạng thái sức khỏe (xem bảng dưới) |
| Đã bỏ 1–3 | 3 | Nằm trong thùng rác |

Có 65 proxy: mỗi profile một proxy riêng, cộng thêm 3 proxy trống (GB, SG, VN), một proxy sắp hết hạn,
một proxy đã hết hạn và một cổng xoay IP (port 823). **Các proxy mẫu không kết nối được**: chúng dùng
dải địa chỉ dành cho tài liệu (203.0.113.x, 198.51.100.x). Mở một profile có proxy mẫu sẽ báo lỗi
proxy, và đó chính là cách thử đường báo lỗi.

Các profile "Mẫu":

| Profile | Phải thấy ở cột Health |
|---|---|
| Mẫu: fingerprint đổi | Đỏ, fingerprint thay đổi (canvas, GPU) |
| Mẫu: đổi quốc gia | Đỏ, quốc gia IP đổi từ US sang CA; ô proxy có huy hiệu đỏ |
| Mẫu: đổi IP | Thông tin, IP ra đã đổi |
| Mẫu: lệch múi giờ | Vàng, múi giờ IP khác múi giờ profile; có nút "Match the IP" |
| Mẫu: không đo được IP | Vàng, không đo được IP ra |
| Mẫu: WebRTC lộ IP | Đỏ, kiểm tra WebRTC hỏng |
| Mẫu: ngôn ngữ lệch | Vàng, kiểm tra ngôn ngữ hỏng |
| Mẫu: chưa đo fingerprint | Vàng, chưa đo |
| Mẫu: proxy sắp hết hạn | Vàng, proxy hết hạn sau 3 ngày |
| Mẫu: proxy đã hết hạn | Đỏ, proxy đã hết hạn |

Mở một profile "Mẫu" sẽ ghi đè trạng thái giả bằng kết quả đo thật, nên hãy xem trước khi mở.

### Danh sách thử trên bộ mẫu

Mỗi mục ghi điều phải thấy. Thử ở cả giao diện sáng và tối (Settings), và một lần với cửa sổ hẹp.

1. **Thanh bên.** "Needs attention" đếm đúng 9 (mọi profile "Mẫu" trừ "đổi IP", vốn chỉ là thông tin). Nhóm Tags có 8 tag với 8 màu. Bấm một tag thì bảng chỉ còn profile có tag đó.
2. **Tìm kiếm.** Gõ `tai khoan nuoi` (không dấu) thì ra "TikTok VN 02" nhờ ghi chú. Gõ `#12` thì ra profile số 12.
3. **Bộ lọc và View.** Bật tắt chip lọc trên thanh công cụ. Menu View đổi mật độ và ẩn hiện cột; tắt app rồi mở lại vẫn giữ.
4. **Ghim.** 3 kênh US nằm trên cùng. Chọn vài hàng, ghim nhiều cùng lúc, rồi bỏ ghim.
5. **Bàn phím.** ⌘K mở bảng lệnh và tìm được profile theo tên. Phím `?` mở bảng phím tắt. Esc chỉ đóng hộp trên cùng.
6. **Ngăn chi tiết.** Bấm một hàng. Đổi tên ngay trên tiêu đề. Xem lần lượt các tab Overview (nhóm Checks), Sessions (lịch sử mở, IP ra), Account, Snapshots.
7. **Sức khỏe.** Đi qua 10 profile "Mẫu" và so với bảng trên. Ở "Mẫu: lệch múi giờ", bấm "Match the IP": app sẽ báo lỗi proxy, vì proxy mẫu không kết nối được.
8. **Thao tác hàng loạt.** Chọn 5 hàng, thêm tag, đổi nền tảng.
9. **Tạo profile.** Tạo hàng loạt 3 profile với proxy "auto": mỗi profile nhận một trong 3 proxy trống. Tạo lần thứ tư phải bị chặn vì hết proxy trống.
10. **Engine.** Mở "Fury cũ 1", chọn Duplicate. Bản sao phải là Apostate. Settings → Browser engines hiện cả hai engine, Apostate là mặc định.
11. **Proxy.** Trang Proxies có huy hiệu sắp hết hạn và đã hết hạn, và cổng xoay bị đánh dấu không gán được.
12. **Thùng rác.** Khôi phục "Đã bỏ 1". Xoá vĩnh viễn "Đã bỏ 2" (có hộp xác nhận).
13. **Mở thật một profile.** Mở "Thử trực tiếp 1", chờ cửa sổ trình duyệt hiện, rồi đóng. Tab Sessions có thêm một hàng; nhóm Checks báo kết quả đo thật.
14. **Mật khẩu và 2FA.** Ở "Thử trực tiếp 2", tab Account, lưu một mật khẩu bất kỳ và khoá 2FA thử `JBSWY3DPEHPK3PXP`. Mã 6 số phải khớp với ứng dụng xác thực của bạn khi thêm cùng khoá đó. Nếu macOS hỏi quyền Keychain, chọn **Always Allow**.
15. **Ảnh chụp phiên.** Mở "Thử trực tiếp 1", vào `example.com`, để chạy **hơn 2 phút**, rồi đóng. Tab Snapshots có một ảnh chụp lý do "close". Bấm "Take snapshot", rồi khôi phục ảnh cũ; toast có nút Undo. Settings → Storage hiện tổng dung lượng ảnh chụp.
16. **Cookie.** Ở "Thử trực tiếp 3", chọn Import cookies và chọn file `internal/cookies/testdata/cookies.txt` trong repo (cookie giả của `example.com`). Toast báo đã nhập 2 cookie và có cảnh báo cookie không thuộc TikTok. Khi profile đang mở, Export cookies hiện cảnh báo rồi lưu file JSON. Xoá file đó sau khi xem.
17. **Ngôn ngữ.** Settings đổi sang Tiếng Việt: mọi chữ đổi theo, không còn chữ tiếng Anh sót.
18. **Màn hình chào.** Thoát app rồi mở với một thư mục trống:
    `open -n build/bin/AnyProfile.app --env ANYPROFILE_DATA_DIR=/tmp/ap-fresh`. Phải thấy hướng dẫn 3 bước. Xoá `/tmp/ap-fresh` sau khi xong.

## Phần B: test với tài khoản thật

Làm trên bộ **thật**. Thoát app mẫu trước, rồi mở `build/bin/AnyProfile.app` bình thường (bấm đúp).

### Nguyên tắc an toàn

- Dùng **tài khoản phụ** cho lần thử đầu, không dùng kênh chính.
- Một tài khoản, một profile, một proxy sticky. Không dùng chung proxy giữa các tài khoản.
- Với DataImpulse, dùng port sticky **10000 trở lên**. Port 823 đổi IP mỗi lần kết nối; app sẽ từ chối.
- Không đổi quốc gia proxy của một profile đã đăng nhập. Muốn đổi thì tạo profile mới.
- Không đăng nhập cùng một tài khoản trên cả Fury và Apostate.
- File cookie xuất ra cho phép người khác đăng nhập tài khoản. Giữ kín và xoá sau khi dùng.

### Các bước

1. **Thêm proxy.** Trang Proxies → Import, dán một dòng `host:port:user:pass` (port sticky). Mật khẩu đi thẳng vào Keychain. Sau khi kiểm, proxy hiện quốc gia và IP ra.
2. **Tạo profile.** New profile: engine Apostate (mặc định), chọn proxy vừa thêm, nền tảng TikTok (hoặc nền tảng của tài khoản).
3. **Mở lần đầu.** macOS có thể hỏi quyền với mục "Chromium Safe Storage": chọn **Always Allow**. Nếu chỉ chọn Allow, lần mở sau sẽ mất đăng nhập.
4. **Kiểm khi đang mở.** Ngăn chi tiết: nhóm Checks báo tất cả đạt; tab Sessions có một hàng với IP ra đúng quốc gia proxy. Nếu muốn, vào `browserscan.net` để so.
5. **Đăng nhập** tài khoản phụ như bình thường. Lưu email, mật khẩu và khoá 2FA (nếu có) ở tab Account, để lần sau dùng menu Copy.
6. **Đóng và mở lại** sau vài phút. Phải vẫn đăng nhập, không phải nhập lại mật khẩu.
7. **Ảnh chụp.** Nếu lần mở trước kéo dài hơn 2 phút, tab Snapshots có ảnh chụp "close". Chụp thêm một ảnh bằng tay trước mọi thay đổi lớn.
8. **Qua đêm.** Để profile đóng đến hôm sau rồi mở lại. Phải vẫn đăng nhập, cùng IP (proxy sticky), không có cảnh báo đỏ ở cột Health.
9. **Chuyển phiên (không bắt buộc, chỉ với tài khoản phụ).** Khi profile đang mở, Export cookies. Tạo profile mới với proxy **cùng quốc gia**, Import file đó, mở profile mới: tài khoản đã đăng nhập sẵn. Nền tảng có thể đòi xác minh lại vì máy ảo khác; đó là hành vi bình thường. Xoá file cookie sau khi xong.

Hai bước kiểm còn mở của plan nằm trong các bước trên: bước 4 hoàn tất phase 9, bước 7 hoàn tất phase 11.

### Báo lại khi gặp lỗi

Ghi lại profile nào, bước nào, chữ trên toast hoặc cột Health, và chụp đúng vùng bị lỗi.
Nhật ký nằm ở `~/Library/Application Support/AnyProfile/logs/anyprofile.log` (không chứa mật khẩu).
