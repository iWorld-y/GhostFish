//go:build darwin && gui

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 使用独立窗口和临时状态文件冒烟检查原生托盘，不注入鼠标或读取用户数据。
func TestTrayNativeSmoke(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("设置 GHOSTFISH_GUI=1 才打开真实窗口")
	}
	// 窗口留到测试进程结束，避免关闭最后一个窗口时主循环提前退出。
	win := mygo.NewWindow(mygo.WindowOptions{Title: "GhostFish tray test", Hidden: true, Width: 320, Height: 200})
	a := &app{win: win, st: state{Opacity: .85}, keys: &shortcutManager{}, statePath: filepath.Join(t.TempDir(), "state.json")}
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
	a.toggleHidden()
	if !win.IsVisible() || a.tray.visible.Label != "隐藏窗口" {
		t.Fatal("快捷键显示后托盘菜单未更新")
	}
	a.toggleHidden()
	a.tray.visible.Click(a.tray.visible, nil)
	if !win.IsVisible() || a.tray.visible.Label != "隐藏窗口" {
		t.Fatal("托盘未显示窗口")
	}
	a.tray.through.Click(a.tray.through, nil)
	if a.st.ClickThrough || a.ignoringMouse || a.tray.through.IsChecked() {
		t.Fatal("托盘未关闭穿透")
	}
}

// 使用内存会话验证隐藏和恢复不会重新创建网页或丢失页面脚本状态。
func TestTrayHideKeepsWebViewState(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("设置 GHOSTFISH_GUI=1 才打开真实窗口")
	}
	a := &app{st: state{Opacity: .85}, trayAvailable: true, keys: &shortcutManager{}, statePath: filepath.Join(t.TempDir(), "state.json")}
	a.win = mygo.NewWindow(mygo.WindowOptions{Title: "GhostFish page test", Hidden: true, Width: 320, Height: 200, Content: ui.View(a.view)})
	view, err := a.win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: mygo.NewEphemeralSession()}})
	if err != nil {
		t.Fatal(err)
	}
	defer view.Destroy()
	tab := a.tabs.add(view)
	ready := make(chan struct{}, 1)
	view.Page().OnDidFinishLoad(func() { ready <- struct{}{} })
	view.Page().LoadHTML("<html><body><p>GhostFish test</p></body></html>", "")
	a.showWindow()
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("测试网页加载超时")
	}
	if _, err := view.Page().Eval("window.ghostfishTest = 42"); err != nil {
		t.Fatal(err)
	}
	a.toggleHidden()
	a.toggleHidden()
	got, err := view.Page().Eval("window.ghostfishTest")
	if err != nil {
		t.Fatal(err)
	}
	if a.tabs.active != tab || tab.view != view || view.IsDestroyed() || fmt.Sprint(got) != "42" {
		t.Fatalf("隐藏后页面状态改变：value=%v, destroyed=%v", got, view.IsDestroyed())
	}
}
