---
title: "AnyProfile upgrade: hiệu năng khi nhiều profile, Folders, Templates và Synchronizer"
description: "Lộ trình nâng cấp AnyProfile sau khi đối chiếu code hiện tại: đo baseline hiệu năng, giới hạn tài nguyên, ảo hoá bảng, Folders, Templates theo nền tảng và Synchronizer."
status: in_progress
priority: P1
effort: 17d
tags: [feature, frontend, backend, tech-debt]
blockedBy: []
blocks: []
created: 2026-10-27
---

# AnyProfile upgrade roadmap

## Vì sao

Phân tích nâng cấp ngày 2026-10-27 (hiệu năng, UI, tính năng) ban đầu dựa trên bản review
2026-10-13. Trước khi lập plan này, code hiện tại trên máy (`frontend/src`, `internal/`,
`docs/`, README) đã được đọc lại để đối chiếu: nhiều mục trong phân tích cũ **đã xong** và
không đưa vào plan. Plan chỉ giữ những khoảng trống còn thật ở code hiện tại, cộng thêm
trục hiệu năng khi số profile lớn — thứ người dùng cảm nhận rõ nhất nhưng chưa từng được đo.

## Hiện trạng đã kiểm chứng (2026-10-27)

Đã có, không làm lại:

- Local API (`internal/api`, `api-settings.tsx`, `docs/local-api.md`) và MCP cho AI agent
  (`internal/agent`, `internal/recipe`, `docs/mcp.md`).
- Kiểm tra nhất quán fingerprint mỗi lần mở (plan `261014-0001`, phase 10) — persona
  validator đã có.
- Bookmarks (`internal/bookmarks`), Extensions (`internal/extension`), Snapshot phiên
  (`internal/snapshot`), Backup (`backup-settings.tsx`), TOTP (`internal/totp`).
- Dialog xác nhận (`confirm.tsx`, thay `window.confirm`), Command Palette, Duplicate có
  chọn lọc, Import/Export cookie, Onboarding (`welcome-page.tsx`), `:focus-visible`.
- Trang Mobile (AnyProfile Pro) và bản Windows.

Chưa có, là phạm vi của plan này:

- **Folders**: `sidebar.tsx` chỉ có View `all | running | attention | site: | tag:`;
  không có `folder_id` trong store, không có component folder. Profile chỉ tổ chức được
  theo site và tag.
- **Synchronizer**: không có gói `internal/` hay component nào mirror thao tác sang
  nhiều profile đang mở. Recipes/MCP lo phần agent, thao tác tay đồng bộ thì chưa.
- **Templates hồ sơ**: có site picker và defaults, nhưng chưa có template gồm
  persona + proxy + ngôn ngữ + site để tạo nhanh profile đúng chuẩn từng nền tảng.
- **Hiệu năng chưa đo**: `tools/bench/` có script đo nhưng chưa có baseline được ghi lại
  cho thời gian mở profile, RAM mỗi profile, và thời gian render bảng khi nhiều profile.
  Bảng profile (`profile-table.tsx`) render mọi dòng, chưa ảo hoá.

## Quyết định

- Đo trước, sửa sau: Phase 01 chốt baseline và ngưỡng; mọi phase hiệu năng sau đó so với
  chính baseline này, không tối ưu theo cảm tính.
- Folders theo spec đã soạn 2026-10-16: một profile thuộc tối đa một folder, folder
  phẳng ở MVP, tag giữ nguyên vai trò nhãn ngang, xoá folder không xoá profile.
- Synchronizer chỉ qua pipe DevTools sẵn có (`internal/cdppipe`), không mở thêm port,
  tắt mặc định, chỉ mirror tới các profile người dùng chọn đích danh trong phiên đồng bộ.
- Giữ nguyên các nguyên tắc bảo mật hiện tại: relay `127.0.0.1` giấu mật khẩu proxy,
  fail-closed, không secret trên command line/log/database.
- Plan này lập bằng skill `ak-plan` (AgentKit 2.2.2) ở chế độ mặc định (auto), không
  `--html`, không `--github`. Vì app Muse trên Mac không chạy được terminal nên không
  scaffold qua `ak` CLI được; file được viết trực tiếp đúng format của skill.

## Phase

| # | Phase | Trạng thái | Ước lượng |
|---|---|---|---|
| 1 | Đo baseline hiệu năng: thời gian mở profile, RAM/profile, render bảng, truy vấn SQLite | Một phần: benchmark backend tái lập trên Mac; thiếu RAM/render | 1d |
| 2 | Giới hạn tài nguyên khi mở nhiều profile: trần đồng thời, hàng đợi, trạng thái ở status bar | Một phần: backend + hủy chờ trên bảng; thiếu UI setting/status bar | 2d |
| 3 | Ảo hoá bảng profile và phân trang phía service cho danh sách lớn | Một phần: backend đã kiểm; chưa nối frontend/ảo hóa | 2d |
| 4 | Folders: model, sidebar, lọc, chuyển hàng loạt, xoá an toàn | Một phần: backend và restore đã kiểm; chưa có UI | 4d |
| 5 | Templates hồ sơ theo nền tảng và tạo nhanh từ template | Một phần: backend đã kiểm; chưa có UI | 2d |
| 6 | Synchronizer cho thao tác tay trên nhóm profile đang mở | Chưa làm | 5d |
| 7 | Kiểm thử chấp nhận, tài liệu và CHANGELOG | Đang làm: kiểm hồi quy đã bổ sung; chưa nghiệm thu toàn plan | 1d |

## Sửa sau review (2026-10-27)

Báo cáo hiện hành: [review-fixes-2026-10-27.md](reports/review-fixes-2026-10-27.md).
Các Cook note trước đó là lịch sử, không thay thế trạng thái/bằng chứng trong báo cáo này.
Chưa đóng phase nào khi còn thiếu tiêu chí UI, đo RAM/render hoặc nghiệm thu đầy đủ.

## Tiêu chí

- Baseline ở Phase 01 được ghi thành file trong thư mục plan này, có số đo lặp lại được
  bằng `tools/bench/` trên máy hiện tại.
- Mở 10 profile cùng lúc không vượt trần đã đặt; profile vượt trần vào hàng đợi và giao
  diện nói rõ đang chờ vì sao.
- Bảng 500 profile (dữ liệu giả lập) cuộn mượt, tìm kiếm không render lại toàn bảng.
- Folders đạt đủ tiêu chí nghiệm thu trong `phase-04-folders.md`, gồm xoá folder có
  profile mà không mất profile nào.
- Synchronizer: thao tác ở profile chính xuất hiện ở mọi profile đích trong phiên với
  độ trễ cảm nhận được là tức thời; dừng đồng bộ là dừng hẳn, không mirror rò rỉ sang
  phiên sau.
- `make test` và `make lint` đạt sau mỗi phase; `git status --short` rỗng khi phase đóng.

## Không làm

- Không làm lại Local API, MCP, persona validator, backup, TOTP — đã có.
- Không folder con, không rule tự xếp folder, không cấu hình theo folder ở MVP.
- Không cloud sync, không team/đa người dùng — ngoài phạm vi đợt này.
- Không mở thêm control port cho Synchronizer; không mirror tới profile không được
  chọn đích danh.
- Không copy Free Proxy / Proxy Store của đối thủ.
