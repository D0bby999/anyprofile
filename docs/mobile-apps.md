# Cài app trên máy mobile

Hướng dẫn này dành cho máy mobile trên **macOS** (máy ảo LineageOS). Máy không có Google Play, nên cài app theo một
trong ba cách dưới đây. Trên Windows (LDPlayer 9), xem mục [Windows](#windows-ldplayer) ở cuối.

Mọi cách đều cần máy **đang chạy**: trên trang Mobile, bấm **Mở** ở dòng của máy, đợi trạng thái thành
**Đang chạy**, rồi bấm **Hiện cửa sổ** nếu chưa thấy cửa sổ điện thoại.

## Chọn cách nào

| Muốn cài | Cách nên dùng |
|---|---|
| App phổ biến trên Google Play (TikTok, Facebook, Instagram, Shopee…) | [Cách 2: Aurora Store](#cách-2-aurora-store-tải-từ-google-play) |
| App mã nguồn mở (trình duyệt, ghi chú, NewPipe…) | [Cách 3: F-Droid](#cách-3-f-droid-app-mã-nguồn-mở) |
| Một file `.apk` bạn đã có trên máy tính | [Cách 1: nút Cài app](#cách-1-nút-cài-app-file-apk-trên-máy-tính) |

Lần đầu dùng máy, cách nhanh nhất là dùng cách 1 để cài **Aurora Store**, rồi cài mọi app khác từ Aurora Store.

## Cách 1: nút Cài app (file APK trên máy tính)

1. Tải file `.apk` về máy tính (nguồn tin được ở [bên dưới](#lấy-file-apk-ở-đâu)).
2. Trên trang **Mobile** của AnyProfile, ở dòng của máy đang chạy, bấm **Cài app**.
3. Chọn một hay nhiều file `.apk`, bấm **Open**.
4. Đợi thông báo **"Đã cài 1 app."** ở góc màn hình. File lớn (vài trăm MB) mất khoảng một phút.
5. App mới hiện trong danh sách app của điện thoại (vuốt lên từ màn hình chính).

Kéo file `.apk` thả vào cửa sổ điện thoại cũng cài được.

Lưu ý:

- **Chỉ cài được file `.apk` đơn.** Gói chia nhỏ (`.apks`, `.xapk`, `.apkm`) thì không. Nhiều app lớn (TikTok,
  YouTube) trên APKMirror chỉ có dạng gói chia nhỏ; khi đó dùng Aurora Store.
- **Chọn bản `arm64-v8a`** (hoặc bản ghi "universal"/"noarch"). Bản chỉ dành cho `x86` hay `x86_64` báo lỗi
  "APK này không dành cho điện thoại ARM".
- Cài lại một app đã có thì app được cập nhật, dữ liệu giữ nguyên.

### Lấy file APK ở đâu

Chỉ tải APK từ nơi bạn tin được: APK sửa đổi có thể chứa mã độc và lấy tài khoản trong máy.

- **Aurora Store:** trang của Aurora trên F-Droid, <https://f-droid.org/packages/com.aurora.store/>, nút
  "Download APK".
- **F-Droid:** <https://f-droid.org/F-Droid.apk>.
- **APKMirror** (<https://www.apkmirror.com/>): bản gốc do chính hãng ký, APKMirror kiểm chữ ký trước khi đăng.
  Chọn bản có nhãn **APK** (không phải BUNDLE) và kiến trúc `arm64-v8a`.

## Cách 2: Aurora Store (tải từ Google Play)

Aurora Store là app mã nguồn mở tải app từ kho Google Play mà không cần tài khoản Google. App tải về là file gốc
của Google Play, kể cả app dạng chia nhỏ.

### Cài và thiết lập Aurora Store (một lần mỗi máy)

1. Tải Aurora Store về máy tính (xem [Lấy file APK ở đâu](#lấy-file-apk-ở-đâu)) và cài bằng **nút Cài app**.
2. Trên điện thoại, mở **Aurora Store**.
3. Màn hình **Welcome**: bấm **Next**.
4. Màn hình quyền: bấm **Finish**. Các quyền ở đây xin sau cũng được.
5. Màn hình đăng nhập: chọn **Anonymous**. Aurora dùng một tài khoản ẩn danh dùng chung; bạn không phải đăng
   nhập Google.
6. Sau vài giây, Aurora hiện kho app (For you, Top charts…).

### Cài một app

1. Trong Aurora Store, bấm **Search**, gõ tên app (ví dụ "TikTok"), chọn đúng app của đúng nhà phát triển.
2. Bấm **Install**.
3. **Lần đầu tiên** Android hỏi quyền cài app từ Aurora:
   - hộp **Installer permission** hay **Permission required**: bấm **Grant** hoặc **Settings**;
   - trang **Install unknown apps**: bật **Allow from this source**;
   - bấm nút **Quay lại** (tam giác ở thanh dưới) để về Aurora.
4. Android hỏi **"Install this app?"**: bấm **Install**.
5. Đợi tải xong. Tốc độ tuỳ mạng và proxy: TikTok (454 MB) mất khoảng 2,5 phút trên mạng gia đình.
6. Nút **Install** đổi thành **Open** là xong.

Cập nhật app: mở Aurora Store → tab **Updates**.

Aurora có lúc báo lỗi đăng nhập ẩn danh hay giới hạn tải, vì tài khoản ẩn danh dùng chung cho nhiều người. Đợi một
lúc rồi thử lại.

## Cách 3: F-Droid (app mã nguồn mở)

1. Tải `F-Droid.apk` (xem [Lấy file APK ở đâu](#lấy-file-apk-ở-đâu)) và cài bằng **nút Cài app**.
2. Mở **F-Droid** trên điện thoại. Lần đầu, F-Droid tải danh sách app trong khoảng nửa phút.
3. Chọn app, bấm **Install**.
4. Lần đầu tiên: **Permission required** → **Settings** → bật **Allow from this source** → **Quay lại**, rồi bấm
   **Install** lần nữa.
5. Android hỏi **"Install this app?"**: bấm **Install**.

## App phổ biến: chạy được không

Kiểm trên máy mobile macOS ngày 23/10/2026, không có proxy:

| App | Kết quả |
|---|---|
| **TikTok** (`com.zhiliaoapp.musically`, cài qua Aurora) | Cài và mở được, tới màn hình điều khoản và đăng nhập. Đăng nhập, lướt và đăng video chưa kiểm |
| **YouTube** (app chính thức) | **Không chạy.** App mở rồi tự đóng ngay, vì cần dịch vụ Google Play mà máy không có |
| **YouTube trên trình duyệt** (`m.youtube.com`) | Chạy, phát được video |
| **DuckDuckGo** (qua F-Droid) | Cài và mở được |

App khác chưa kiểm. Quy tắc chung: app nào **bắt buộc** dịch vụ Google Play (YouTube, Gmail, Google Maps, một số app
ngân hàng và game) sẽ không chạy, hoặc chạy nhưng thiếu tính năng như đăng nhập bằng Google, thông báo đẩy.

### YouTube

App YouTube chính thức không dùng được trên máy này. Thay vào đó:

- mở `m.youtube.com` trong **Trình duyệt** (Browser) của điện thoại, đăng nhập như trên web; hoặc
- dùng **NewPipe** từ F-Droid để xem video. NewPipe không đăng nhập tài khoản YouTube được, và chưa được kiểm trên
  máy này.

Cần app YouTube thật thì dùng profile Web của AnyProfile, hoặc LDPlayer trên Windows (có Google Play).

### Đăng nhập trong app

- **Không chọn "Tiếp tục với Google" / "Continue with Google".** Máy không có dịch vụ Google nên cách này thường
  hỏng. Đăng nhập bằng email, số điện thoại hoặc tên người dùng.
- Gắn **proxy** cho máy **trước** khi đăng nhập lần đầu (sửa máy khi máy đóng). Đăng nhập bằng IP thật rồi mới đổi
  sang proxy thì nền tảng thấy tài khoản đổi IP.
- Ứng dụng vẫn biết đây là máy ảo; xem [Ứng dụng vẫn biết đây là máy ảo](mobile.md#ứng-dụng-vẫn-biết-đây-là-máy-ảo).

## Ngôn ngữ của app mới cài

AnyProfile đặt ngôn ngữ cho từng app mỗi lần **mở máy**, theo quốc gia của proxy. App cài khi máy đang chạy sẽ hiện
tiếng Anh tới lần mở máy sau: **Đóng** rồi **Mở** máy là app nhận ngôn ngữ đúng.

## Gỡ app

Trên điện thoại: giữ biểu tượng app → **App info** → **Uninstall**. Hoặc mở **Settings** → **Apps** → chọn app →
**Uninstall**. App cài qua Aurora hay F-Droid cũng gỡ được ngay trong Aurora hay F-Droid.

## Khi có lỗi

| Thấy | Làm |
|---|---|
| Nút **Cài app** không có | Máy chưa chạy. Bấm **Mở** trước |
| "Chỉ cài được file .apk" | File là gói chia nhỏ (`.apks`, `.xapk`, `.apkm`). Tải bản APK đơn, hoặc cài qua Aurora Store |
| "APK này không dành cho điện thoại ARM" | Tải lại bản `arm64-v8a` |
| Bấm Install trong Aurora hay F-Droid mà không có gì xảy ra | Chưa bật **Allow from this source**: xem bước 3 của [Cài một app](#cài-một-app) |
| App mở rồi tự đóng ngay | App cần dịch vụ Google Play. Dùng bản web của app trong Trình duyệt |
| Aurora báo lỗi đăng nhập ẩn danh | Đợi vài phút rồi mở lại Aurora |
| Tải rất chậm hoặc đứng | Kiểm proxy của máy ở trang Proxy (Check). Proxy chậm thì tải chậm |
| App hiện tiếng Anh | Đóng rồi mở lại máy (xem [Ngôn ngữ của app mới cài](#ngôn-ngữ-của-app-mới-cài)) |

## Windows (LDPlayer)

Máy trên Windows là một instance LDPlayer 9, có sẵn store của LDPlayer. Nút **Cài app** của AnyProfile và kéo
thả file APK vào cửa sổ LDPlayer cũng dùng được. Cách cài trên Windows chưa được kiểm trên máy thật.
