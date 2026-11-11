package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

func TestNotifierHostStatus(t *testing.T) {
	tests := []struct {
		name      string
		kde, free watcherInfo
		want      trayHostStatus
	}{
		{"KDE 宿主可用", watcherInfo{owner: ":1.1", nameKnown: true, known: true, host: true}, watcherInfo{}, trayHostAvailable},
		{"KDE 明确无宿主", watcherInfo{owner: ":1.1", nameKnown: true, known: true}, watcherInfo{owner: ":1.1", nameKnown: true, known: true, host: true}, trayHostUnavailable},
		{"同一服务的 freedesktop 别名", watcherInfo{owner: ":1.1", nameKnown: true}, watcherInfo{owner: ":1.1", nameKnown: true, known: true, host: true}, trayHostAvailable},
		{"不同服务不能代替 KDE", watcherInfo{owner: ":1.1", nameKnown: true}, watcherInfo{owner: ":1.2", nameKnown: true, known: true, host: true}, trayHostUnknown},
		{"KDE 名称无所有者", watcherInfo{nameKnown: true}, watcherInfo{owner: ":1.1", nameKnown: true, known: true, host: true}, trayHostUnavailable},
		{"KDE 属性不可读", watcherInfo{owner: ":1.1", nameKnown: true}, watcherInfo{}, trayHostUnknown},
		{"KDE 名称查询失败", watcherInfo{}, watcherInfo{}, trayHostUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notifierHostStatus(tt.kde, tt.free); got != tt.want {
				t.Fatalf("notifierHostStatus = %v, want %v", got, tt.want)
			}
		})
	}
}

// 宿主存在并不代表本进程的托盘项已经注册。
func TestNotifierItemStatus(t *testing.T) {
	lookup := func(name string) (uint32, error) {
		switch name {
		case ":1.7":
			return 1234, nil
		case "org.kde.StatusNotifierItem-999-1":
			return 999, nil
		default:
			return 0, errors.New("名称不可用")
		}
	}
	for _, tt := range []struct {
		name  string
		items []string
		want  trayHostStatus
	}{
		{"自身托盘项", []string{":1.7/org/ayatana/NotificationItem/mygo"}, trayHostAvailable},
		{"只有其他托盘项", []string{"org.kde.StatusNotifierItem-999-1"}, trayHostUnknown},
		{"没有托盘项", nil, trayHostUnknown},
		{"无法读取进程号", []string{":1.8/StatusNotifierItem"}, trayHostUnknown},
		{"空名称", []string{"/StatusNotifierItem"}, trayHostUnknown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := notifierItemStatus(tt.items, 1234, lookup); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// 宿主查询短暂失败时保留菜单操作证据，持续失败后保护窗口。
func TestUnknownTrayHostAfterMenuAction(t *testing.T) {
	m := &shortcutManager{registered: [4]bool{true, true, false, false}}
	warnings := 0
	a := &app{st: state{Opacity: 0}, keys: m, statePath: filepath.Join(t.TempDir(), "state.json"), trayWarning: func(string) { warnings++ }}
	a.showWindow()
	// 菜单回调确认托盘当前可用。
	a.setTrayAvailable(true)
	a.toggleClickThrough()
	a.setTrayHostStatus(trayHostUnknown)
	if !a.trayAvailable || !a.st.ClickThrough || !a.ignoringMouse || warnings != 0 {
		t.Fatal("未知宿主不应撤销可点击菜单的证据")
	}
	// 单次查询失败不立刻撤销点击证据，持续失败必须保护窗口。
	a.setTrayHostStatus(trayHostUnknown)
	if !a.trayAvailable || !a.st.ClickThrough || warnings != 0 {
		t.Fatal("宽限期内不应撤销托盘可用状态")
	}
	a.setTrayHostStatus(trayHostUnknown)
	if a.trayAvailable || a.st.ClickThrough || a.ignoringMouse || warnings != 1 {
		t.Fatal("持续无法确认宿主后应关闭无法恢复的穿透")
	}
	a.setTrayHostStatus(trayHostAvailable)
	if !a.trayAvailable || warnings != 1 {
		t.Fatal("宿主恢复后应重新允许托盘操作")
	}
}

// 有可用证据或再次操作菜单时应重新计算连续未知次数。
func TestTrayHostUnknownGraceResets(t *testing.T) {
	a := &app{trayAvailable: true, trayWarning: func(string) {}}
	a.setTrayHostStatus(trayHostUnknown)
	a.setTrayHostStatus(trayHostUnknown)
	a.setTrayAvailable(true)
	a.setTrayHostStatus(trayHostUnknown)
	if !a.trayAvailable {
		t.Fatal("实际菜单操作应重置宽限期")
	}
	a.setTrayHostStatus(trayHostAvailable)
	a.setTrayHostStatus(trayHostUnknown)
	a.setTrayHostStatus(trayHostUnknown)
	if !a.trayAvailable {
		t.Fatal("已确认的宿主应重置宽限期")
	}
	a.setTrayHostStatus(trayHostUnavailable)
	if a.trayAvailable {
		t.Fatal("明确没有宿主时应立即判定不可用")
	}
}

func TestTrayStateSyncAndRecovery(t *testing.T) {
	m := &shortcutManager{registered: [4]bool{true, true, false, false}}
	a := &app{
		st: state{Opacity: 0}, keys: m, trayAvailable: true,
		statePath:   filepath.Join(t.TempDir(), "state.json"),
		tray:        &trayControls{through: &mygo.MenuItem{Type: mygo.MenuItemCheckbox}, visible: &mygo.MenuItem{}},
		trayWarning: func(string) {},
	}
	a.showWindow()
	a.toggleClickThrough()
	if !a.st.ClickThrough || !a.ignoringMouse || !a.tray.through.Checked {
		t.Fatalf("穿透状态未同步: %+v", a)
	}
	a.toggleHidden()
	if a.windowShown() || a.tray.visible.Label != "显示窗口" || a.st.Opacity != 0 {
		t.Fatalf("隐藏状态或透明度错误: %+v", a)
	}
	a.setTrayAvailable(false)
	if a.st.ClickThrough || a.ignoringMouse || !a.windowShown() || a.st.Opacity != 0 {
		t.Fatalf("丢失托盘后未恢复窗口: %+v", a)
	}
	if a.tray.through.Checked || a.tray.visible.Label != "隐藏窗口" {
		t.Fatal("托盘菜单未同步恢复后的状态")
	}
	if loadState(a.statePath).ClickThrough {
		t.Fatal("强制关闭的穿透状态未保存")
	}
}

// 宿主消失后分别检查穿透和隐藏动作的恢复入口。
func TestTrayLossProtectsOnlyActionsWithoutShortcut(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		throughKey, hiddenKey bool
		wantThrough, wantShow bool
	}{
		{"两键可用", true, true, true, false},
		{"只有穿透键", true, false, true, true},
		{"只有隐藏键", false, true, false, false},
		{"两键均不可用", false, false, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := &shortcutManager{registered: [4]bool{true, true, tt.throughKey, tt.hiddenKey}}
			a := &app{st: state{Opacity: 0, ClickThrough: true}, keys: m, trayAvailable: true,
				statePath: filepath.Join(t.TempDir(), "state.json"), trayWarning: func(string) {}}
			a.hideWindow()
			a.applyClickThrough()
			a.setTrayAvailable(false)
			if a.st.ClickThrough != tt.wantThrough || a.windowShown() != tt.wantShow || a.st.Opacity != 0 {
				t.Fatalf("恢复结果错误：through=%v visible=%v opacity=%v", a.st.ClickThrough, a.windowShown(), a.st.Opacity)
			}
		})
	}
}

// Windows 和 Linux 的逐动作注册结果与托盘一起决定能否进入穿透或隐藏。
func TestRegisteredShortcutsAndTrayCombinations(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		registered := [4]bool{true, true, mask&1 != 0, mask&2 != 0}
		trayAvailable := mask&4 != 0
		a := &app{st: state{Opacity: .85}, registered: &registered, trayAvailable: trayAvailable,
			statePath: filepath.Join(t.TempDir(), "state.json")}
		a.showWindow()
		a.toggleClickThrough()
		if a.st.ClickThrough != (trayAvailable || registered[shortcutThrough]) {
			t.Fatalf("mask=%d 穿透恢复入口判断错误", mask)
		}
		a.toggleHidden()
		if !a.windowShown() != (trayAvailable || registered[shortcutHidden]) {
			t.Fatalf("mask=%d 隐藏恢复入口判断错误", mask)
		}
	}
}

// 首次探测未确认宿主时，不注册穿透快捷键则必须保存安全状态。
func TestPendingTrayUnavailableProtectsSavedThrough(t *testing.T) {
	a := &app{st: state{Opacity: 0, ClickThrough: true}, keys: &shortcutManager{}, trayPending: true,
		statePath: filepath.Join(t.TempDir(), "state.json"), trayWarning: func(string) {}}
	a.showWindow()
	a.applyClickThrough()
	if a.ignoringMouse {
		t.Fatal("首次探测期间不能启用无法恢复的穿透")
	}
	a.setTrayHostStatus(trayHostUnknown)
	a.setTrayHostStatus(trayHostUnknown)
	a.setTrayHostStatus(trayHostUnknown)
	if a.trayPending || a.st.ClickThrough || a.ignoringMouse || loadState(a.statePath).ClickThrough {
		t.Fatal("无法确认宿主时应关闭并保存穿透状态")
	}
}

func TestTrayAllowsThroughWhileSettingsOpen(t *testing.T) {
	m := &shortcutManager{}
	defaults := defaultSettings()
	a := &app{st: state{Opacity: .85, Settings: &defaults}, keys: m, trayAvailable: true, statePath: filepath.Join(t.TempDir(), "state.json"),
		tray: &trayControls{through: &mygo.MenuItem{Type: mygo.MenuItemCheckbox}, visible: &mygo.MenuItem{}}}
	a.showWindow()
	a.openSettings()
	a.toggleClickThrough()
	if !a.st.ClickThrough || a.ignoringMouse || a.tray.through.Subtitle == "" {
		t.Fatal("设置期间应保存穿透配置并暂停实际穿透")
	}
	a.closeSettings()
	if !a.ignoringMouse || a.tray.through.Subtitle != "" {
		t.Fatal("关闭设置后应恢复穿透")
	}
}

// 构造托盘失败后仍显示窗口，并按各快捷键状态保护已保存的配置。
func TestTrayCreationFailureContinuesStartup(t *testing.T) {
	registered := [4]bool{}
	warning := ""
	a := &app{st: state{Opacity: 0, ClickThrough: true}, registered: &registered,
		statePath: filepath.Join(t.TempDir(), "state.json"), trayWarning: func(message string) { warning = message }}
	trayErr := a.createTray(func(opts mygo.TrayOptions) (*mygo.Tray, error) {
		if runtime.GOOS == "linux" {
			if opts.IconIsTemplate || !bytes.Equal(opts.Icon, trayLinuxIcon) {
				t.Error("Linux 应使用浅色底板图标")
			}
		} else if !opts.IconIsTemplate || !bytes.Equal(opts.Icon, trayIcon) {
			t.Error("macOS 和 Windows 应使用透明模板图标")
		}
		return nil, errors.New("创建失败")
	})
	if trayErr == nil {
		t.Fatal("应返回托盘创建失败")
	}
	a.showStartupWindow(registered)
	a.reportTrayFailure(trayErr)
	if !a.windowShown() || a.st.ClickThrough || a.ignoringMouse || a.st.Opacity != 0 {
		t.Fatalf("托盘失败后窗口状态错误：%+v", a.st)
	}
	if warning == "" || !strings.Contains(warning, "创建失败") || loadState(a.statePath).ClickThrough {
		t.Fatal("应显示失败提示并保存安全的穿透状态")
	}
}

// 录入快捷键时，托盘只更改穿透配置，设置窗口始终可操作。
func TestTrayThroughDuringShortcutRecording(t *testing.T) {
	m, _, _ := fakeShortcutManager()
	defaults := defaultSettings()
	if failures := m.start(defaults.Shortcuts); len(failures) != 0 {
		t.Fatal(failures)
	}
	a := &app{st: state{Opacity: .85, Settings: &defaults}, keys: m, trayAvailable: true,
		statePath: filepath.Join(t.TempDir(), "state.json"),
		tray:      &trayControls{through: &mygo.MenuItem{Type: mygo.MenuItemCheckbox}, visible: &mygo.MenuItem{}}}
	a.showWindow()
	a.openSettings()
	a.beginRecording(shortcutDown)
	a.toggleClickThrough()
	if !a.st.ClickThrough || a.ignoringMouse || a.tray.through.Subtitle == "" {
		t.Fatal("录入期间应保存配置并暂停穿透")
	}
	a.endRecording()
	a.closeSettings()
	if !a.st.ClickThrough || !a.ignoringMouse || a.tray.through.Subtitle != "" {
		t.Fatal("录入结束并关闭设置后应恢复穿透")
	}
}

// 提高不透明度快捷键未注册时，提示仍提供状态文件恢复方法。
func TestTrayWarningExplainsOpacityRecoveryWithoutShortcut(t *testing.T) {
	message := ""
	a := &app{st: state{Opacity: 0}, keys: &shortcutManager{}, trayWarning: func(got string) { message = got }}
	a.warnTrayUnavailable("测试")
	if !strings.Contains(message, "快捷键不可用") || !strings.Contains(message, "修改或重置状态文件") {
		t.Fatalf("缺少快捷键不可用时的不透明度恢复说明：%q", message)
	}
}

func TestGuardRecoveryWithTray(t *testing.T) {
	st := state{Opacity: .85, ClickThrough: true}
	got, notes := guardRecovery([4]bool{}, st, true)
	if !got.ClickThrough || len(notes) != 0 {
		t.Fatalf("托盘可用时应保留穿透: %+v %v", got, notes)
	}
}

func TestDisplayVersion(t *testing.T) {
	for _, tt := range []struct{ build, packaged, want string }{
		{"dev", "", "dev"},
		{"dev", "1.2.3", "v1.2.3"},
		{"dev", "v1.2.3", "v1.2.3"},
		{"v2.0.0", "1.2.3", "v2.0.0"},
	} {
		if got := displayVersion(tt.build, tt.packaged); got != tt.want {
			t.Fatalf("displayVersion(%q, %q) = %q, want %q", tt.build, tt.packaged, got, tt.want)
		}
	}
}
