package desktop

import (
	"fmt"
	"os"
	"strings"

	cfg "ghostfish/internal/config"
)

// CurrentSettings 返回当前配置，并为旧状态补全默认值。
func (a *Controller) CurrentSettings() cfg.Settings {
	if a.st.Settings == nil {
		defaults := cfg.DefaultSettings()
		a.st.Settings = &defaults
	}
	return *a.st.Settings
}

// SetSettingsOpen 同步设置窗口状态，并应用统一的恢复规则。
func (a *Controller) SetSettingsOpen(open bool) {
	a.settingsOpen = open
	a.ensureRecoverable()
	a.applyClickThrough()
}

// SuspendShortcuts 在录入期间释放全局快捷键。
func (a *Controller) SuspendShortcuts() [4]bool {
	a.applyClickThrough()
	if a.keys == nil {
		return [4]bool{}
	}
	return a.keys.suspend()
}

// ResumeShortcuts 恢复录入前的快捷键，并处理恢复失败。
func (a *Controller) ResumeShortcuts(previous [4]bool) {
	defer a.applyClickThrough()
	if a.keys == nil {
		return
	}
	if failures := a.keys.resume(previous); len(failures) != 0 {
		a.noteRestoreFailure("恢复快捷键失败：" + strings.Join(failures, "；"))
		a.noteRecoveryLost(previous)
		a.ensureRecoverable()
	}
}

// ShortcutAvailable 返回指定动作当前是否可用。
func (a *Controller) ShortcutAvailable(index int) bool { return a.shortcutAvailable(index) }

// RestoreError 返回仍需用户处理的快捷键恢复错误。
func (a *Controller) RestoreError() string { return a.restoreError }

// ApplySettings 注册快捷键并持久化配置，失败时保留原配置并恢复窗口操作入口。
func (a *Controller) ApplySettings(draft cfg.Settings) error {
	if err := cfg.ValidateSettings(draft); err != nil {
		return err
	}
	if a.statePath == "" {
		return fmt.Errorf("无法获取设置保存位置")
	}
	if a.keys == nil {
		return fmt.Errorf("快捷键管理器不可用")
	}
	next := a.st
	next.Settings = &draft
	next.Opacity = draft.ClampOpacity(next.Opacity)
	registeredBefore := a.keys.registered
	notes, err := a.keys.update(draft.Shortcuts, func() error { return cfg.SaveState(a.statePath, next) })
	if err != nil {
		failure := fmt.Errorf("无法保存设置：%w", err)
		fmt.Fprintf(os.Stderr, "ghostfish: %v\n", failure)
		a.noteRecoveryLost(registeredBefore)
		a.ensureRecoverable()
		return failure
	}
	a.st = next
	a.setOpacity(next.Opacity)
	a.restoreError = ""
	a.recoveryLost = false
	a.settingsOpen = false
	a.ensureRecoverable()
	a.applyClickThrough()
	if len(notes) > 0 {
		fmt.Fprintf(os.Stderr, "ghostfish: 设置已保存，但这些快捷键仍未注册成功：%s\n", strings.Join(notes, "；"))
	}
	reportState(a.st)
	return nil
}
