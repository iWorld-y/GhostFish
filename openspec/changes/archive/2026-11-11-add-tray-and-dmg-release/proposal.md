# Proposal

## Why

GhostFish 目前只能通过全局快捷键恢复穿透或隐藏的窗口，快捷键不可用时缺少独立的操作入口。现有下载说明只列出命令行程序，项目中也缺少受版本管理的 macOS DMG 打包和发布流程。

## What Changes

- 在 macOS、Windows 和 Linux 提供托盘菜单，使用选定的单色图标，支持切换鼠标穿透、隐藏或显示窗口以及退出应用。
- 让托盘、现有快捷键和设置共享窗口状态，并在托盘或快捷键不可用时保留可恢复的窗口状态。
- 支持从 macOS 应用程序启动，使用彩色 P1 作为应用图标；窗口关闭和最后一个标签页关闭时仍退出应用。
- 参照 GhostPin 的发布流程，通过 GitHub Actions 构建和校验通用 macOS DMG，在版本标签触发时发布，同时保留现有四个命令行下载产物。
- 更新安装和使用说明，说明 Linux 托盘依赖、macOS 未公证应用的打开方式及图形启动时的认证限制。

## Capabilities

### New Capabilities

- `release-distribution`：定义命令行产物、macOS 应用和 DMG 的构建、校验及 GitHub Release 发布行为。

### Modified Capabilities

- `control-surface`：增加跨平台托盘入口和状态同步，并按托盘与快捷键的可用性处理恢复。
- `click-through`：允许托盘作为穿透恢复入口，调整启动恢复与不可用时的保护条件。
- `browser-window`：增加通过 macOS 应用程序启动的方式，保留现有命令行启动和关闭行为。

## Impact

- 涉及应用启动、窗口控制、快捷键管理、macOS 设置和相应测试，以及托盘图标资源。
- 引入彩色 P1 和单色 PNG 作为项目资源；现有单色图仍在原工作区且未跟踪，彩色 P1 仍在本会话的生成目录，实施时须复制到变更分支。
- 新增 macOS 打包和 GitHub Actions 发布配置。Linux 托盘依赖系统 AppIndicator 库；未配置 Developer ID 公证时，DMG 保持未公证发布。
- 保留现有不透明度范围、终端认证方式以及 Windows 和 Linux 命令行发行方式，不增加自动更新。
