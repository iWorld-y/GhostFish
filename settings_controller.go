package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// openSettings 将当前生效配置复制为可丢弃的界面草稿。
func (a *app) openSettings() {
	if a.st.Settings == nil {
		defaults := defaultSettings()
		a.st.Settings = &defaults
	}
	a.settingsDraft = *a.st.Settings
	a.settingsMin = strconv.Itoa(a.settingsDraft.MinOpacity)
	a.settingsMax = strconv.Itoa(a.settingsDraft.MaxOpacity)
	a.settingsError = ""
	a.settingsFocus = true
	a.recording = -1
	a.settingsOpen = true
	a.settingsModalOpen = true
	// 设置界面需要鼠标；穿透状态仍在生效配置里，关闭设置后再作用到窗口。
	a.ensureRecoverable()
	a.applyClickThrough()
}

func (a *app) beginRecording(index int) {
	if a.recording >= 0 {
		a.endRecording()
	}
	a.settingsError = ""
	a.recording = index
	a.recordingFocus = true
	// 录入要注销全部全局快捷键；此时窗口必须能接收鼠标，否则失焦后无法取消或关闭设置。
	a.applyClickThrough()
	if a.keys != nil {
		a.suspendedKeys = a.keys.suspend()
	}
}

func (a *app) endRecording() {
	if a.recording < 0 {
		return
	}
	a.recording = -1
	defer a.applyClickThrough()
	if a.keys == nil {
		return
	}
	if failures := a.keys.resume(a.suspendedKeys); len(failures) != 0 {
		// 恢复失败要一直显示到用户重新保存为止，不被下一次录入清空。
		a.noteRestoreFailure("恢复快捷键失败：" + strings.Join(failures, "；"))
		a.noteRecoveryLost(a.suspendedKeys)
		a.ensureRecoverable()
	}
}

// recoverableNote 说明强制可操作状态的原因，和具体哪个键失败一起显示。
const recoverableNote = "窗口已恢复可见、不穿透，请重新保存可用的组合"

// noteRestoreFailure 把一条恢复失败说明追加到界面与终端共用的提示里，
// 相同说明不重复追加；旧提示保留，避免后来的失败盖掉先前的原因。
func (a *app) noteRestoreFailure(note string) {
	if note == "" || strings.Contains(a.restoreError, note) {
		return
	}
	if a.restoreError == "" {
		a.restoreError = note
	} else {
		a.restoreError += "；" + note
	}
	fmt.Fprintln(os.Stderr, "ghostfish:", note)
}

// noteRecoveryLost 在恢复后真的弄丢了穿透或隐藏入口时标记强制状态。
// 只有不透明度之类的快捷键没恢复不算，窗口仍然完全可恢复。
// 这里同时把原因留下：恢复失败可能发生在保存回滚里，或者之前已有别的提示，
// 用户取消设置再打开时要能看到当前丢的是哪个恢复入口。
func (a *app) noteRecoveryLost(registeredBefore [4]bool) {
	if a.keys == nil {
		return
	}
	var lost []string
	for _, i := range []int{shortcutThrough, shortcutHidden} {
		if !registeredBefore[i] || a.keys.registered[i] {
			continue
		}
		a.recoveryLost = true
		if strings.Contains(a.restoreError, shortcutLabels[i]) {
			continue
		}
		lost = append(lost, fmt.Sprintf("%s（%s）", shortcutLabels[i], a.keys.bindings[i].display()))
	}
	if !a.recoveryLost {
		return
	}
	if len(lost) > 0 {
		a.noteRestoreFailure("恢复快捷键失败：" + strings.Join(lost, "；"))
	}
	a.noteRestoreFailure(recoverableNote)
}

// ensureRecoverable 只在录入或保存回滚真的没能恢复旧快捷键时，
// 把窗口拉回可见、不穿透。用户仍可通过可用的入口主动更改状态。
func (a *app) ensureRecoverable() {
	if !a.restoreFailed() || (!a.settingsOpen && a.trayAvailable) {
		return
	}
	if a.st.ClickThrough {
		a.st.ClickThrough = false
		a.save()
	}
	a.applyClickThrough()
	if !a.windowShown() {
		a.showWindow()
	}
}

// restoreFailed 报告穿透或隐藏入口是否在录入或保存回滚中真的丢了。
func (a *app) restoreFailed() bool {
	return a.recoveryLost
}

// canClickThrough 判断托盘或穿透快捷键是否提供恢复入口。
func (a *app) canClickThrough() bool {
	if a.trayAvailable {
		return true
	}
	return a.shortcutAvailable(shortcutThrough)
}

// applyClickThrough 把生效的穿透状态同步到窗口，设置打开时保持窗口可操作。
func (a *app) applyClickThrough() {
	a.setIgnoreMouse(a.st.ClickThrough && !a.settingsOpen && a.canClickThrough())
	a.syncTrayMenu()
}

func (a *app) closeSettings() {
	if a.recording >= 0 {
		a.endRecording()
	}
	a.settingsOpen = false
	a.settingsModalOpen = false
	a.ensureRecoverable()
	a.applyClickThrough()
}

// saveSettings 只有快捷键注册和持久化均成功后才提交草稿和不透明度。
func (a *app) saveSettings() error {
	if a.recording >= 0 {
		return fmt.Errorf("请先完成或取消快捷键录入")
	}
	low, lowErr := strconv.Atoi(strings.TrimSpace(a.settingsMin))
	high, highErr := strconv.Atoi(strings.TrimSpace(a.settingsMax))
	if lowErr != nil || highErr != nil {
		return fmt.Errorf("请输入 0%% 至 100%% 的整数，且下限不能大于上限")
	}
	draft := a.settingsDraft
	draft.MinOpacity, draft.MaxOpacity = low, high
	if err := validateSettings(draft); err != nil {
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
	next.Opacity = draft.clampOpacity(next.Opacity)
	registeredBefore := a.keys.registered
	notes, err := a.keys.update(draft.Shortcuts, func() error { return saveState(a.statePath, next) })
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
	a.settingsModalOpen = false
	a.ensureRecoverable()
	a.applyClickThrough()
	if len(notes) > 0 {
		fmt.Fprintf(os.Stderr, "ghostfish: 设置已保存，但这些快捷键仍未注册成功：%s\n", strings.Join(notes, "；"))
	}
	reportState(a.st)
	return nil
}
