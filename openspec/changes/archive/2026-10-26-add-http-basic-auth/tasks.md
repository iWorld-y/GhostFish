# Tasks

## 1. 框架侧：认证质询回调

- [x] 1.1 在 mygo fork 的窗口处理器接口里新增认证询问与"是否有处理器"两个方法；验证：`go build ./...` 通过，`go doc` 能看到新接口
- [x] 1.2 macOS 后端实现 `webView:didReceiveAuthenticationChallenge:completionHandler:`，应用给凭据就用、没有处理器就交回默认处理；验证：用本地 401 服务实测，服务器日志出现带 Authorization 的请求
- [x] 1.3 应用拒绝时取消质询，避免缓存凭据被反复重发；验证：错误密码场景请求数从 7809 降到 5 以内
- [x] 1.4 对外暴露 `Page.SetAuthenticationHandler` 与 `AuthenticationRequest`/`Credential` 类型；验证：`go doc github.com/egoist/mygo.Page.SetAuthenticationHandler` 有说明

## 2. 探测与凭据

- [x] 2.1 用 GET 探测目标网址，仅在 401 且质询为 Basic 时进入认证流程；验证：普通站点、401+Digest、带凭据网址三种情形都不触发（单元测试覆盖）
- [x] 2.2 解析 `WWW-Authenticate` 取出认证方式与 realm；验证：单元测试覆盖带 charset、无引号、Digest、空头部
- [x] 2.3 按环境变量、钥匙串、终端输入的顺序取凭据，密码不回显；验证：单元测试覆盖环境变量解析，终端路径手测一次
- [x] 2.4 用凭据请求一次做校验，失败只在终端说明状态码且不阻断；验证：本地服务上正确与错误密码各跑一次，输出符合预期

## 3. 窗口接入与失败收手

- [x] 3.1 先建隐藏窗口、挂好认证处理器再显示并载入网址；验证：本地 401 服务的子资源（CSS/JS/图片）全部带上凭据
- [x] 3.2 同一份凭据最多交给窗口两次，之后停手并提示；验证：错误密码时请求数被限制在 5 次以内
- [x] 3.3 校验通过后把凭据写进系统钥匙串，下次启动免输；验证：第二次启动不再提示输入密码

## 4. 收尾

- [x] 4.1 真实站点验证：https://luker.iworld.fun/ 能正常打开；验证：用户本机实测通过
- [x] 4.2 README 补认证流程、环境变量、钥匙串与 fork 依赖说明；验证：文档内容与实际行为一致
- [x] 4.3 提交 GhostFish 与 fork 的改动，并推送 fork 分支；验证：`git log` 与远端分支均可见

## Workflow follow-up

- 实现与验证完成后用 openspec-archive-change 归档本变更。
- 余下的 3.3 待用户在本机确认钥匙串是否记住凭据后勾选。
