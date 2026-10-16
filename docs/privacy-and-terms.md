# Privacy and terms / Quyền riêng tư và điều khoản

Last updated / Cập nhật: 2026-10-17

## English

### What AnyProfile stores

Everything stays on your computer:

- the database (profiles, proxies without passwords, settings, launch history with exit IPs) and one browser
  folder per profile, under `~/Library/Application Support/AnyProfile` (macOS) or `%APPDATA%\AnyProfile` (Windows);
- proxy passwords, account passwords, 2FA secrets, cookie keys and the local API token, in the macOS Keychain or
  Windows Credential Manager.

There is no AnyProfile account and no AnyProfile server.

### What AnyProfile sends, and where

- **Through each profile's proxy:** what the browser loads, and the exit checks (ipinfo.io, ipwho.is, ipify.org)
  that show the proxy's IP, country and timezone.
- **Directly:** the browser engine download from GitHub Releases on first use, and, when updates are on, one
  request a day to GitHub for `latest.json`. That request carries no identifier beyond what any web request
  carries (your IP address and the app version in the User-Agent).
- **When you click a partner link:** a provider under Proxies → Buy proxies, or an app on the Creator tools page,
  opens in your default browser, not in a profile. Some are referral links, so AnyProfile may earn a commission
  and you pay the same price; PostClaw links carry `utm_source=anyprofile`. The list ships with the app, and
  AnyProfile itself sends nothing about the click.
- **Nothing else.** No analytics, no crash reports, no telemetry. Settings → Help → Copy diagnostics only fills
  your clipboard; you decide where to paste it.

### Terms

- AnyProfile is free software under the GNU AGPL v3 or later, provided **as is, without any warranty**
  (sections 15–16 of the license).
- You are responsible for following the terms of every site and platform you use with it, and the law where you
  live. Many platforms limit how many accounts one person may run; AnyProfile does not change those rules.
- Do not use AnyProfile for fraud, spam, ban evasion, or to access accounts that are not yours.
- Keep backups of what matters. A lost Keychain or Credential Manager entry can sign a profile out of every site.

## Tiếng Việt

### AnyProfile lưu gì

Mọi thứ nằm trên máy của bạn:

- cơ sở dữ liệu (profile, proxy không kèm mật khẩu, cài đặt, lịch sử mở kèm IP ra) và một thư mục trình duyệt cho
  mỗi profile, trong `~/Library/Application Support/AnyProfile` (macOS) hoặc `%APPDATA%\AnyProfile` (Windows);
- mật khẩu proxy, mật khẩu tài khoản, khoá 2FA, khoá cookie và token của API trên máy, trong Keychain của macOS
  hoặc Windows Credential Manager.

Không có tài khoản AnyProfile, không có máy chủ AnyProfile.

### AnyProfile gửi gì, đi đâu

- **Qua proxy của từng profile:** những gì trình duyệt tải, và các lần kiểm IP ra (ipinfo.io, ipwho.is,
  ipify.org) để biết IP, nước và múi giờ của proxy.
- **Đi thẳng:** tải engine trình duyệt từ GitHub Releases ở lần đầu, và khi bật tự kiểm tra cập nhật, mỗi ngày một
  lần hỏi GitHub file `latest.json`. Yêu cầu này không mang định danh nào ngoài những gì mọi yêu cầu web đều có
  (địa chỉ IP của bạn và phiên bản app trong User-Agent).
- **Khi bạn bấm link đối tác:** nhà cung cấp trong Proxy → Mua proxy, hoặc ứng dụng ở trang Công cụ cho creator,
  mở trong trình duyệt mặc định, không mở trong profile. Một số là link giới thiệu, nên AnyProfile có thể nhận hoa
  hồng, giá bạn trả không đổi; link PostClaw mang `utm_source=anyprofile`. Danh sách nằm sẵn trong app, và
  AnyProfile không tự gửi gì về lượt bấm.
- **Ngoài ra không gì cả.** Không thống kê, không báo cáo lỗi tự động. Settings → Help → Copy diagnostics chỉ chép
  vào clipboard; bạn tự quyết dán ở đâu.

### Điều khoản

- AnyProfile là phần mềm tự do theo giấy phép GNU AGPL v3 trở lên, cung cấp **nguyên trạng, không bảo đảm**
  (mục 15–16 của giấy phép).
- Bạn tự chịu trách nhiệm tuân thủ điều khoản của mọi trang và nền tảng bạn dùng cùng app, và pháp luật nơi bạn
  sống. Nhiều nền tảng giới hạn số tài khoản một người được dùng; AnyProfile không thay đổi các quy định đó.
- Không dùng AnyProfile để lừa đảo, spam, lách lệnh cấm, hay truy cập tài khoản không phải của bạn.
- Hãy sao lưu những gì quan trọng. Mất một mục trong Keychain hoặc Credential Manager có thể làm profile bị đăng
  xuất khỏi mọi trang.
