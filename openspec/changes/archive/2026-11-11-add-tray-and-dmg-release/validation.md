# 验证记录

## 用户接受说明

2026-11-11，用户在了解本记录中的实际结果和未验证限制后，要求将全部任务勾选并同步归档。该决定不改变下方的测试结果；Intel Mac 真机、Windows 和 Linux 原生图形验收等未执行项目仍未验证。

本次在 Apple 芯片的 macOS 上执行检查，使用 Go 1.27.4。下列结果不代表其他平台的图形验收结果。

## 已完成的检查

- `go test ./...`、`go test -race ./...`、`go vet ./...` 和 `go mod tidy -diff`：通过。托盘测试覆盖创建失败、恢复入口组合、宿主查询超时和失效、旧探测结果、0% 不透明度及设置录入。
- `GHOSTFISH_GUI=1 go test -tags gui -run '^TestTray(NativeSmoke|HideKeepsWebViewState)$' -count=1 -v .`：通过。测试创建真实 macOS 托盘和独立窗口，使用内存网页会话及临时状态文件，程序化调用菜单回调；隐藏并显示后，网页脚本状态仍在。测试没有模拟鼠标点击，也不等同于人工验收。
- `GOOS=darwin GOARCH=arm64`、`GOOS=darwin GOARCH=amd64`、`GOOS=linux GOARCH=amd64` 和 `GOOS=windows GOARCH=amd64`，分别配合 `CGO_ENABLED=0 go build -ldflags '-X main.version=v0.2.0'`：均构建成功。Linux 和 Windows 的 `go test -c` 也成功，但未在这些平台执行测试。
- macOS arm64 程序和经 Rosetta 运行的 macOS amd64 程序都输出 `ghostfish v0.2.0`；`go run . -version` 输出 `ghostfish dev`。Linux 和 Windows 程序的构建信息包含正确架构和版本注入参数，未在对应系统执行版本命令。
- `go run github.com/egoist/mygo/cmd/mygo@v0.8.3 build -platform darwin/universal -o dist` 和 `./script/verify_macos_dmg.sh 0.2.0`：通过。检查覆盖 arm64 和 x86_64、彩色图标、应用标识、最低 macOS 版本、`LSUIElement`、版本输出、临时签名、DMG 校验和只读挂载中的 Applications 链接。
- `go test -run 'TestReleaseMetadata|TestPublishReleaseSafety' ./...`：通过。模拟已有正式 Release、已有草稿、权限错误、网络错误、草稿创建后上传失败和资产缺失，均不会公开不完整发布。
- `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/release.yml`、`bash -n script/*.sh` 和 `openspec validate add-tray-and-dmg-release --strict`：通过。

## 图标与限制

- 用户选定的彩色 P1 和原始单色图分别保留在 `assets/logo/ghostfish-color.png` 与 `assets/logo/ghostfish-monochrome.png`。使用内置 imagegen 工具，以原始单色图为参考，保留浏览器、幽灵和鱼，去除细线并加粗轮廓，生成 `assets/logo/ghostfish-tray.png`。通过像素检查和 AppKit 模板图离屏绘制确认空白区域保持透明。
- MyGo 的 Linux AppIndicator 后端直接使用 PNG，不根据托盘背景改变模板颜色。因此另外使用内置 imagegen 工具，以简化图为参考，生成带白色圆角底板的黑白图 `assets/logo/ghostfish-tray-linux.png`。离屏的 16、24、32 像素浅色和深色背景对照见 `/private/tmp/ghostfish-linux-contrast.png`；原图和简化图的尺寸对照见 `/private/tmp/ghostfish-tray-size-comparison.png`。24 和 32 像素的主体轮廓较清楚，16 像素下鱼和幽灵的细节仍难区分。
- 上述对照是离屏预览，未在真实 Linux 或 Windows 托盘中验证。Linux 桌面仍可能隐藏或不显示已注册图标；宿主和注册状态检查不能证明图标实际呈现，不能把它当作已完成的桌面验收。
- 未在 Finder 中启动打包应用，未验证现有用户数据目录和无终端认证的完整图形流程。未在 Intel Mac 真机启动 DMG，Rosetta 的命令行版本检查不能代替真机验收。
- 尚未运行 GitHub Actions，也未创建、修改或发布 GitHub Release。工作流资产上传和四个平台原生执行结果需要实际运行后确认。
