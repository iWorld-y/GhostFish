//go:build darwin && gui

package desktop

import (
	"os"
	"path/filepath"
	"testing"

	cfg "ghostfish/internal/config"

	"github.com/egoist/mygo"
)

// 使用独立窗口和临时状态文件冒烟检查原生托盘，不注入鼠标或读取用户数据。
func TestTrayNativeSmoke(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("设置 GHOSTFISH_GUI=1 才打开真实窗口")
	}
	// 窗口留到测试进程结束，避免关闭最后一个窗口时主循环提前退出。
	win := mygo.NewWindow(mygo.WindowOptions{Title: "GhostFish tray test", Hidden: true, Width: 320, Height: 200})
	a := &Controller{win: win, st: cfg.State{Opacity: .85}, keys: &shortcutManager{}, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.showWindow()
	if err := a.createTray(mygo.NewTray); err != nil {
		t.Fatal(err)
	}
	defer a.tray.icon.Destroy()
	defer a.tray.stop()
	if a.tray.icon == nil || !a.trayAvailable || !win.IsVisible() {
		t.Fatal("托盘或测试窗口未创建")
	}
	a.tray.through.Click(a.tray.through, nil)
	if !a.st.ClickThrough || !a.ignoringMouse || !a.tray.through.IsChecked() {
		t.Fatal("托盘穿透菜单未同步窗口状态")
	}
	// 快捷键和托盘使用相同的控制函数，交替操作应同步菜单。
	a.toggleClickThrough()
	if a.st.ClickThrough || a.tray.through.IsChecked() {
		t.Fatal("快捷键切换后托盘勾选未更新")
	}
	a.tray.through.Click(a.tray.through, nil)
	a.tray.visible.Click(a.tray.visible, nil)
	if win.IsVisible() || a.tray.visible.Label != "显示窗口" {
		t.Fatal("托盘未隐藏窗口")
	}
	a.ToggleHidden()
	if !win.IsVisible() || a.tray.visible.Label != "隐藏窗口" {
		t.Fatal("快捷键显示后托盘菜单未更新")
	}
	a.ToggleHidden()
	a.tray.visible.Click(a.tray.visible, nil)
	if !win.IsVisible() || a.tray.visible.Label != "隐藏窗口" {
		t.Fatal("托盘未显示窗口")
	}
	a.tray.through.Click(a.tray.through, nil)
	if a.st.ClickThrough || a.ignoringMouse || a.tray.through.IsChecked() {
		t.Fatal("托盘未关闭穿透")
	}
}
