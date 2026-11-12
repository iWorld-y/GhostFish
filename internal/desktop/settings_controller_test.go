package desktop

import (
	"path/filepath"
	"testing"

	cfg "ghostfish/internal/config"
)

func TestSaveSettingsClampsAndRollsBack(t *testing.T) {
	m, _, blocked := fakeShortcutManager()
	defaults := cfg.DefaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := newSettingsFixture(&Controller{st: cfg.State{Opacity: .85, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")})
	a.dialog.Open()
	a.dialog.Min, a.dialog.Max = "40", "60"
	if err := a.dialog.Save(); err != nil {
		t.Fatal(err)
	}
	if a.st.Opacity != .60 || cfg.LoadState(a.statePath, true).Opacity != .60 {
		t.Fatalf("state: %+v", a.st)
	}
	a.dialog.Open()
	a.dialog.Min, a.dialog.Max = "50", "50"
	a.dialog.Draft.Shortcuts[0] = cfg.ShortcutBinding{Key: "X", Command: true}
	blocked[a.dialog.Draft.Shortcuts[0]] = true
	if err := a.dialog.Save(); err == nil {
		t.Fatal("expected shortcut failure")
	}
	if a.st.Opacity != .60 || a.st.Settings.MinOpacity != 40 {
		t.Fatalf("state changed: %+v", a.st)
	}
	delete(blocked, a.dialog.Draft.Shortcuts[0])
	a.statePath = filepath.Join(t.TempDir(), "missing", "state.json")
	if err := a.dialog.Save(); err == nil {
		t.Fatal("expected write failure")
	}
	if a.st.Opacity != .60 || m.bindings != defaults.Shortcuts {
		t.Fatalf("rollback failed: %+v", a.st)
	}
}

func TestSaveSettingsValidationAndRecording(t *testing.T) {
	m, _, _ := fakeShortcutManager()
	defaults := cfg.DefaultSettings()
	m.start(defaults.Shortcuts)
	a := newSettingsFixture(&Controller{st: cfg.State{Opacity: .85, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")})
	a.dialog.Open()
	a.dialog.Min = "20.5"
	if err := a.dialog.Save(); err == nil {
		t.Fatal("expected invalid input")
	}
	a.dialog.Min = "20"
	a.dialog.BeginRecording(0)
	if err := a.dialog.Save(); err == nil {
		t.Fatal("expected recording error")
	}
	a.dialog.EndRecording()
	if len(m.resume([4]bool{})) != 0 {
		t.Fatal("unexpected restore error")
	}
}

func TestEnsureRecoverable(t *testing.T) {
	m := &shortcutManager{registered: [4]bool{true, true, false, true}}
	a := &Controller{st: cfg.State{ClickThrough: true}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json"), restoreError: "恢复快捷键失败：切换穿透", recoveryLost: true}
	a.ensureRecoverable()
	if a.st.ClickThrough {
		t.Fatal("window remained click-through")
	}
	// 启动时某个键就注册失败不算恢复失败，不改写生效的穿透状态。
	b := &Controller{st: cfg.State{ClickThrough: true}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")}
	b.ensureRecoverable()
	if !b.st.ClickThrough {
		t.Fatal("without a restore failure the click-through state should stay")
	}
}

// 自定义范围要同时约束快捷键步进和状态文件。
func TestChangeOpacityUsesConfiguredRange(t *testing.T) {
	config := cfg.DefaultSettings()
	config.MinOpacity, config.MaxOpacity = 40, 60
	a := &Controller{st: cfg.State{Opacity: .55, Settings: &config}, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.changeOpacity(-cfg.OpacityStep)
	if a.st.Opacity != .45 || a.opacity != .45 {
		t.Fatalf("step down: state=%v applied=%v", a.st.Opacity, a.opacity)
	}
	a.changeOpacity(-cfg.OpacityStep)
	a.changeOpacity(-cfg.OpacityStep)
	if a.st.Opacity != .40 || a.opacity != .40 {
		t.Fatalf("lower bound: state=%v applied=%v", a.st.Opacity, a.opacity)
	}
	if got := cfg.LoadState(a.statePath, true).Opacity; got != .40 {
		t.Fatalf("saved opacity = %v, want 0.40", got)
	}
	a.changeOpacity(cfg.OpacityStep)
	a.changeOpacity(cfg.OpacityStep)
	a.changeOpacity(cfg.OpacityStep)
	if a.st.Opacity != .60 {
		t.Fatalf("upper bound: state=%v", a.st.Opacity)
	}

	same := cfg.DefaultSettings()
	same.MinOpacity, same.MaxOpacity = 50, 50
	a.st.Settings, a.st.Opacity = &same, .5
	a.changeOpacity(cfg.OpacityStep)
	a.changeOpacity(-cfg.OpacityStep)
	if a.st.Opacity != .5 {
		t.Fatalf("equal bounds should keep 50%%: state=%v", a.st.Opacity)
	}
}

// 范围含 0 时，步进不再被 20% 的旧下限拦下，[0,0] 也能保存并保持。
func TestZeroOpacityRangeSavesAndSteps(t *testing.T) {
	m, _, _ := fakeShortcutManager()
	defaults := cfg.DefaultSettings()
	if defaults.MinOpacity != 0 || defaults.MaxOpacity != 100 {
		t.Fatalf("默认范围应为 0-100：%+v", defaults)
	}
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := newSettingsFixture(&Controller{st: cfg.State{Opacity: .1, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")})
	a.changeOpacity(-cfg.OpacityStep)
	if a.st.Opacity != 0 || a.opacity != 0 {
		t.Fatalf("降低到 0%%：state=%v applied=%v", a.st.Opacity, a.opacity)
	}
	a.changeOpacity(cfg.OpacityStep)
	if a.st.Opacity != .1 {
		t.Fatalf("从 0%% 提高：state=%v", a.st.Opacity)
	}

	a.dialog.Open()
	a.dialog.Min, a.dialog.Max = "0", "0"
	if err := a.dialog.Save(); err != nil {
		t.Fatal(err)
	}
	if a.st.Settings.MinOpacity != 0 || a.st.Settings.MaxOpacity != 0 || a.st.Opacity != 0 || a.opacity != 0 {
		t.Fatalf("保存 [0,0] 应把当前值转为 0：state=%+v applied=%v", a.st, a.opacity)
	}
	saved := cfg.LoadState(a.statePath, true)
	if saved.Settings == nil || saved.Settings.MinOpacity != 0 || saved.Settings.MaxOpacity != 0 || saved.Opacity != 0 {
		t.Fatalf("saved = %+v", saved)
	}
	a.changeOpacity(cfg.OpacityStep)
	if a.st.Opacity != 0 {
		t.Fatalf("上下限相等时应停在 0：%v", a.st.Opacity)
	}
}

// 保存失败时窗口不透明度保持旧值，成功后才使用新范围限制后的值。
func TestSaveSettingsKeepsWindowOpacityOnFailure(t *testing.T) {
	m, _, blocked := fakeShortcutManager()
	defaults := cfg.DefaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := newSettingsFixture(&Controller{st: cfg.State{Opacity: .85, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")})
	a.setOpacity(.85)
	a.dialog.Open()
	a.dialog.Min, a.dialog.Max = "40", "60"
	a.dialog.Draft.Shortcuts[cfg.ShortcutHidden] = cfg.ShortcutBinding{Key: "J", Command: true}
	blocked[a.dialog.Draft.Shortcuts[cfg.ShortcutHidden]] = true
	if err := a.dialog.Save(); err == nil {
		t.Fatal("expected shortcut registration failure")
	}
	if a.opacity != .85 || a.st.Opacity != .85 || a.st.Settings.MinOpacity != cfg.SettingsMinOpacity {
		t.Fatalf("failure changed the window: applied=%v state=%+v", a.opacity, a.st)
	}
	delete(blocked, a.dialog.Draft.Shortcuts[cfg.ShortcutHidden])
	if err := a.dialog.Save(); err != nil {
		t.Fatal(err)
	}
	if a.opacity != .60 || a.st.Opacity != .60 {
		t.Fatalf("success should clamp to 60%%: applied=%v state=%v", a.opacity, a.st.Opacity)
	}
}

// 启动时就注册不了的快捷键不挡住只改范围的保存，仍标记为不可用。
func TestSaveSettingsWithUnavailableShortcut(t *testing.T) {
	m, _, blocked := fakeShortcutManager()
	defaults := cfg.DefaultSettings()
	blocked[defaults.Shortcuts[cfg.ShortcutThrough]] = true
	if failures := m.start(defaults.Shortcuts); len(failures) != 1 {
		t.Fatalf("failures = %v", failures)
	}
	a := newSettingsFixture(&Controller{st: cfg.State{Opacity: .85, Settings: &defaults}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json")})
	a.setOpacity(.85)
	a.dialog.Open()
	a.dialog.Min, a.dialog.Max = "40", "60"
	if err := a.dialog.Save(); err != nil {
		t.Fatalf("range change should still save: %v", err)
	}
	if m.registered[cfg.ShortcutThrough] {
		t.Fatal("unavailable shortcut should stay unregistered")
	}
	if !m.registered[cfg.ShortcutHidden] {
		t.Fatalf("registered = %v", m.registered)
	}
	saved := cfg.LoadState(a.statePath, true)
	if saved.Settings == nil || saved.Settings.MinOpacity != 40 || saved.Settings.MaxOpacity != 60 || saved.Opacity != .60 {
		t.Fatalf("saved = %+v", saved)
	}
}
