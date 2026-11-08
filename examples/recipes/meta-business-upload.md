---
name: meta-business-upload
description: Đăng bài viết hoặc Reel/video qua Meta Business Suite (business.facebook.com) lên Facebook Page và Instagram; dùng publish_schedule nếu muốn hẹn giờ
version: "1.0"
arguments:
  - name: profile
    description: profile đang đăng nhập Meta Business Suite (id, tên hoặc #số thứ tự)
    required: true
  - name: file
    description: tên file video hoặc ảnh trong thư mục uploads
    required: true
  - name: caption
    description: nội dung bài viết / mô tả Reel kèm hashtag
    required: true
  - name: destination
    description: page (chỉ Facebook Page), instagram (chỉ Instagram) hoặc both (cả hai, mặc định)
  - name: schedule_at
    description: nếu có, hẹn giờ bằng publish_schedule thay vì đăng ngay, dạng "2026-11-09 09:00" theo giờ máy
---

# Meta Business Suite upload (business.facebook.com)

Meta Business Suite quản lý tập trung Facebook Page và tài khoản Instagram liên kết.
Trình tạo bài viết (Composer) tại `https://business.facebook.com/latest/composer`
cho phép đăng bài, video hoặc Reel lên Facebook Page, Instagram hoặc cả hai cùng lúc.

1. Mở profile: `open_profile` {{profile}}.
2. Nếu {{schedule_at}} có: **dừng** — gọi `publish_schedule` với platform `meta-business`,
   file {{file}}, caption {{caption}}, destination {{destination}}, scheduleAt, rồi báo đã hẹn giờ. Không bấm Đăng tay.
3. `navigate` tới https://business.facebook.com/latest/composer và chờ giao diện soạn bài tải xong.
4. Chọn nơi đăng (Post to):
   - Mặc định là cả Page và Instagram liên kết (nếu có).
   - Nếu {{destination}} là `page`: chỉ tích chọn Facebook Page, bỏ chọn Instagram.
   - Nếu {{destination}} là `instagram`: chỉ tích chọn Instagram, bỏ chọn Page.
   - Nếu {{destination}} là `both`: tích chọn cả hai.
5. Tải tệp lên: bấm nút "Add photo" (nếu là ảnh) hoặc "Add video" / "Upload from desktop"
   (nếu là video/Reel), gọi `upload_file` {{file}} (chỉ file trong thư mục uploads).
6. Nhập nội dung: bấm vào ô văn bản "Text" (hoặc "Write your post..."), `type` {{caption}};
   kiểm tra lại để đảm bảo nội dung đã vào đúng ô.
7. Chờ xử lý media: chờ video xử lý tải lên 100% (nút "Publish" / "Đăng" chuyển sang màu xanh kích hoạt).
8. Bấm **Publish** (Đăng).
9. Chờ thông báo xác nhận thành công ("Your post has been published" hoặc xuất hiện trong Content Planner).
10. Đọc link bài viết nếu có, báo lại trạng thái đã đăng thành công kèm thời gian.
11. Đóng profile nếu bước 1 là do agent mở; nếu người dùng đã mở từ trước thì giữ nguyên.

Lỗi thường gặp:
- Chưa đăng nhập tài khoản: trang chuyển hướng về login (dừng lại và báo người dùng đăng nhập).
- Bấm Publish quá sớm khi video còn đang tải lên.
- File không nằm trong thư mục `uploads/` của AnyProfile.
