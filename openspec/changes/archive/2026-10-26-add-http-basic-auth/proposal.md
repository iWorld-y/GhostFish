# Proposal

## Why

内部工具（例如 https://luker.iworld.fun/）常挂在 HTTP Basic 认证后面。系统 webview 不会替应用弹登录框，页面于是只停在 401，摸鱼工具在这些站点上等于打不开。认证能力必须由应用自己承担。

## What Changes

- 启动时先用 Go 探测一次目标网址：只有服务器返回 401 且质询方式为 Basic 才进入认证流程，其余情况行为完全不变。
- 凭据按顺序取：环境变量 `GHOSTFISH_AUTH=用户名:密码` → 系统钥匙串 → 终端输入（密码不回显）。
- 凭据先做一次真实请求校验；通过则存进系统钥匙串，同一主机下次免输。
- 校验失败不阻断：仍然把凭据交给窗口尝试，并在终端打印服务器返回的状态码。
- 同一份凭据最多交给窗口两次，之后停手并提示，避免凭据错误时 webview 反复重试。
- 网址里自带 `用户名:密码@主机` 的写法继续有效，走 webview 自己的路径，不经过钥匙串。
- **BREAKING** 无。无认证的站点行为不变。

## Capabilities

### New Capabilities

- `http-auth`: 针对需要 HTTP Basic 认证的站点，负责识别认证需求、取得并校验凭据、把凭据交给窗口，以及认证失败时的收手行为。

### Modified Capabilities

无。既有能力（打开网址、透明度、穿透、快捷键）的行为不变；`browser-window` 里"启动时载入指定网页"的判定不受影响。

## Impact

- 新增 `auth.go`：探测、质询头解析、凭据来源、校验与保存。
- 依赖系统钥匙串，macOS 上是登录钥匙串；在 Linux 上依赖 Secret Service，没有时认证仍可临时使用，只是不记忆。
- 依赖 mygo fork 的认证质询回调（`Page.SetAuthenticationHandler`），上游未合并；`go.mod` 通过 `replace` 指向本地 fork。
- 只接管 Basic。Digest、NTLM 与客户端证书质询仍交给 webview 的默认处理。
- 每次启动多一次探测请求（GET，读到 4 KB 即断开）。
