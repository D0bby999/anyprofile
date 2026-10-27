# Ma trận nghiệm thu nâng cấp

Ma trận này là gate cho **tính năng của roadmap**, không phải phép phát hành public.
Mọi ô R cần kết quả chạy thật; cross-build/vet chỉ chứng minh biên dịch/tĩnh.

## Lane và điều kiện

| Lane | Môi trường | Engine | Tình trạng đã biết |
|---|---|---|---|
| M-A | macOS arm64, M2/16 GiB, native WebView | Apostate 0.5.1 / Chromium 155.0.8059.31 đúng hash | Có binary; integration cũ đạt một phần, UI chưa kiểm |
| M-F | Cùng Mac | Fury 0.4.5 đúng catalog hiện tại | BLOCKED: máy đang có Fury 0.3.5, không thay hash để chạy |
| W-11 | Windows 11 x64, 16 GiB, SSD, native WebView2 | Apostate đúng catalog Windows | Chưa có bằng chứng chạy thực |
| W-10 | Windows 10 x64, 16 GiB, WebView2 còn được hỗ trợ | Apostate đúng catalog Windows | Chưa có bằng chứng compatibility |
| RAM-32 | Mac và Windows thật 32 GiB, ghi CPU/OS | Engine được platform hỗ trợ | Chưa đo; không ngoại suy thành PASS từ máy 16 GiB |

Fury trên Windows là N/A vì không được hỗ trợ, không phải test bị skip để đạt.
Nếu đổi catalog/engine sang AnyBrowser sau này, rerun các lane bị ảnh hưởng; plan
này không tự đổi thương hiệu/ID engine. Không ghim vào binary 156 chưa tích hợp.

## Coverage bắt buộc

R = required, N/A = không áp dụng. Bảng dưới là yêu cầu, **không phải kết quả**.
M-A/M-F dùng chung app UI nhưng phải kiểm các luồng phụ thuộc engine riêng.

| Case | M-A | M-F | W-11 | W-10 | RAM-32 | Phase |
|---|---|---|---|---|---|---|
| ACC-01: schema 17 -> hiện tại, profile/proxy/account/tag cũ nguyên vẹn | R | N/A | R | R | N/A | 4/5 |
| ACC-02: backup mã hóa folder/template, collision tên/ID, rollback/import lặp | R | N/A | R | R | N/A | 4/5 |
| ACC-03: submit 10/trần 2, FIFO/position/dedupe/cancel races/legacy adapter | R | R | R | R | N/A | 2 |
| ACC-04: adopt/PID reuse/browser crash/cancel startup/quit; cleanup 3 lượt | R | R | R | R | N/A | 2 |
| ACC-05: auto/manual/unlimited, migration missing/corrupt/0/positive setting | R | R | R | R | R | 1/2 |
| ACC-06: query parity, snapshot/count/sort/mutation/stale response, 500 profile | R | N/A | R | R | N/A | 3 |
| ACC-07: virtual rows, range/select-all, drag/keyboard/hidden IDs, focus | R | N/A | R | R | N/A | 3 |
| ACC-08: Folders UI tạo/đổi/lọc/bulk/xóa/Trash, templates CRUD/create/bulk | R | R | R | R | N/A | 4/5 |
| ACC-09: PERF-01..08 trên release build/native WebView | R | R* | R | R | R* | 1/2/3 |
| ACC-10: Synchronizer spike + PERF-09 + safety/Stop barrier, 1 chính/2 đích | R | R | R | R | N/A | 6 |
| ACC-11: en/vi, sáng/tối, normal/compact, resize/keyboard/a11y | R | N/A | R | R | N/A | 2..6 |
| ACC-12: build/test/lint/typecheck/race/security regression | R | N/A | R | R | N/A | 7 |

`R*`: M-F bắt buộc đo startup/RAM/queue/sync; UI thuần bảng có thể dùng evidence
M-A khi cùng binary app, fixture và WebView. RAM-32 chỉ gate phép đo/công thức RAM
và trần, không bắt chạy lại mọi benchmark UI. Quy tắc reuse phải ghi cụ thể từng
ô, không biến N/A thành bằng chứng engine đã chạy.

## Dữ liệu và thao tác

- Seed app test 100/300/500 profile, có Unicode/composed/decomposed, timestamps
  trùng, pin, proxy exit IP, custom site khác slug, folders, templates, attention
  và trash. Store unit fixture và UI fixture phải cùng spec và có generator/hash.
- Dữ liệu migration: DB schema 17 có profile/account/proxy/tag thật về cấu trúc
  nhưng không secret thật, cùng bản backup trước migration để so từng field.
- Backup: destination trống và destination có folder cùng tên khác ID/khác case,
  folder cùng ID đã rename; template/profile đích đã có không bị ghi đè. Gây lỗi
  trước commit để kiểm rollback DB/file/vault và database vẫn dùng được.
- Hàng đợi: normal close, crash/kill, startup error, quit và restart có Adopt;
  giữ một profile ngoài fixture để chứng minh không bị stop/kill/mirror nhầm.
- Synchronizer: tổng 3 browser, một tab explicit/browser, local multi-origin
  fixture/iframe/input/IME/sensitive canary; ngoại lệ theo spike phải ghi rõ.
  Tài khoản phụ thật chỉ dùng sau khi local safety gates PASS; không dùng account
  thật để thay test tự động hoặc đưa payload vào screenshot/log.
- Layout chuẩn 1280x900 (table viewport 720 px cho PERF-06), thêm 1024x768 và
  resize nhỏ nhất app hỗ trợ; không tự yêu cầu mobile web nếu native app không
  hỗ trợ. Capture text/attributes trước screenshot element khi cần nhìn.

## Evidence ledger và quy tắc đóng

Tạo `reports/acceptance-results.md` khi thực thi (chưa tồn tại/chưa có kết quả mới).
Mỗi record bắt buộc có `caseId, lane, status, commit, appBuildHash, engineHash,
fixtureHash, os/runtime, command-or-steps, sampleCount, rawArtifactPath+hash,
expected, actual, blockerOwner+nextAction`. Screenshot không thay kiểm logic.

- Status chỉ `PASS | FAIL | BLOCKED | NOT_RUN | N/A`; có lịch sử partial thì ghi
  trong actual, không đổi ô required thành PASS. FAIL/BLOCKED có owner + lệnh/bước
  để unblock, không rerun mù trên cùng binary sai hash.
- PASS cần toàn bộ tiêu chí case, đúng revision/input/môi trường; đổi code liên
  quan thì invalidates evidence đó. Test cũ có thể reuse khi phạm vi khớp hoàn toàn,
  không suy từ một case happy-path ra toàn matrix.
- Phase chỉ completed khi mọi R thuộc nó PASS, artifact truy cập được, không còn
  defect nghiêm trọng hay item chưa kiểm; N/A phải có lý do từ support matrix.
- Gate thất bại: giữ phase mở, ghi bằng chứng, phân công remediation. Không sửa
  tiêu chí/scope để đóng; thay sản phẩm/giảm support cần user duyệt rõ ràng.
- Không tự commit/push để làm `git status` sạch. Khi user yêu cầu commit, chỉ đưa
  đúng phạm vi; thay đổi không liên quan không phải lý do reset dữ liệu của họ.

## Lệnh đã tồn tại

```sh
GOFLAGS=-p=2 make test
make lint
make build
go test -race ./internal/launcher ./internal/store ./internal/service -count=1
go test ./internal/store -run '^TestProfileListUsesOrderIndex$' \
  -bench '^BenchmarkProfileList$' -benchmem -benchtime=100x -count=1 -v
```

Integration hiện có: `go test -tags integration ./internal/launcher -run
'^TestApostate(LaunchCeilingQueue|AdoptedBrowserCountsTowardCeiling|Lifecycle)$'
-count=3` với `ANYPROFILE_TEST_APOSTATE_APP` và `ANYPROFILE_TEST_HEADLESS=1` đúng
platform; Fury `TestLifecycleFiveProfiles` cần binary đúng 0.4.5. Chỉ các case
này đã tồn tại; test contract mới cần được viết ở phase sở hữu, không giả tên test
mới là command đã runnable.

Windows chạy các Go/FE tests và binary native của Windows trong CI/VM/máy thật;
`make build` trên Mac không chứng minh packaging/runtime Windows đạt. Phase 07
phải lưu lệnh thực tế theo toolchain của runner, không coi target cross-vet là test.

## Bằng chứng lịch sử không thay gate mới

[review-fixes](reports/review-fixes-2026-10-27.md) ghi test/build/race trên Mac và
9 lượt integration cũ đạt; [baseline](reports/baseline.md) mục 5 ghi SQL mean.
Đó là evidence cho backend hiện có, chưa chứng minh async API, toàn bộ snapshot
query contract, p95 render hoặc Synchronizer. M-F còn thiếu binary đúng; các
lane Windows/RAM-32 và UI native chưa được xác nhận trong phiên này.
