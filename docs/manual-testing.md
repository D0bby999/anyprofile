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
| TikTok US 01–10, TikTok VN 01–06 | 16 | Tag "Kênh chính" và "Nuôi"; 3 kênh US được ghim; TikTok VN 01–02 có ba nền tảng (TikTok, YouTube, Gmail) |
| YouTube Studio, Facebook Ads, Seller, Instagram, X, Amazon… | 27 | Nhiều nền tảng, tag "Khách A", "Khách B", "Shop" |
| Partner 1–3 | 3 | Nền tảng tự tạo "Partner portal" |
| Fury cũ 1–5 | 5 | Engine Fury; dùng để thử nhân bản sang Apostate |
| Thử trực tiếp 1–4 | 4 | **Không có proxy, mở được thật**; dùng cho ảnh chụp, cookie, mật khẩu, 2FA; số 4 có hai nền tảng (YouTube, Gmail) |
| Mẫu: … | 10 | Mỗi profile minh hoạ một trạng thái sức khỏe (xem bảng dưới) |
| Đã bỏ 1–3 | 3 | Nằm trong thùng rác |

Có 65 proxy: mỗi profile một proxy riêng, cộng thêm 3 proxy trống (GB, SG, VN), một proxy sắp hết hạn,
một proxy đã hết hạn và một cổng xoay IP (port 823). **Các proxy mẫu không kết nối được**: chúng dùng
dải địa chỉ dành cho tài liệu (203.0.113.x, 198.51.100.x). Mở một profile có proxy mẫu sẽ báo lỗi
proxy, và đó chính là cách thử đường báo lỗi.

Các profile "Mẫu":

| Profile | Phải thấy ở cột Status |
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

1. **Thanh bên và thanh trạng thái.** Thanh bên không còn mục Running và Needs attention. Thanh trạng thái
   dưới bảng ghi "9 need attention" (mọi profile "Mẫu" trừ "đổi IP", vốn chỉ là thông tin); bấm vào thì bảng
   chỉ còn 9 profile đó, bấm lần nữa thì hiện lại tất cả. "N running" lọc profile đang chạy theo cùng cách. Nhóm Tags có 8 tag với 8 màu. Bấm một tag thì bảng chỉ còn profile có tag đó.
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
14. **Mật khẩu và 2FA.** Ở "Thử trực tiếp 4" (YouTube và Gmail), tab Account có một khung cho mỗi nền tảng.
    Lưu một mật khẩu bất kỳ cho YouTube và khoá 2FA thử `JBSWY3DPEHPK3PXP` cho Gmail. Mã 6 số phải khớp với
    ứng dụng xác thực của bạn khi thêm cùng khoá đó. Menu ⋯ của hàng có "Copy password · YouTube" và
    "Copy 2FA code · Gmail". Bỏ Gmail khỏi nền tảng: tài khoản Gmail chuyển xuống mục nền tảng đã bỏ, bấm
    "Forget" thì hỏi xác nhận rồi mới xoá. Nếu macOS hỏi quyền Keychain, chọn **Always Allow**.
15. **Ảnh chụp phiên.** Mở "Thử trực tiếp 1", vào `example.com`, để chạy **hơn 2 phút**, rồi đóng. Tab Snapshots có một ảnh chụp lý do "close". Bấm "Take snapshot", rồi khôi phục ảnh cũ; toast có nút Undo. Settings → Storage hiện tổng dung lượng ảnh chụp.
16. **Cookie.** Ở "Thử trực tiếp 3", chọn Import cookies và chọn file `internal/cookies/testdata/cookies.txt` trong repo (cookie giả của `example.com`). Toast báo đã nhập 2 cookie và có cảnh báo cookie không thuộc TikTok. Khi profile đang mở, Export cookies hiện cảnh báo rồi lưu file JSON. Xoá file đó sau khi xem.
17. **Ngôn ngữ.** Settings đổi sang Tiếng Việt: mọi chữ đổi theo, không còn chữ tiếng Anh sót.
18. **Mở theo trang mở đầu.** Mở "Thử trực tiếp 4": cửa sổ có đúng các trang trong ô Start page. Mở thêm tab
    `example.com` và `example.org`, chờ vài giây, **đóng cửa sổ trình duyệt bằng nút đỏ**. Trong khoảng 2
    giây, profile tự chuyển sang đã đóng trong app (không cần bấm Close), và Chromium không còn trên Dock. Mở
    lại: chỉ có các trang trong Start page, không có hai tab vừa mở. Settings → "Khi mở profile" chọn "Mở
    lại các tab lần trước", làm lại các bước trên: lần mở sau hai tab đó hiện lại, không còn tab trống. Chọn
    lại "Luôn mở trang mở đầu" sau khi thử.
19. **Nhiều nền tảng.** Cột nền tảng của "TikTok VN 01" có ba biểu tượng và chữ "TikTok +2"; bấm "YouTube"
    ở cột trái thì profile này vẫn hiện. New profile: bấm "Change platforms" mở một modal riêng; chọn thêm
    YouTube, Gmail rồi Done: ô Platform có ba thẻ, ô Trang mở đầu có đủ ba dòng. Trong modal, Cancel hoặc Esc
    thì không đổi gì; gõ tên rồi Enter bật/tắt nền tảng đầu tiên khớp. Bấm × trên thẻ YouTube thì dòng
    YouTube Studio biến mất, dòng tự gõ thì giữ nguyên. Ngăn chi tiết và "Set platforms" hàng loạt dùng
    cùng modal; Esc trong modal chỉ đóng modal, không đóng ngăn chi tiết hay form New profile.
    TikTok Shop mở Seller Center theo nước của proxy: chọn proxy Mỹ thì ô Start page là
    `https://seller-us.tiktok.com`, đổi sang proxy Việt Nam thì thành `https://seller-vn.tiktok.com`; nước chưa có
    Seller Center (hoặc không proxy) dùng `https://seller.tiktok.com`. Đổi proxy trong ngăn chi tiết cũng đổi theo.
    Shopee (8 nước), Lazada (6 nước) và Amazon Seller Central (22 thị trường) làm giống vậy. Không chọn được "Other"
    cùng nền tảng khác, và mỗi nền tảng chỉ chọn được một lần. Từ nền tảng thứ hai, dưới ô Platform có dòng
    nhắc "Chỉ thêm nền tảng của cùng một người hoặc một kênh"; bỏ bớt còn một thì dòng đó mất.
20. **Extension.** Settings → Extensions → "Add folder…", chọn thư mục `/tmp/ext-spike/ext` hoặc một extension
    đã giải nén bất kỳ. Bật nó cho "Thử trực tiếp 1" trong ngăn chi tiết, mở profile, vào `chrome://extensions`:
    extension có mặt. Mở "Thử trực tiếp 2": không có. Xoá extension trong Settings hỏi xác nhận và nói số profile
    đang dùng.
21. **Bookmark.** Xuất bookmark từ Chrome thật (Bookmark manager → ⋮ → Export bookmarks). Menu ⋯ của
    "Thử trực tiếp 1" (đang đóng) → "Import bookmarks…": toast báo số bookmark; mở profile, thanh bookmark có
    thư mục "Imported". Khi profile đang mở, mục này bị mờ.
22. **API trên máy.** Settings → Local API → On, Copy token. Chạy
    `curl -s -H "Authorization: Bearer <token>" http://127.0.0.1:51525/v1/profiles` ra danh sách profile; thiếu
    token ra 401. Theo `docs/local-api.md` mở "Thử trực tiếp 1" với `devtools`, rồi đóng. Tắt API thì
    `lsof -nP -iTCP:51525 -sTCP:LISTEN` không còn gì.
23. **Màn hình chào.** Thoát app rồi mở với một thư mục trống:
    `open -n build/bin/AnyProfile.app --env ANYPROFILE_DATA_DIR=/tmp/ap-fresh`. Phải thấy hướng dẫn 3 bước. Xoá `/tmp/ap-fresh` sau khi xong.

## Phần B: test với tài khoản thật

Làm trên bộ **thật**. Thoát app mẫu trước, rồi mở `build/bin/AnyProfile.app` bình thường (bấm đúp).

### Nguyên tắc an toàn

- Dùng **tài khoản phụ** cho lần thử đầu, không dùng kênh chính.
- Một tài khoản, một profile, một proxy sticky. Không dùng chung proxy giữa các tài khoản.
- Dùng proxy **dân cư** (residential) hoặc 4G. Proxy datacenter (ví dụ `dc.decodo.com`, IP của công ty
  hosting) bị eBay và Reddit chặn ngay ở mức IP: eBay hiện "Something went wrong on our end", Reddit hiện
  "You've been blocked by network security". Đo ngày 16/10/2026, Chrome thật qua cùng proxy cũng bị chặn y
  hệt, nên đó không phải lỗi của app. Reddit còn hỏi "Prove your humanity" với mọi trình duyệt mới chưa có
  cookie, kể cả Chrome thật trên mạng nhà; bấm qua một lần là xong.
- Với DataImpulse, dùng port sticky **10000 trở lên**. Port 823 đổi IP mỗi lần kết nối; app sẽ từ chối.
- Không đổi quốc gia proxy của một profile đã đăng nhập. Muốn đổi thì tạo profile mới.
- Không đăng nhập cùng một tài khoản trên cả Fury và Apostate.
- File cookie xuất ra cho phép người khác đăng nhập tài khoản. Giữ kín và xoá sau khi dùng.

### Trước khi bắt đầu

- Lần mở đầu tiên của bản mới nâng cấp cơ sở dữ liệu (thêm nhiều nền tảng, tài khoản theo nền tảng,
  extension). App tự chép bản cũ vào `~/Library/Application Support/AnyProfile/backups/anyprofile-v11-….db`
  trước khi nâng cấp. Mở xong, profile cũ phải còn đủ, tài khoản cũ hiện dưới nền tảng đầu tiên của profile.
- Chuẩn bị: 1–2 proxy sticky có user/pass, 1 tài khoản phụ (ví dụ TikTok) và tài khoản Gmail phụ của
  cùng "người" đó, file bookmark xuất từ Chrome, một extension nhẹ (ví dụ thư mục đã giải nén).

### Các bước

**Ngày 1: tạo và đăng nhập**

1. **Thêm proxy.** Trang Proxies → Import, dán một dòng `host:port:user:pass` (port sticky). Mật khẩu đi
   thẳng vào Keychain. Sau khi kiểm, proxy hiện quốc gia và IP ra.
2. **Tạo profile.** New profile: đặt tên, Add platform → chọn TikTok và Gmail → Done, chọn proxy vừa thêm.
   Ô Start page có hai dòng.
3. **Mở lần đầu.** macOS có thể hỏi quyền với mục "Chromium Safe Storage": chọn **Always Allow**. Nếu chỉ
   chọn Allow, lần mở sau sẽ mất đăng nhập. Cửa sổ mở hai tab TikTok Studio và Gmail.
4. **Kiểm khi đang mở.** Ngăn chi tiết: nhóm Checks báo tất cả đạt; tab Sessions có một hàng với IP ra đúng
   quốc gia proxy. Mở thêm tab `browserscan.net`: IP là IP proxy, múi giờ khớp IP, không có "Canvas
   Tampering"; danh sách trang kiểm đầy đủ ở `docs/browser-fingerprint-testing.md`.
5. **Đăng nhập** hai tài khoản phụ như bình thường. Tab Account có hai khối TikTok và Gmail: lưu
   email, mật khẩu, khoá 2FA cho từng khối. Menu ⋯ của hàng có "Copy password · TikTok" và
   "Copy password · Gmail"; mã 2FA trùng Google Authenticator; clipboard tự xoá sau 90 giây.
6. **Đóng cửa sổ trình duyệt bằng nút đỏ** (không bấm Close trong app): profile tự đóng trong app sau vài
   giây. Mở lại từ app: phải vẫn đăng nhập cả hai, và cửa sổ mở đúng các trang trong Start page.
7. **Extension.** Settings → Extensions → thêm extension, bật cho profile này trong ngăn chi tiết, đóng rồi
   mở profile: `chrome://extensions` có extension đó. Profile khác không có.
8. **Bookmark.** Đóng profile, menu ⋯ → Import bookmarks… chọn file xuất từ Chrome. Mở lại: thanh
   bookmark có thư mục "Imported".
9. **API (không bắt buộc).** Settings → Local API → On, Copy token. Trong Terminal:
   `curl -s -H "Authorization: Bearer <token>" http://127.0.0.1:51525/v1/profiles` ra danh sách profile.
   Tắt API xong: `lsof -nP -iTCP:51525 -sTCP:LISTEN` không ra gì.
10. **Ảnh chụp.** Nếu lần mở trước kéo dài hơn 2 phút, tab Snapshots có ảnh chụp "close". Chụp thêm một
    ảnh bằng tay.

**Ngày 2: qua đêm**

11. Để profile đóng đến hôm sau rồi mở lại. Phải vẫn đăng nhập cả hai tài khoản, cùng IP (proxy sticky),
    cột Status không đỏ, thanh trạng thái không báo "need attention" cho profile này.
12. **Chuyển phiên (không bắt buộc, chỉ với tài khoản phụ).** Khi profile đang mở, Export cookies. Tạo
    profile mới với proxy **cùng quốc gia**, Import file đó, mở profile mới: tài khoản đã đăng nhập sẵn.
    Nền tảng có thể đòi xác minh lại vì máy ảo khác; đó là hành vi bình thường. Xoá file cookie sau khi xong.

**Trong vài ngày tiếp theo**

13. Dùng profile như bình thường (đăng bài, xem, nhắn tin). Đạt khi không tài khoản nào bị đăng xuất bất
    thường, bị khoá, hay bị đòi xác minh lặp lại.

Hai bước kiểm còn mở của plan nằm trong các bước trên: bước 4 hoàn tất phase 9, bước 10 hoàn tất phase 11.

### Báo lại khi gặp lỗi

Ghi lại profile nào, bước nào, chữ trên toast hoặc cột Status, và chụp đúng vùng bị lỗi.
Nhật ký nằm ở `~/Library/Application Support/AnyProfile/logs/anyprofile.log` (không chứa mật khẩu).
