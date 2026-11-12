//go:build darwin && gui

package browser

import (
	"context"
	"os"
	"testing"

	"ghostfish/internal/auth"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 认证窗口随标签切换隐藏和恢复，不丢失两个标签各自的输入。
func TestAuthWindowsFollowTabs(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("需要隔离图形测试")
	}
	off := mygo.App.OnWindowAllClosed(func() {})
	defer off()
	a := &Browser{}
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title: "Auth tabs test", Hidden: true, Width: 320, Height: 200,
		Content: ui.View(func(c *ui.Context) { ui.Text(c, "认证标签测试") }),
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

	var firstWindow *mygo.Window
	var firstVisible, secondVisible, firstKept, secondKept bool
	var backgroundShown, switchedToSecond bool
	mygo.RunOnMain(func() {
		a.win.Show()
		a.selectTab(first)
		a.showAuth(firstPrompt)
		firstWindow = firstPrompt.window
		firstVisible = firstWindow != nil && firstWindow.IsVisible()
	})
	if !firstVisible {
		t.Fatal("当前标签的认证窗口应当显示")
	}

	var thirdActive, restored bool
	mygo.RunOnMain(func() {
		third := a.addTab(thirdView)
		thirdActive = a.tabs.active == third && !firstWindow.IsVisible()
		a.selectTab(first)
		restored = firstPrompt.window == firstWindow && firstWindow.IsVisible()
		a.closeTab(third)
	})
	if !thirdActive || !restored {
		t.Fatalf("新建标签后认证窗口未随标签切换：thirdActive=%v restored=%v", thirdActive, restored)
	}

	mygo.RunOnMain(func() {
		a.showAuth(secondPrompt)
		backgroundShown = secondPrompt.window != nil && secondPrompt.window.IsVisible()
		a.selectTab(second)
		firstVisible = firstWindow.IsVisible()
		switchedToSecond = secondPrompt.window != nil && secondPrompt.window.IsVisible()
		firstKept = first.authPrompt == firstPrompt && firstPrompt.ctx.Err() == nil && firstPrompt.username == "first" && firstPrompt.password == "first-secret" && firstPrompt.errorText == "请重试"
	})
	if firstVisible || backgroundShown || !switchedToSecond || !firstKept {
		t.Fatalf("切换后认证状态错误：firstVisible=%v backgroundShown=%v secondVisible=%v firstKept=%v", firstVisible, backgroundShown, switchedToSecond, firstKept)
	}

	mygo.RunOnMain(func() {
		a.selectTab(first)
		firstVisible = firstPrompt.window == firstWindow && firstWindow.IsVisible()
		secondVisible = secondPrompt.window != nil && secondPrompt.window.IsVisible()
		secondKept = second.authPrompt == secondPrompt && secondPrompt.ctx.Err() == nil && secondPrompt.username == "second" && secondPrompt.password == "second-secret"
	})
	if !firstVisible || secondVisible || !secondKept {
		t.Fatalf("切回后认证状态错误：firstVisible=%v secondVisible=%v secondKept=%v", firstVisible, secondVisible, secondKept)
	}

	mygo.RunOnMain(func() {
		a.selectTab(second)
		a.closeTab(second)
		firstVisible = firstPrompt.window == firstWindow && firstWindow.IsVisible()
		secondKept = second.authPrompt == nil && secondPrompt.ctx.Err() != nil && secondPrompt.password == ""
	})
	if !firstVisible || !secondKept {
		t.Fatalf("关闭当前标签后认证状态错误：firstVisible=%v secondCleared=%v", firstVisible, secondKept)
	}
}
