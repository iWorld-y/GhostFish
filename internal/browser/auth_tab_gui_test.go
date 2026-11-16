//go:build darwin && gui

package browser

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"ghostfish/internal/auth"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 认证表单随标签切换，保留各标签的输入。
func TestAuthWindowsFollowTabs(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("需要隔离图形测试")
	}
	off := mygo.App.OnWindowAllClosed(func() {})
	defer off()
	a := &Browser{}
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title: "Auth tabs test", Hidden: true, Width: 320, Height: 200,
		Content: ui.View(a.View),
	})
	defer a.win.Destroy()
	firstView, err := a.win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: mygo.NewEphemeralSession()}})
	if err != nil {
		t.Fatal(err)
	}
	secondView, err := a.win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: mygo.NewEphemeralSession()}})
	if err != nil {
		t.Fatal(err)
	}
	thirdView, err := a.win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: mygo.NewEphemeralSession()}})
	if err != nil {
		t.Fatal(err)
	}
	loaded := make(chan struct{}, 1)
	firstView.Page().OnDidFinishLoad(func() { loaded <- struct{}{} })
	firstView.Page().LoadHTML("<html><body><button id='item' onclick='window.testValue++'>点按</button></body></html>", "")
	select {
	case <-loaded:
	case <-time.After(10 * time.Second):
		t.Fatal("测试网页加载超时")
	}
	if _, err := firstView.Page().Eval("window.testValue = 41"); err != nil {
		t.Fatal(err)
	}
	first := a.tabs.add(firstView)
	second := a.tabs.add(secondView)
	makePrompt := func(tab *browserTab, username, password string) *authPrompt {
		ctx, cancel := context.WithCancel(context.Background())
		p := &authPrompt{tab: tab, view: tab.view, generation: tab.generation,
			url: "https://example.test/", need: auth.Need{Host: "example.test"}, ctx: ctx, cancel: cancel,
			username: username, password: password, errorText: "请重试"}
		tab.authPrompt = p
		return p
	}
	firstPrompt := makePrompt(first, "first", "first-secret")
	secondPrompt := makePrompt(second, "second", "second-secret")
	defer func() {
		mygo.RunOnMain(func() {
			a.cancelTabAuth(first)
			a.cancelTabAuth(second)
		})
	}()

	var windowsBefore, windowsAfter int
	mygo.RunOnMain(func() {
		a.win.Show()
		windowsBefore = len(mygo.Windows())
		a.selectTab(first)
		a.showAuth(firstPrompt)
		a.showAuth(secondPrompt)
		windowsAfter = len(mygo.Windows())
	})
	if windowsBefore != windowsAfter {
		t.Fatal("认证不应创建独立窗口")
	}
	if !firstPrompt.ready || !secondPrompt.ready || a.tabs.active != first {
		t.Fatal("后台认证不应改变当前标签")
	}
	mygo.RunOnMain(func() {
		a.win.Hide()
		if a.win.IsVisible() {
			t.Error("主窗口未隐藏")
		}
		a.win.Show()
		a.win.SetIgnoreMouseEvents(true)
		a.win.SetOpacity(0.55)
		if !a.win.IsVisible() || a.win.Opacity() < 0.54 || a.win.Opacity() > 0.56 {
			t.Error("主窗口显示或不透明度错误")
		}
		a.win.SetIgnoreMouseEvents(false)
		a.win.SetOpacity(1)
		third := a.addTab(thirdView)
		a.selectTab(first)
		a.closeTab(third)
		a.selectTab(second)
		a.selectTab(first)
	})
	if first.authPrompt != firstPrompt || firstPrompt.username != "first" || firstPrompt.password != "first-secret" || firstPrompt.errorText != "请重试" || second.authPrompt != secondPrompt || secondPrompt.password != "second-secret" {
		t.Fatal("切换标签应保留各自认证输入")
	}
	mygo.RunOnMain(func() {
		a.selectTab(second)
		a.closeTab(second)
	})
	mygo.RunOnMain(func() { a.cancelTabAuth(first) })
	value, err := firstView.Page().Eval("(document.getElementById('item').click(), window.testValue)")
	if err != nil || fmt.Sprint(value) != "42" {
		t.Fatalf("认证结束后网页状态或交互异常：%v %v", value, err)
	}
	if a.tabs.active != first || second.authPrompt != nil || secondPrompt.ctx.Err() == nil || secondPrompt.password != "" {
		t.Fatal("关闭标签应取消对应认证并清除密码")
	}
}
