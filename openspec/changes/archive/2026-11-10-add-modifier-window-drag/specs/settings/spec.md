# Spec Delta

## ADDED Requirements

### Requirement: macOS 可以设置窗口拖动修饰键

macOS 设置界面 SHALL 显示当前窗口拖动修饰键组合，并允许用户选择 Command、Control、Option 和 Shift 中的任意一个或多个键。该组合 SHALL 独立于现有四组全局快捷键，默认值为仅 Command；Fn/Globe 不在本次支持范围内。

#### Scenario: 选择单个修饰键

- **WHEN** 用户在设置中只选择 Option 并成功保存
- **THEN** 窗口拖动组合变为 Option，现有四组全局快捷键不变

#### Scenario: 选择多个修饰键

- **WHEN** 用户在设置中选择 Command 和 Shift 并成功保存
- **THEN** 窗口拖动组合变为 Command 加 Shift，两个键同时按下时才能匹配

#### Scenario: 拒绝空组合

- **WHEN** 用户未选择任何修饰键并尝试保存
- **THEN** 设置界面提示必须选择至少一个修饰键，保留未保存的输入，原有生效设置不变

#### Scenario: 取消修改

- **WHEN** 用户修改拖动组合后取消或关闭设置界面
- **THEN** 尚未保存的组合不生效，再次打开设置时仍显示原有组合

#### Scenario: 保存失败

- **WHEN** 用户提交有效的拖动组合，但设置无法应用或持久保存
- **THEN** 界面说明失败并保留输入，原有拖动组合和其他生效设置保持不变

### Requirement: 拖动修饰键设置在重启后恢复

应用 SHALL 保存成功应用的拖动修饰键组合，并在 macOS 下次启动时恢复。配置缺少该项或该项无效时 SHALL 使用默认的 Command，保留能够独立读取的其他设置及状态；无效配置 SHALL 报告回退结果。

#### Scenario: 保存后重启

- **WHEN** 用户保存新的拖动组合并重新启动应用
- **THEN** 设置界面显示保存的组合，窗口拖动使用该组合

#### Scenario: 从旧版本升级

- **WHEN** 应用读取包含原有不透明度、穿透状态或快捷键设置，但没有拖动组合的旧状态文件
- **THEN** 拖动组合使用 Command，原有有效设置和状态保持不变

#### Scenario: 拖动组合损坏

- **WHEN** 已保存的拖动组合为空、类型错误或包含不受支持的键
- **THEN** 拖动组合回退到 Command，其他有效设置和状态保持不变，应用报告回退结果
