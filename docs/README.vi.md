# Locale Studio

Ứng dụng Windows giúp quản lý thư viện game/app và khởi chạy với locale riêng.

**Tác giả: Dat Diep** · [English](../README.md)

## Tính năng

- Chạy ứng dụng native x86 và x64, tự chọn engine phù hợp.
- Tự lưu ứng dụng đã chạy vào thư viện; có tìm kiếm và sắp xếp gần nhất.
- Mở lại từ thư viện mà không cần chọn file mỗi lần.
- Giao diện VI/EN, lưu lựa chọn ngôn ngữ trên máy.
- Xử lý một số API đường dẫn ANSI qua các API Unicode của Windows.
- Không đổi system locale hoặc sửa EXE gốc.

Profile giả lập hiện có là **ja-JP / CP932 / LCID 0x0411**. Chưa có các profile locale khác. Ngôn ngữ giao diện VI/EN độc lập với locale giả lập của ứng dụng đích.

## Build

Cần Windows x64, Go 1.25+, Wails CLI v2.14.0, Node.js 22.12+ và compiler C tương thích MinGW cho cả x64/x86. WebView2 Runtime cần thiết để chạy giao diện.

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
./scripts/build.ps1 -CC x86_64-w64-mingw32-gcc -CC32 i686-w64-mingw32-gcc
```

Có thể dùng `x86_64-w64-mingw32-clang` và `i686-w64-mingw32-clang` của LLVM-MinGW, hoặc truyền đường dẫn compiler đầy đủ. Script không tự tải/cài compiler. Engine viết bằng Go; cgo và compiler C cần để tạo DLL.

`build/bin` chỉ chứa bốn file cần phân phối:

- `locale-emulator-go.exe`: giao diện, đây là file người dùng mở.
- `locale-run-x86.exe`: helper 32-bit được gọi tự động.
- `locale-engine.dll`: engine x64.
- `locale-engine-x86.dll`: engine x86.

Giữ bốn file cùng thư mục. Chọn EXE và nhấn **Khởi chạy**. Các công cụ CLI/kiểm thử được tạo riêng trong `build/tests` khi build với `-WithTests`.

## Thư viện và dữ liệu riêng tư

Các lần chạy thành công qua giao diện được lưu vào `%APPDATA%\LocaleStudio\library.json`. Thư viện lưu tên, đường dẫn, kiến trúc, lần chạy gần nhất và số lần chạy. Xóa mục khỏi thư viện không xóa game hay save data.

Ứng dụng được chạy trước khi có tính năng thư viện cần chạy lại một lần để được thêm vào. Lịch sử CLI không tự lưu vào thư viện. Khi EXE bị di chuyển/xóa, mục tương ứng hiện không khả dụng.

Không đưa thư viện cá nhân, log, WebView profile hoặc file game lên repository. Dữ liệu thư viện không được ứng dụng tải lên dịch vụ ngoài.

## Giới hạn

Giao diện chạy trên Windows x64, hỗ trợ EXE native x86/x64. Chưa hỗ trợ ARM64, .NET, hook toàn bộ DLL phụ, delay imports, API lấy động qua `GetProcAddress`, tiến trình con, registry, font/GDI hay múi giờ. Khả năng tương thích tùy ứng dụng; thông báo khởi chạy thành công không bảo đảm toàn bộ game hoạt động đúng. Đường dẫn ANSI vẫn cần biểu diễn được bằng CP932.

## Kiểm thử

```powershell
./scripts/test.ps1 -CC32 i686-w64-mingw32-gcc
npm --prefix frontend run check
```

Native baseline có thể báo `FAIL` khi system locale không phải Nhật; đây là đối chứng. Các lần chạy qua engine phải `PASS`. Chi tiết kiến trúc, các kiểm thử và hướng dẫn chia sẻ nằm trong [README tiếng Anh](../README.md) và [hướng dẫn publish](PUBLISHING.md).

Project sử dụng [MIT License](../LICENSE). Tên tác giả được giữ trong [AUTHORS.md](../AUTHORS.md).

CI tự build ZIP và tạo Release khi push tag phiên bản. Xem [hướng dẫn release](PUBLISHING.md#automated-zip-releases).
