package desktop

import (
	"path/filepath"
	"testing"

	cfg "ghostfish/internal/config"

	"github.com/egoist/mygo/ui"
)

func settingsTestApp(t *testing.T) *settingsFixture {
	t.Helper()
	m, _, _ := fakeShortcutManager()
	defaults := cfg.DefaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := newSettingsFixture(&Controller{st: cfg.State{Opacity: .85, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")})
	a.dialog.Open()
	return a
}

func TestSettingsDialogRecordingAndEscape(t *testing.T) {
	a := settingsTestApp(t)
	tester := ui.NewTester(a.dialog.View, 640, 600)
	if !tester.Focused("不透明度下限") {
		t.Fatal("minimum should receive initial focus")
	}
	if err := tester.Click("切换穿透快捷键"); err != nil {
		t.Fatal(err)
	}
	if a.dialog.Recording() != cfg.ShortcutThrough || !a.keys.suppressed {
		t.Fatal("recording did not start")
	}
	tester.Key(ui.Super|ui.Shift, ui.KeyH)
	if a.dialog.Recording() != -1 || a.dialog.Draft.Shortcuts[cfg.ShortcutThrough].Key != "H" || a.keys.suppressed || a.st.ClickThrough {
		t.Fatal("recording affected window state")
	}
	if err := tester.Click("隐藏 / 显示快捷键"); err != nil {
		t.Fatal(err)
	}
	tester.Key(0, ui.KeyEscape)
	if !a.settingsOpen || a.dialog.Recording() != -1 || a.keys.suppressed {
		t.Fatal("Escape should cancel recording only")
	}
	tester.Key(0, ui.KeyEscape)
	if a.settingsOpen {
		t.Fatal("Escape should close settings")
	}
}

func TestSettingsDialogValidationAndCancel(t *testing.T) {
	a := settingsTestApp(t)
	tester := ui.NewTester(a.dialog.View, 640, 600)
	a.dialog.Min = "101"
	tester.Frame()
	if err := tester.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if !a.settingsOpen || a.dialog.Error() == "" || a.st.Settings.MinOpacity != cfg.SettingsMinOpacity {
		t.Fatal("invalid range was accepted")
	}
	if err := tester.Click("取消"); err != nil {
		t.Fatal(err)
	}
	a.dialog.Open()
	tester.Frame()
	if a.dialog.Min != "0" || a.dialog.Error() != "" {
		t.Fatal("cancelled draft was retained")
	}
}

func TestSettingsDialogShowsSharingRiskAndUnavailableShortcuts(t *testing.T) {
	m, _, blocked := fakeShortcutManager()
	defaults := cfg.DefaultSettings()
	blocked[defaults.Shortcuts[cfg.ShortcutThrough]] = true
	if failures := m.start(defaults.Shortcuts); len(failures) != 1 {
		t.Fatalf("failures = %v", failures)
	}
	a := newSettingsFixture(&Controller{st: cfg.State{Opacity: .85, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")})
	a.dialog.Open()
	tester := ui.NewTester(a.dialog.View, 640, 720)
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
	tester := ui.NewTester(a.dialog.View, 640, 720)
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
	defaults := cfg.DefaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := newSettingsFixture(&Controller{st: cfg.State{Opacity: .85, ClickThrough: true, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")})
	a.setIgnoreMouse(true)
	a.dialog.Open()
	if a.ignoringMouse {
		t.Fatal("settings dialog must be clickable")
	}
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { a.dialog.View(c) })
	}, 640, 720)
	a.dialog.Min = "30"
	tester.Key(0, ui.KeyEscape)
	if a.settingsOpen {
		t.Fatal("Escape should close the settings dialog")
	}
	if a.st.Settings.MinOpacity != cfg.SettingsMinOpacity {
		t.Fatalf("Escape should discard the draft: %+v", a.st.Settings)
	}
	if !a.ignoringMouse || !a.st.ClickThrough {
		t.Fatalf("closing with Escape should restore click-through: ignoringMouse=%v state=%+v", a.ignoringMouse, a.st)
	}
	a.dialog.Open()
	if a.dialog.Min != "0" {
		t.Fatalf("reopening should show the saved range: %q", a.dialog.Min)
	}
}

// 保存回滚失败时界面直接指出失败原因，而不只是在终端里。
func TestSettingsDialogReportsRollbackFailure(t *testing.T) {
	a, _ := rollbackFailureApp(t)
	tester := ui.NewTester(a.dialog.View, 640, 720)
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
	if err := a.dialog.Save(); err == nil {
		t.Fatal("expected rollback failure")
	}
	a.dialog.Close()
	a.dialog.Open()
	tester := ui.NewTester(a.dialog.View, 640, 720)
	if !tester.HasText("恢复快捷键失败") {
		t.Fatalf("reopened settings should explain the forced state: %v", tester.Texts())
	}
	if a.restoreError == "" || !a.restoreFailed() {
		t.Fatalf("restore failure should persist: %q", a.restoreError)
	}
}
