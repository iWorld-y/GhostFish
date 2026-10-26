# GhostFish

摸鱼用的浮动浏览器：一个窗口，浮在别的窗口上面，能调到若隐若现，还能让鼠标点击直接穿过去。只在终端启动，控制全靠全局快捷键。

## 下载

不想自己编译的话，到 [Releases](https://github.com/iWorld-y/GhostFish/releases) 取一个文件即可：

| 文件 | 适用 |
| --- | --- |
| `ghostfish-darwin-arm64` | macOS，Apple 芯片 |
| `ghostfish-darwin-amd64` | macOS，Intel 芯片 |
| `ghostfish-linux-amd64` | Linux，x86_64 |
| `ghostfish-windows-amd64.exe` | Windows，64 位 |

下载来的二进制没有代码签名，也没做公证，macOS 第一次打开会被拦下：右键点「打开」，或者在终端执行 `xattr -dr com.apple.quarantine ./ghostfish-darwin-arm64` 去掉隔离标记。

`./ghostfish -version` 打印版本号；从源码直接构建出来的是 `dev`。

## 跑起来

需要 Go 1.27.3 或更高。桌面框架用的是 fork（比上游多了认证质询回调和快捷键长按连发），本仓库通过 `replace` 指向本地目录，所以先把这个 fork 的分支放在 `~/project/mygo`：

```sh
git clone -b feature/http-auth-challenge git@github.com:iWorld-y/mygo.git ~/project/mygo
```

分支名别省：fork 的 `main` 还是上游原样，缺了认证回调和长按连发，编译会报找不到方法。路径不一样的话改 `go.mod` 里那行 `replace`。

```sh
go build -o ghostfish .
./ghostfish https://example.com
```

- 网址是必填参数，不给就打印用法并以退出码 2 结束。
- 不写协议时自动补 `https://`，只接受 http 和 https。
- 多给几个网址只认第一个。
- 想在终端里直接跑也行：`go run . https://example.com`。

## 快捷键

本版没有菜单栏图标，这四个键就是全部控制面，窗口穿透或隐藏时也只能靠它们。

| 快捷键 | 作用 |
| --- | --- |
| `⌘⇧-` | 不透明度降低 10 个百分点 |
| `⌘⇧=` | 不透明度提高 10 个百分点 |
| `⌘⇧T` | 切换点击穿透 |
| `⌘⇧H` | 隐藏 / 显示窗口 |

不是 macOS 的话，`⌘` 换成 `Ctrl`。

## 需要密码的站点

站点要求 HTTP Basic 认证时（服务器返回 401 加 `WWW-Authenticate: Basic`），启动流程会先自己探一次，再按这个顺序找凭据：

1. 环境变量 `GHOSTFISH_AUTH=用户名:密码`；
2. 系统钥匙串里存过的凭据（macOS 登录钥匙串）；
3. 什么都没有时在终端里问，密码不回显。

凭据会先拿去做一次真实请求验证，通过了才存进钥匙串，之后同一主机不用再输。网址里直接写 `https://用户名:密码@主机/` 也仍然有效，那条路不经过钥匙串。

目前只接管 Basic；Digest、NTLM 和客户端证书交给 webview 自己的默认处理。

## 状态保存在哪

- 不透明度和穿透开关：应用数据目录下的 `state.json`。
- 窗口位置和尺寸：同目录下的 `window-state.json`，由框架负责。

macOS 上是 `~/Library/Application Support/GhostFish/`，Linux 是 `~/.config/GhostFish/`，Windows 是 `%AppData%\GhostFish\`。

不透明度限制在 20%–100%，每次调整 10 个百分点，第一次启动默认 85%。状态文件损坏或取值超出范围时回落到默认值，不会挡住启动。

当前状态会打印在启动它的那个终端里，这是本版唯一的状态显示方式。

## 平台

- macOS 12 及以上：WebKit 自带，无需额外依赖。
- Windows 10/11：需要 WebView2 运行时。
- Linux：需要 GTK 3 和 WebKitGTK 4.1（或 4.0）。

单文件二进制，纯 Go，不需要 cgo。

## 已知限制

- 透明是整窗变淡，字也会跟着淡，40% 以下看内容就费劲了。
- 没有菜单栏图标和托盘：快捷键被别的应用占走时，终端会指出是哪个键不可用，但没有第二条入口。
- 快捷键是全局的，运行期间会占用系统里其他应用的同一组组合键。
- 页面兼容性跟着系统 webview 走，macOS 上就是 Safari 内核，个别站点可能要求 Chromium。
- 关窗口即退出应用。

## 开发

```sh
go test ./...
go vet ./...
```

代码不大，分四块：

- `main.go`：启动流程、窗口创建、状态应用、隐藏/显示。
- `hotkeys.go`：四个全局快捷键的注册与失败处理。
- `state.go`：状态读写、取值范围与损坏回落。
- `target.go`：启动参数里的网址解析。

行为规格在 `openspec/specs/`（五个能力各一份），提案与设计文档归档在 `openspec/changes/archive/`。
