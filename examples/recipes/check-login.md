---
name: check-login
description: Kiểm tra một profile còn đăng nhập trên một trang không (không tự đăng nhập)
version: "1"
arguments:
  - name: profile
    description: profile cần kiểm tra (id, tên hoặc #số thứ tự)
    required: true
  - name: site
    description: trang cần kiểm tra, ví dụ https://www.youtube.com
    required: true
---
Goal: tell whether profile {{profile}} is still signed in on {{site}}. Only look; change nothing.

1. Call list_profiles and note whether {{profile}} is open now. Then call open_profile with {{profile}}.
2. Call open_tab with {{site}}. Remember the tab id: you close it at the end.
3. Take a snapshot. Decide:
   - signed in: an account menu, avatar or profile button is there (names like "Account", "Your profile", the
     account's name or picture);
   - signed out: a "Sign in", "Log in" or "Đăng nhập" link or button is there, or the page is a login form;
   - unsure: neither, or the page asks for a check (captcha, "confirm it's you"). Take a screenshot.
4. Never type a password, never press a sign-in button, never solve a captcha. If the page wants any of these,
   the answer is "signed out" or "unsure"; the user handles it.
5. Close the tab you opened. Close the profile only if it was not open in step 1.
6. Answer in one line: signed in, signed out or unsure, and the reason you saw on the page.
