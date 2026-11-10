package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func settingsTestApp(t *testing.T) *app {
	t.Helper()
	m, _, _ := fakeShortcutManager()
	defaults := defaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := &app{st: state{Opacity: .85, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.openSettings()
	return a
}

func TestSettingsDialogRecordingAndEscape(t *testing.T) {
	a := settingsTestApp(t)
	tester := ui.NewTester(a.settingsDialog, 640, 600)
	if !tester.Focused("不透明度下限") {
		t.Fatal("minimum should receive initial focus")
	}
	if err := tester.Click("切换穿透快捷键"); err != nil {
		t.Fatal(err)
	}
	if a.recording != shortcutThrough || !a.keys.suppressed {
		t.Fatal("recording did not start")
	}
	tester.Key(ui.Super|ui.Shift, ui.KeyH)
	if a.recording != -1 || a.settingsDraft.Shortcuts[shortcutThrough].Key != "H" || a.keys.suppressed || a.st.ClickThrough {
		t.Fatal("recording affected window state")
	}
	if err := tester.Click("隐藏 / 显示快捷键"); err != nil {
		t.Fatal(err)
	}
	tester.Key(0, ui.KeyEscape)
	if !a.settingsOpen || a.recording != -1 || a.keys.suppressed {
		t.Fatal("Escape should cancel recording only")
	}
	tester.Key(0, ui.KeyEscape)
	if a.settingsOpen {
		t.Fatal("Escape should close settings")
	}
}

func TestSettingsDialogValidationAndCancel(t *testing.T) {
	a := settingsTestApp(t)
	tester := ui.NewTester(a.settingsDialog, 640, 600)
	a.settingsMin = "101"
	tester.Frame()
	if err := tester.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if !a.settingsOpen || a.settingsError == "" || a.st.Settings.MinOpacity != settingsMinOpacity {
		t.Fatal("invalid range was accepted")
	}
	if err := tester.Click("取消"); err != nil {
		t.Fatal(err)
	}
	a.openSettings()
	tester.Frame()
	if a.settingsMin != "0" || a.settingsError != "" {
		t.Fatal("cancelled draft was retained")
	}
}

func TestShortcutFromInput(t *testing.T) {
	binding, ok := shortcutFromInput(ui.InputEvent{Key: ui.KeyT, PhysicalKey: ui.KeyT, Mods: ui.Super | ui.Shift})
	if !ok || binding != (shortcutBinding{Key: "T", Command: true, Shift: true}) {
		t.Fatalf("binding=%+v, ok=%v", binding, ok)
	}
	if _, ok := shortcutFromInput(ui.InputEvent{Key: ui.KeyT, Mods: ui.Shift}); ok {
		t.Fatal("modifier required")
	}
	if _, ok := shortcutFromInput(ui.InputEvent{Key: ui.KeyEscape, Mods: ui.Super}); ok {
		t.Fatal("unsupported key accepted")
	}
}

func TestSettingsDialogShowsSharingRiskAndUnavailableShortcuts(t *testing.T) {
	m, _, blocked := fakeShortcutManager()
	defaults := defaultSettings()
	blocked[defaults.Shortcuts[shortcutThrough]] = true
	if failures := m.start(defaults.Shortcuts); len(failures) != 1 {
		t.Fatalf("failures = %v", failures)
	}
	a := &app{st: state{Opacity: .85, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.openSettings()
	tester := ui.NewTester(a.settingsDialog, 640, 720)
	if !tester.HasText("其他应用") || !tester.HasText("拦截") {
		t.Fatalf("settings should explain shortcut sharing: %v", tester.Texts())
	}
	if !tester.HasText("不可用") {
		t.Fatalf("settings should mark shortcuts that failed to register: %v", tester.Texts())
	}
}

func TestSettingsDialogKeepsRestoreFailureVisible(t *testing.T) {
	a := settingsTestApp(t)
	a.restoreError = "恢复快捷键失败：隐藏 / 显示（⇧ ⌘ H）：快捷键不可用"
	tester := ui.NewTester(a.settingsDialog, 640, 720)
	if !tester.HasText("恢复快捷键失败") {
		t.Fatalf("settings should keep showing the restore failure: %v", tester.Texts())
	}
	if err := tester.Click("降低不透明度快捷键"); err != nil {
		t.Fatal(err)
	}
	if a.restoreError == "" {
		t.Fatal("recording cleared the restore failure")
	}
	tester.Frame()
	if !tester.HasText("恢复快捷键失败") {
		t.Fatal("restore failure disappeared while recording")
	}
}

func TestClosingSettingsWithEscapeRestoresClickThrough(t *testing.T) {
	m, _, _ := fakeShortcutManager()
	defaults := defaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := &app{st: state{Opacity: .85, ClickThrough: true, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.setIgnoreMouse(true)
	a.openSettings()
	if a.ignoringMouse {
		t.Fatal("settings dialog must be clickable")
	}
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { a.settingsDialog(c) })
	}, 640, 720)
	a.settingsMin = "30"
	tester.Key(0, ui.KeyEscape)
	if a.settingsOpen {
		t.Fatal("Escape should close the settings dialog")
	}
	if a.st.Settings.MinOpacity != settingsMinOpacity {
		t.Fatalf("Escape should discard the draft: %+v", a.st.Settings)
	}
	if !a.ignoringMouse || !a.st.ClickThrough {
		t.Fatalf("closing with Escape should restore click-through: ignoringMouse=%v state=%+v", a.ignoringMouse, a.st)
	}
	a.openSettings()
	if a.settingsMin != "0" {
		t.Fatalf("reopening should show the saved range: %q", a.settingsMin)
	}
}

// 保存回滚失败时界面直接指出失败原因，而不只是在终端里。
func TestSettingsDialogReportsRollbackFailure(t *testing.T) {
	a, _ := rollbackFailureApp(t)
	tester := ui.NewTester(a.settingsDialog, 640, 720)
	if err := tester.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if !tester.HasText("恢复原快捷键失败") && !tester.HasText("恢复快捷键失败") {
		t.Fatalf("settings should show the rollback failure: %v", tester.Texts())
	}
	if !a.settingsOpen {
		t.Fatal("settings should stay open after a failed save")
	}
}

// 保存回滚失败后取消再打开设置，强制不穿透的原因仍然可见。
func TestReopenedSettingsExplainRecoveryFailure(t *testing.T) {
	a, _ := rollbackFailureApp(t)
	if err := a.saveSettings(); err == nil {
		t.Fatal("expected rollback failure")
	}
	a.closeSettings()
	a.openSettings()
	tester := ui.NewTester(a.settingsDialog, 640, 720)
	if !tester.HasText("恢复快捷键失败") {
		t.Fatalf("reopened settings should explain the forced state: %v", tester.Texts())
	}
	if a.restoreError == "" || !a.restoreFailed() {
		t.Fatalf("restore failure should persist: %q", a.restoreError)
	}
}

func TestMacTabBarActionsStayVisible(t *testing.T) {
	var a app
	for i := 0; i < 8; i++ {
		tab := a.tabs.add(nil)
		tab.title = "标签页"
	}
	tester := ui.NewTester(a.macTabBar, 360, 40)
	tester.SetTitleBar(ui.TitleBar{Height: 36, Left: 78})
	plus, found := tester.Find("新建标签页")
	if !found || plus.H != 36 || plus.W != 32 {
		t.Fatalf("plus=%+v found=%v", plus, found)
	}
	gear, found := tester.Find("设置")
	if !found || gear.W != 36 || gear.X < plus.X+plus.W || gear.X+gear.W > 360 {
		t.Fatalf("settings=%+v found=%v", gear, found)
	}
}
