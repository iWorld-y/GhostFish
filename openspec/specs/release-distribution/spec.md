# release-distribution Specification

## Purpose

为 GhostFish 的 macOS 安装包和现有命令行程序提供可重复的构建、校验和发布流程，让用户能从同一版本的 GitHub Release 下载适合自己平台的产物。

## Requirements

### Requirement: macOS 安装包包含带图标的通用应用

发布流程 SHALL 生成同时支持 Apple 芯片和 Intel 的 macOS DMG。DMG SHALL 包含可运行的 `GhostFish.app`、Applications 目录入口，并为应用使用用户选择的彩色 P1 图标。应用 SHALL 支持项目现有的 macOS 最低版本。

#### Scenario: 下载并安装 DMG

- **WHEN** macOS 用户下载已发布的 DMG 并打开它
- **THEN** 映像中包含 `GhostFish.app` 和 Applications 目录入口，用户可以将应用复制到 Applications

#### Scenario: 在两种架构上启动

- **WHEN** 用户分别在受支持的 Apple 芯片和 Intel Mac 上启动安装后的应用
- **THEN** 应用无需为另一种架构下载单独的 DMG，并显示应用窗口和托盘图标

#### Scenario: 查看应用图标

- **WHEN** 用户在 Finder 中查看 DMG 内或已安装的应用
- **THEN** 应用图标使用彩色 P1 图案，不使用单色托盘图标

### Requirement: 发布保留现有命令行下载

每个成功发布的版本 SHALL 同时提供现有 macOS、Windows 和 Linux 命令行产物，沿用现有文件名和平台架构，并使版本查询结果与发布版本一致。

#### Scenario: 下载各平台命令行程序

- **WHEN** 用户查看成功发布的 GitHub Release
- **THEN** 除 macOS DMG 外，还能下载 `ghostfish-darwin-arm64`、`ghostfish-darwin-amd64`、`ghostfish-linux-amd64` 和 `ghostfish-windows-amd64.exe`

#### Scenario: 查询构建版本

- **WHEN** 用户对任一发布的命令行程序或 DMG 中的应用可执行文件执行版本查询
- **THEN** 输出的版本与该 Release 的版本一致

### Requirement: 版本标签触发完整校验后发布

版本标签推送 SHALL 触发自动构建和校验；只有标签、受版本管理的版本号及全部必需产物一致且校验通过时，流程 SHALL 创建并发布对应 GitHub Release。已存在的 Release SHALL NOT 被覆盖。

#### Scenario: 标签与版本一致且校验通过

- **WHEN** 推送格式为 `vX.Y.Z` 且与项目版本号一致的标签，所有测试、构建和产物校验均成功
- **THEN** 流程发布对应标签的 Release，并附带一个通用 macOS DMG 和四个现有命令行产物

#### Scenario: 标签与版本不一致

- **WHEN** 推送的版本标签与项目版本号不一致，或以 `v` 开头的标签格式不受支持
- **THEN** 流程失败，不发布新的 Release

#### Scenario: 构建或校验失败

- **WHEN** 任一必需测试、构建或产物校验失败
- **THEN** 流程失败，不发布不完整的 Release

#### Scenario: Release 已存在

- **WHEN** 同一标签的 GitHub Release 已存在
- **THEN** 流程报告冲突并停止，不覆盖已有 Release 或其资产

### Requirement: 手动构建不自动发布

手动触发的发布工作流 SHALL 构建并校验同一组产物，供检查结果，但 SHALL NOT 自动创建或发布 GitHub Release。

#### Scenario: 手动触发工作流

- **WHEN** 用户在 GitHub Actions 手动启动工作流且构建校验成功
- **THEN** 可以从工作流产物下载 DMG 和命令行程序，且不会因此创建 GitHub Release

### Requirement: 说明 macOS 签名与安装限制

未配置 Developer ID 签名及 Apple 公证时，发布说明和安装文档 SHALL 明确说明应用未公证及可能出现的系统拦截，不得宣称应用已公证。

#### Scenario: 使用默认发布配置

- **WHEN** 工作流未提供 Developer ID 和公证配置
- **THEN** 发布产物仍可供下载，安装说明准确描述未公证状态及 macOS 的允许打开步骤
