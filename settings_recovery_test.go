package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 设置界面和快捷键录入期间必须能用鼠标操作，否则一旦窗口失焦就没有恢复入口。
func TestRecordingKeepsWindowOperable(t *testing.T) {
	m, active, _ := fakeShortcutManager()
	defaults := defaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := &app{st: state{Opacity: .85, ClickThrough: true, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.setIgnoreMouse(true)
	a.openSettings()
	if a.ignoringMouse || !a.st.ClickThrough {
		t.Fatalf("打开设置后窗口应可点、穿透状态不变：ignoringMouse=%v state=%+v", a.ignoringMouse, a.st)
	}
	a.beginRecording(shortcutThrough)
	if len(active) != 0 || !m.suppressed {
		t.Fatalf("录入期间应注销全局快捷键：active=%v", active)
	}
	if a.ignoringMouse {
		t.Fatal("录入期间窗口必须接收鼠标")
	}
	a.endRecording()
	if a.restoreError != "" {
		t.Fatalf("unexpected restore error: %s", a.restoreError)
	}
	if !m.registered[shortcutThrough] {
		t.Fatal("录入门槛结束后应恢复穿透快捷键")
	}
	if a.ignoringMouse {
		t.Fatal("设置界面仍打开时窗口不能变回穿透")
	}
	a.closeSettings()
	if !a.ignoringMouse || !a.st.ClickThrough {
		t.Fatalf("关闭设置后应按生效状态恢复穿透：ignoringMouse=%v state=%+v", a.ignoringMouse, a.st)
	}
}

// 录入结束后恢复旧快捷键失败时，错误要留着，并且窗口必须保持可操作。
func TestRestoreFailureSurvivesNextRecording(t *testing.T) {
	m, _, blocked := fakeShortcutManager()
	defaults := defaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := &app{st: state{Opacity: .85, ClickThrough: true, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.setIgnoreMouse(true)
	a.openSettings()
	a.beginRecording(shortcutHidden)
	blocked[defaults.Shortcuts[shortcutHidden]] = true
	a.endRecording()
	if a.restoreError == "" || m.registered[shortcutHidden] {
		t.Fatalf("应记录恢复失败：error=%q registered=%v", a.restoreError, m.registered[shortcutHidden])
	}
	if a.ignoringMouse || a.st.ClickThrough {
		t.Fatalf("隐藏快捷键缺失时窗口必须可见、不穿透：ignoringMouse=%v state=%+v", a.ignoringMouse, a.st)
	}
	restoreError := a.restoreError
	a.beginRecording(shortcutDown)
	if a.restoreError != restoreError {
		t.Fatalf("下一次录入清空了恢复失败：%q", a.restoreError)
	}
	a.endRecording()
	if a.restoreError != restoreError {
		t.Fatalf("恢复失败没有被保留：%q", a.restoreError)
	}
}

// 穿透快捷键没能恢复时，窗口不能留在穿透状态。
func TestFailedThroughRestoreDisablesClickThrough(t *testing.T) {
	m, _, blocked := fakeShortcutManager()
	defaults := defaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := &app{st: state{Opacity: .85, ClickThrough: true, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.showWindow()
	a.setIgnoreMouse(true)
	a.openSettings()
	a.beginRecording(shortcutDown)
	blocked[defaults.Shortcuts[shortcutThrough]] = true
	a.endRecording()
	if m.registered[shortcutThrough] {
		t.Fatal("穿透快捷键不应还处于注册状态")
	}
	if a.st.ClickThrough || a.ignoringMouse || !a.windowShown() {
		t.Fatalf("窗口必须可见、可操作：state=%+v ignoringMouse=%v visible=%v", a.st, a.ignoringMouse, a.visible)
	}
	if got := loadState(a.statePath); got.ClickThrough {
		t.Fatal("关闭穿透应写入状态文件")
	}
}

// 隐藏快捷键不可用只影响隐藏，不能隐藏窗口，也不影响穿透。
func TestMissingHiddenShortcutCannotHideWindow(t *testing.T) {
	m := &shortcutManager{registered: [4]bool{true, true, true, false}}
	a := &app{st: state{Opacity: .85}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.showWindow()
	a.toggleHidden()
	if !a.windowShown() {
		t.Fatal("隐藏快捷键不可用时不能隐藏窗口")
	}
}

// 启动时只有隐藏快捷键注册失败、穿透快捷键正常：穿透仍可开关，
// 关闭设置后要按生效的穿透状态回到穿透。
func TestHiddenFailureAtStartupStillAllowsClickThrough(t *testing.T) {
	m, _, blocked := fakeShortcutManager()
	defaults := defaultSettings()
	blocked[defaults.Shortcuts[shortcutHidden]] = true
	if failures := m.start(defaults.Shortcuts); len(failures) != 1 {
		t.Fatalf("failures = %v", failures)
	}
	a := &app{st: state{Opacity: .85, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.showWindow()
	a.setIgnoreMouse(false)
	a.openSettings()
	a.toggleClickThrough()
	if !a.st.ClickThrough || a.ignoringMouse {
		t.Fatalf("设置界面里应能开启穿透：state=%+v ignoringMouse=%v", a.st, a.ignoringMouse)
	}
	a.closeSettings()
	if a.restoreError != "" {
		t.Fatalf("没有恢复失败却记录了错误：%q", a.restoreError)
	}
	if !a.st.ClickThrough || !a.ignoringMouse {
		t.Fatalf("关闭设置后应按生效状态进入穿透：state=%+v ignoringMouse=%v", a.st, a.ignoringMouse)
	}
	if got := loadState(a.statePath); !got.ClickThrough {
		t.Fatal("穿透状态应写入状态文件")
	}
}

// 只有不透明度快捷键没恢复时，恢复入口仍完整，不进入强制不穿透状态。
func TestOpacityRestoreFailureKeepsClickThrough(t *testing.T) {
	m, _, blocked := fakeShortcutManager()
	defaults := defaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := &app{st: state{Opacity: .85, ClickThrough: true, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.showWindow()
	a.setIgnoreMouse(true)
	a.openSettings()
	a.beginRecording(shortcutDown)
	blocked[defaults.Shortcuts[shortcutDown]] = true
	a.endRecording()
	if a.restoreError == "" {
		t.Fatal("应记录恢复失败")
	}
	if a.restoreFailed() || !a.st.ClickThrough {
		t.Fatalf("不透明度键失败不应触发强制状态：lost=%v state=%+v", a.restoreFailed(), a.st)
	}
	a.closeSettings()
	if !a.ignoringMouse {
		t.Fatal("关闭设置后应按生效状态恢复穿透")
	}
}

// 恢复失败后的强制状态期间不开再开穿透，与关闭设置的行为一致。
func TestToggleThroughBlockedAfterRestoreFailure(t *testing.T) {
	m, _, blocked := fakeShortcutManager()
	defaults := defaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := &app{st: state{Opacity: .85, ClickThrough: true, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.showWindow()
	a.setIgnoreMouse(true)
	a.openSettings()
	a.beginRecording(shortcutDown)
	blocked[defaults.Shortcuts[shortcutHidden]] = true
	a.endRecording()
	a.closeSettings()
	if !a.restoreFailed() {
		t.Fatal("expected a restore failure")
	}
	a.toggleClickThrough()
	if a.st.ClickThrough || a.ignoringMouse {
		t.Fatalf("恢复失败期间不能开启穿透：state=%+v ignoringMouse=%v", a.st, a.ignoringMouse)
	}
}

// 只有隐藏快捷键恢复失败时，关闭设置也不能把窗口重新送进穿透。
func TestHiddenRestoreFailureKeepsWindowOutOfClickThrough(t *testing.T) {
	m, _, blocked := fakeShortcutManager()
	defaults := defaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := &app{st: state{Opacity: .85, ClickThrough: true, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.showWindow()
	a.setIgnoreMouse(true)
	a.openSettings()
	a.beginRecording(shortcutDown)
	blocked[defaults.Shortcuts[shortcutHidden]] = true
	a.endRecording()
	a.closeSettings()
	if m.registered[shortcutHidden] {
		t.Fatal("隐藏快捷键不应还处于注册状态")
	}
	if a.ignoringMouse || a.st.ClickThrough || !a.windowShown() {
		t.Fatalf("关闭设置后必须可见、不穿透：state=%+v ignoringMouse=%v visible=%v", a.st, a.ignoringMouse, a.visible)
	}
	if got := loadState(a.statePath); got.ClickThrough {
		t.Fatal("关闭穿透应写入状态文件")
	}
}

// 保存回滚时若恢复不了穿透或隐藏快捷键，也不能把窗口留在穿透状态，
// 并且要在界面和终端都说明实际结果。
func TestSaveRollbackFailureKeepsWindowOutOfClickThrough(t *testing.T) {
	a, m := rollbackFailureApp(t)
	var err error
	output := captureStderr(t, func() { err = a.saveSettings() })
	if err == nil || !strings.Contains(err.Error(), "恢复原快捷键失败") {
		t.Fatalf("保存错误应包含恢复失败：%v", err)
	}
	if !strings.Contains(output, "恢复快捷键失败") {
		t.Fatalf("终端应报告恢复失败：%q", output)
	}
	if !a.restoreFailed() || a.restoreError == "" {
		t.Fatalf("应留下恢复失败提示：lost=%v error=%q", a.restoreFailed(), a.restoreError)
	}
	if m.registered[shortcutHidden] {
		t.Fatal("隐藏快捷键不应还处于注册状态")
	}
	if a.ignoringMouse || a.st.ClickThrough || !a.windowShown() {
		t.Fatalf("回滚失败后必须可见、不穿透：state=%+v ignoringMouse=%v", a.st, a.ignoringMouse)
	}
	if a.st.Settings.MinOpacity != settingsMinOpacity {
		t.Fatalf("settings should keep their previous value: %+v", a.st.Settings)
	}
	if got := loadState(a.statePath); got.ClickThrough {
		t.Fatal("关闭穿透应写入状态文件")
	}
	if err := a.saveSettings(); err == nil {
		t.Fatal("the same unavailable shortcut should still fail")
	}
}

// 先有其他恢复错误、后发生保存回滚丢键时，新原因要追加到旧提示之后。
func TestLaterRollbackAddsToEarlierRestoreFailure(t *testing.T) {
	m, _, blocked := fakeShortcutManager()
	defaults := defaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := &app{st: state{Opacity: .85, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.showWindow()
	a.openSettings()
	a.beginRecording(shortcutDown)
	blocked[defaults.Shortcuts[shortcutDown]] = true
	a.endRecording()
	if !strings.Contains(a.restoreError, "降低不透明度") || a.restoreFailed() {
		t.Fatalf("不透明度失败应只留提示：error=%q lost=%v", a.restoreError, a.restoreFailed())
	}
	earlier := a.restoreError

	a.settingsMin, a.settingsMax = "40", "60"
	newBinding := shortcutBinding{Key: "J", Command: true}
	a.settingsDraft.Shortcuts[shortcutHidden] = newBinding
	blocked[defaults.Shortcuts[shortcutHidden]] = true
	blocked[newBinding] = true
	if err := a.saveSettings(); err == nil {
		t.Fatal("expected rollback failure")
	}
	if !strings.HasPrefix(a.restoreError, earlier) {
		t.Fatalf("旧提示应保留：%q", a.restoreError)
	}
	if !strings.Contains(a.restoreError, "隐藏 / 显示") || !strings.Contains(a.restoreError, recoverableNote) {
		t.Fatalf("新丢失的恢复入口应写进提示：%q", a.restoreError)
	}
	if !a.restoreFailed() {
		t.Fatal("丢失恢复入口后应进入强制状态")
	}

	a.closeSettings()
	a.openSettings()
	tester := ui.NewTester(a.settingsDialog, 640, 720)
	if !tester.HasText("隐藏 / 显示") || !tester.HasText(recoverableNote) {
		t.Fatalf("重新打开设置应看到新的恢复失败：%v", tester.Texts())
	}
}

// rollbackFailureApp 准备一个「新旧隐藏快捷键都注册失败」的保存现场。
func rollbackFailureApp(t *testing.T) (*app, *shortcutManager) {
	t.Helper()
	m, _, blocked := fakeShortcutManager()
	defaults := defaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := &app{st: state{Opacity: .85, ClickThrough: true, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.showWindow()
	a.setIgnoreMouse(true)
	a.openSettings()
	a.settingsMin, a.settingsMax = "40", "60"
	newBinding := shortcutBinding{Key: "J", Command: true}
	a.settingsDraft.Shortcuts[shortcutHidden] = newBinding
	blocked[defaults.Shortcuts[shortcutHidden]] = true
	blocked[newBinding] = true
	return a, m
}

// captureStderr 捕获 fn 期间写到标准错误的文本。
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stderr
	os.Stderr = file
	defer func() { os.Stderr = previous }()
	fn()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// 设置界面打开时按穿透快捷键只改状态，窗口保持可点，关闭后再作用到窗口。
func TestThroughShortcutWhileSettingsOpenKeepsWindowUsable(t *testing.T) {
	m, _, _ := fakeShortcutManager()
	defaults := defaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := &app{st: state{Opacity: .85, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.setIgnoreMouse(false)
	a.openSettings()
	a.toggleClickThrough()
	if !a.st.ClickThrough || a.ignoringMouse {
		t.Fatalf("状态应切换但窗口保持可点：state=%+v ignoringMouse=%v", a.st, a.ignoringMouse)
	}
	a.closeSettings()
	if !a.ignoringMouse {
		t.Fatal("关闭设置后应进入穿透")
	}
}
