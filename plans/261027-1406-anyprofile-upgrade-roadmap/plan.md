---
title: "AnyProfile upgrade: hiệu năng, Folders, Templates và Synchronizer"
description: "Roadmap có hợp đồng queue/query/sync, ngân sách đo được và gate nghiệm thu đa nền tảng."
status: in_progress
priority: P1
effort: 17d
tags: [feature, frontend, backend, tech-debt]
blockedBy: []
blocks: []
created: 2026-10-27
---

# AnyProfile upgrade roadmap

## Outcome và phạm vi

Làm việc với 500 profile có danh sách đúng và phản hồi tốt; mở browser có trần,
hàng đợi minh bạch/hủy được; tổ chức bằng Folders và tạo nhanh bằng Templates;
đồng bộ thao tác tay chỉ trên nhóm profile/tab đã chọn, có Stop barrier và bảo vệ
input nhạy cảm. Đây là **roadmap tính năng**, không phải checklist public release.

Không làm lại API/MCP, persona validator, bookmarks, extensions, snapshot, backup
mã hóa, TOTP, onboarding và Mobile đã có. Tuy nhiên tính năng mới phải nối đúng
vào các luồng đó: backup/restore/migration không được bị regression.

## Hiện trạng và bằng chứng

Mốc code: `b6f7b78`, sau các commit store/launcher/service/docs `6e9844a`,
`06a4e2e`, `aff2a52`, `b6f7b78`. Trạng thái này khác thời điểm lập plan ban đầu:

- Có backend Folders/Templates, phân trang/index/tìm kiếm bỏ dấu, backup merge
  tổ chức; đã có test trên Mac. UI Folders/Templates chưa có.
- Có FIFO/cancel/adopt/StopAll và nhãn/nút hủy queue trên bảng/drawer. Binding
  mở vẫn đợi ready, chưa có async job/position/setting UI hoàn chỉnh.
- Bảng frontend vẫn tải/render toàn bộ, filter/sort client-side. Store paging
  chưa tương đương mọi filter/sort/selection của UI.
- Có SQL benchmark tái lập trên M2/16 GiB; chưa có browser RAM/startup và WebView
  render p95. Không dùng số mean SQL để tuyên bố đạt UI.
- Chưa có Synchronizer. Fury integration bị chặn vì binary cài 0.3.5 khác catalog
  yêu cầu 0.4.5; Windows native và RAM-32 chưa có bằng chứng.

Bằng chứng lịch sử: [review fixes](reports/review-fixes-2026-10-27.md),
[baseline](reports/baseline.md). Chưa có test mới nào chạy trong lượt chỉnh plan.
Cook notes cũ của các file viết lại được giữ tại
[history](reports/history-before-contract-hardening/README.md), không còn là hợp
đồng thực thi hiện hành.

## Hợp đồng thực thi (nguồn tiêu chí chính)

1. [Hiệu năng và cách đo](contracts/performance.md): PERF-01..09; host/fixture,
   p50/p95/p99, RAM, ngưỡng cụ thể và hạn chế phép đo.
2. [Launch queue](contracts/launch-queue.md): async acceptance, request generation,
   FIFO/position, cancellation race, compatibility, restart và default policy.
3. [Profile list](contracts/profile-list.md): query/sort toàn cục, snapshot/count,
   selection theo ID và reorder không làm mất thứ tự hàng ẩn.
4. [Synchronizer](contracts/synchronizer.md): spike phải pass trước production,
   privacy, tab/frame mapping và Stop barrier, không giả stream input sẵn có.
5. [Acceptance matrix](acceptance-matrix.md): ACC-01..12, lane macOS/Windows,
   data migration/backup và bằng chứng cần có để đóng phase.

Các tên API/snapshot/mode mới trong hợp đồng là thiết kế **chưa triển khai**.
Nội dung hiện hành này thay yêu cầu mơ hồ "mượt/tức thời" và ghi chú cũ trái nhau;
không sửa report lịch sử thành số đo đã đạt các gate mới.

## Phases và dependency

| # | Phase | Trạng thái | Gate trước khi đóng |
|---|---|---|---|
| 1 | [Baseline](phase-01-performance-baseline.md) | Một phần: SQL Mac có; thiếu RAM/render | Measurement contract đủ dữ liệu/lane |
| 2 | [Launch guard](phase-02-launch-resource-guard.md) | Một phần backend + nút hủy | Async API, UI, default đã đo, ACC-03..05 |
| 3 | [Danh sách lớn](phase-03-table-virtualization.md) | Một phần store | Query/selection/reorder contract + PERF-01..06 |
| 4 | [Folders](phase-04-folders.md) | Backend có, UI chưa có | ACC-01/02/08/11 |
| 5 | [Templates](phase-05-profile-templates.md) | Backend có, UI chưa có | Phase 4 folder integration + ACC-02/08/11 |
| 6 | [Synchronizer](phase-06-synchronizer.md) | Chưa làm; spike first | Task 06.0 GO + queue/control ownership + ACC-10 |
| 7 | [Acceptance](phase-07-acceptance-docs.md) | Chưa nghiệm thu toàn plan | Mọi R của matrix PASS |

- Phase 01 không chặn sửa tính đúng đắn queue/store, nhưng chặn tuyên bố default
  RAM-derived và hiệu năng đạt. Không chờ RAM để sửa cleanup process.
- Phase 02 và 03 chia scheduler/lifecycle vs query/selection; cùng sửa
  `app_bindings.go`, `api.ts`, model bindings: phối hợp tuần tự khi chạm cùng file.
- Phase 03 cần folder schema đã có; Phase 04/05 UI phải dùng query/selection contract
  thay vì tự lọc riêng từng trang. Chốt DTO ở Phase 03 trước khi nối sidebar/bulk.
- Phase 06.0 có thể khảo sát bằng primitive hiện có; production 06.1+ phải đợi GO
  và admission/ownership từ Phase 02. Phase 07 là gate cuối, không phải phase vá scope.
- `17d` là ước lượng gốc, không phải số ngày còn lại/cam kết. Spike timebox 0,5–1d;
  re-estimate Phase 02/03/06 sau evidence, ghi chênh lệch trước khi thực thi tiếp.

## Điều kiện hoàn thành

- Có kết quả thực, raw artifacts và lệnh tái lập theo PERF/ACC, đúng build/engine.
- Folders/Templates không mất dữ liệu cũ khi migration/backup/Trash/bulk lỗi.
- UI không đổi tập kết quả/total/order khi chuyển sang paging; shift-range/drag
  qua vùng chưa tải vẫn đúng, không chỉ giảm số DOM rows.
- Queue nhận nhanh, đúng trần kể cả Adopt, hủy/quit không spawn muộn.
- Synchronizer trên 1 chính + 2 đích đạt latency/safety; không input mới sau ack Stop.
- Mọi required lane PASS; missing engine/Windows host là BLOCKED chứ không PASS.
  Không hạ scope/tiêu chí để đóng. Không tự commit/push hay reset worktree của user.

## Không làm và giới hạn phát hành

Không folder con/rule tự phân loại, cloud/team sync, Free Proxy/Proxy Store; không
OS-wide input hook, control port mới, tự mở/auto-join target hoặc ghi secret ra log.
Đổi tên/ID AnyBrowser, publish engine catalog, public release, Pro/runtime deploy,
notarization và phân phối là các công việc riêng; hoàn thành roadmap này không
ngầm chứng minh các gate phát hành đó đạt.

## Validation log — contract hardening 2026-10-27

User đã đồng ý siết plan theo đề xuất review; lượt này chỉ sửa tài liệu. Các quyết
định kỹ thuật mới được ghi tường minh, chưa áp dụng vào production. Đối chiếu lại
filter/sort hiện tại, settings, CDP Call/Subscribe và host Mac; kiểm structural/link
và consistency ở [validation](reports/contract-validation-2026-10-27.md).
Không có auto-start cook, publish, commit hay bằng chứng runtime mới trong lượt
chỉnh plan đó.

### Lưu trữ Git sau yêu cầu `cm`

Theo yêu cầu tiếp theo của user, roadmap này được đưa vào repo chính dưới
`plans/261027-1406-anyprofile-upgrade-roadmap/`. `.gitignore` chỉ mở theo dõi thư
mục này; plan khác và journals vẫn local. Không đặt roadmap trong repo `pro`,
không cần `git add -f` và không tự push.
