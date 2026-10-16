# AnyProfile trên Windows

AnyProfile chạy trên Windows 10/11 x64 với engine Apostate. Profile tạo trên Windows mang fingerprint
Windows, khớp với máy thật: font, GPU và hệ điều hành đều là của máy đó.

## Khác với bản macOS

| | macOS | Windows |
|---|---|---|
| Engine | Apostate, Fury | Chỉ Apostate. Fury đọc cấu hình qua file descriptor phụ, Windows không có |
| Persona | `apostate-macos`, `navigator.platform` = `MacIntel` | `apostate-windows`, `navigator.platform` = `Win32` |
| Mật khẩu, khoá | Keychain | Windows Credential Manager (mã hoá DPAPI theo tài khoản Windows) |
| Cookie của trình duyệt | Khoá "Chromium Safe Storage" trong Keychain | DPAPI, trong `Local State` của profile |
| Mã hoá ổ đĩa | FileVault | BitLocker hoặc Device encryption |
| Dữ liệu | `~/Library/Application Support/AnyProfile` | `%APPDATA%\AnyProfile` |
| Phím tắt | ⌘ | Ctrl |
| Đóng trình duyệt có lưu cookie | SIGHUP | `Browser.close` qua pipe DevTools, hoặc WM_CLOSE |
| Kiểm bản engine | Hash SHA-256 và chữ ký Apple | Hash SHA-256 của `chrome.exe` và `chrome.dll`. Bản Windows của Apostate không có chữ ký số |
| File bổ sung | Không | `155.0.8059.31.manifest` cạnh `chrome.exe`: bản 0.5.1 thiếu nó nên Windows không chạy được (heretic-tech/apostate#2); app tự tạo khi cài |

Như trên macOS, app không mở cổng mạng nào cho trình duyệt: DevTools đi qua hai pipe được chuyền cho
`chrome.exe` bằng `--remote-debugging-io-pipes`.

**Profile gắn với hệ điều hành đã tạo ra nó.** Profile tạo trên Mac không mở được trên Windows, và ngược
lại: persona của hệ điều hành khác sẽ vẽ chữ bằng font của máy này, và trang kiểm tra nhận ra ngay.

## Cài đặt

1. Tải `AnyProfile-amd64-installer.exe`: tab Actions của repo → workflow **windows** → lần chạy mới nhất →
   Artifacts → `anyprofile-windows-x64`.
2. Chạy file cài. Windows có thể báo "Windows protected your PC" vì file chưa có chữ ký số: chọn
   **More info → Run anyway**.
3. App cài cho riêng tài khoản đang dùng, không cần quyền admin, vào `%LOCALAPPDATA%\Programs\AnyProfile`.
   Như trên Mac, màn chào hỏi cài Apostate (khoảng 180 MB).

Gỡ cài đặt không xoá profile: dữ liệu vẫn ở `%APPDATA%\AnyProfile`. Muốn xoá hẳn thì xoá thư mục đó bằng tay.

## Build

Trên một máy Windows, trong Git Bash, cần Go, Node 22, Rust (toolchain MSVC), NSIS và Wails CLI v2.14:

```
make build-windows
```

Kết quả: `build/bin/AnyProfile-amd64-installer.exe`. Workflow `.github/workflows/windows.yml` chạy cùng các
bước đó, sau khi chạy test.

Trên Mac, `make lint` kiểm thêm rằng toàn bộ code, kể cả test, vẫn biên dịch được cho Windows.

## Test

Workflow **windows** chạy trên máy Windows của GitHub:

- toàn bộ unit test Go và frontend;
- vòng ghi, đọc, xoá với Credential Manager thật;
- tải bản Apostate đã ghim và kiểm hash theo đúng cách app cài;
- test tích hợp với trình duyệt thật: mở và đóng, cookie còn sau khi mở lại, fingerprint giữ nguyên, các
  kiểm tra nhất quán đạt, không có cổng TCP nào mở, trình duyệt sống sót khi app đóng pipe, đóng cửa sổ
  cuối thì profile đóng, extension và bookmark, API trên máy.

Lần chạy đầu tiên đạt toàn bộ: 2026-10-17, commit `af804d8`, máy `windows-2022` của GitHub.

Phần chỉ kiểm được bằng tay trên một máy Windows thật:

1. Cài từ file `.exe`, mở app, cài Apostate trong Settings.
2. Thêm proxy, tạo profile, mở: tab Sessions có IP proxy, nhóm Checks đạt hết.
3. Mở `browserscan.net` và `pixelscan.net` trong profile: hệ điều hành là Windows, múi giờ khớp IP.
4. Bấm Open lần nữa khi profile đang mở: cửa sổ trình duyệt hiện lên trước.
5. Thoát app khi profile còn mở, mở lại app: profile vẫn được nhận là đang chạy, bấm Close thì đóng.
6. Đăng nhập một trang, đóng profile, mở lại: vẫn đăng nhập.
7. Settings: dòng BitLocker đúng với máy (bật hay tắt).
