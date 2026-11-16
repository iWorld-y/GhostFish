//go:build darwin && gui

package browser

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"ghostfish/internal/auth"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type authTestPoint struct{ X, Y float64 }

// TestAuthFormNativeMouse 验证原生鼠标命中、穿透及网页恢复。
func TestAuthFormNativeMouse(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("需要隔离图形测试")
	}
	off := mygo.App.OnWindowAllClosed(func() {})
	defer off()
	cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	var mouse func(uintptr, uint32, authTestPoint, uint32) uintptr
	var key func(uintptr, uint16, bool) uintptr
	var setInteger func(uintptr, uint32, int64)
	var post func(uint32, uintptr)
	var release func(uintptr)
	var create func(uintptr) uintptr
	var location func(uintptr) authTestPoint
	var warp func(authTestPoint) int32
	purego.RegisterLibFunc(&mouse, cg, "CGEventCreateMouseEvent")
	purego.RegisterLibFunc(&key, cg, "CGEventCreateKeyboardEvent")
	purego.RegisterLibFunc(&setInteger, cg, "CGEventSetIntegerValueField")
	purego.RegisterLibFunc(&post, cg, "CGEventPost")
	purego.RegisterLibFunc(&release, cf, "CFRelease")
	purego.RegisterLibFunc(&create, cg, "CGEventCreate")
	purego.RegisterLibFunc(&location, cg, "CGEventGetLocation")
	purego.RegisterLibFunc(&warp, cg, "CGWarpMouseCursorPosition")
	current := create(0)
	if current == 0 {
		t.Fatal("CGEventCreate 失败")
	}
	oldMouse := location(current)
	release(current)
	defer warp(oldMouse)
	click := func(point authTestPoint) {
		t.Helper()
		for _, kind := range []uint32{5, 1, 2} {
			event := mouse(0, kind, point, 0)
			if event == 0 {
				t.Fatal("CGEventCreateMouseEvent 失败")
			}
			if kind == 1 || kind == 2 {
				setInteger(event, 1, 1)
			}
			post(0, event)
			release(event)
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(200 * time.Millisecond)
	}
	var presses atomic.Int32
	under := mygo.NewWindow(mygo.WindowOptions{Title: "Auth underlay", Hidden: true, Width: 560, Height: 460, TitleBarStyle: mygo.TitleBarHidden, Content: ui.View(func(c *ui.Context) {
		ui.Box(c).Fill().HandleInput(func(event ui.InputEvent) bool {
			if event.Kind == ui.InputPointerDown {
				presses.Add(1)
			}
			return true
		})
	})})
	defer under.Destroy()
	a := &Browser{}
	a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Auth native test", Hidden: true, Width: 500, Height: 400, TitleBarStyle: mygo.TitleBarHidden, AlwaysOnTop: true, Content: ui.View(a.View)})
	defer a.win.Destroy()
	view, err := a.win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: mygo.NewEphemeralSession()}})
	if err != nil {
		t.Fatal(err)
	}
	loaded := make(chan struct{}, 1)
	view.Page().OnDidFinishLoad(func() { loaded <- struct{}{} })
	view.Page().LoadHTML(`<html><body style="margin:0"><button id="target" style="position:absolute;left:20px;top:20px;width:200px;height:60px" onclick="window.clicked=(window.clicked||0)+1">点击</button></body></html>`, "")
	select {
	case <-loaded:
	case <-time.After(10 * time.Second):
		t.Fatal("网页加载超时")
	}
	tab := a.tabs.add(view)
	tab.openedPage = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &authPrompt{tab: tab, view: view, generation: tab.generation, ctx: ctx, cancel: cancel, url: "https://example.test/", need: auth.Need{Host: "example.test"}}
	tab.authPrompt = p
	mygo.RunOnMain(func() {
		under.SetPosition(170, 120)
		a.win.SetPosition(200, 150)
		under.Show()
		a.win.Show()
		mygo.App.Focus()
		a.win.Focus()
		a.showAuth(p)
	})
	tester := ui.NewTester(a.View, 500, 400)
	input, ok := tester.Find("用户名")
	if !ok {
		t.Fatal("未找到用户名输入框")
	}
	bounds := a.win.Bounds()
	point := authTestPoint{X: float64(bounds.X) + float64(input.X+input.W/2), Y: float64(bounds.Y + int(input.Y+4))}
	time.Sleep(400 * time.Millisecond)
	a.win.SetIgnoreMouseEvents(true)
	click(point)
	if presses.Load() == 0 {
		t.Fatal("表单区域的鼠标事件未穿透到下层窗口")
	}
	a.win.SetIgnoreMouseEvents(false)
	mygo.App.Focus()
	a.win.Show()
	a.win.Focus()
	time.Sleep(300 * time.Millisecond)
	t.Logf("window=%+v input=%+v point=%+v under=%d", bounds, input, point, presses.Load())
	click(point)
	for _, down := range []bool{true, false} {
		event := key(0, 0, down)
		if event == 0 {
			t.Fatal("CGEventCreateKeyboardEvent 失败")
		}
		post(0, event)
		release(event)
	}
	time.Sleep(300 * time.Millisecond)
	var username string
	mygo.RunOnMain(func() { username = p.username })
	if username != "a" {
		t.Fatalf("真实输入未到达认证表单：%q", username)
	}

	mygo.RunOnMain(func() { a.cancelTabAuth(tab) })
	time.Sleep(400 * time.Millisecond)
	click(authTestPoint{X: float64(bounds.X + 100), Y: float64(bounds.Y + 36 + 50)})
	value, err := view.Page().Eval("window.clicked || 0")
	if err != nil || fmt.Sprint(value) != "1" {
		t.Fatalf("认证结束后真实点击未到达网页：%v %v", value, err)
	}
}
