//go:build darwin && gui

package desktop

import (
	"os"
	"sync/atomic"
	"testing"
	"time"

	cfg "ghostfish/internal/config"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type dragTestPoint struct{ X, Y float64 }

type dragTestEvents struct {
	createMouse func(uintptr, uint32, dragTestPoint, uint32) uintptr
	setFlags    func(uintptr, uint64)
	setInteger  func(uintptr, uint32, int64)
	post        func(uint32, uintptr)
	release     func(uintptr)
	create      func(uintptr) uintptr
	location    func(uintptr) dragTestPoint
	warp        func(dragTestPoint) int32
}

// newDragTestEvents 加载测试所需的鼠标事件函数。
func newDragTestEvents(t *testing.T) dragTestEvents {
	t.Helper()
	lib, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	var events dragTestEvents
	purego.RegisterLibFunc(&events.createMouse, lib, "CGEventCreateMouseEvent")
	purego.RegisterLibFunc(&events.setFlags, lib, "CGEventSetFlags")
	purego.RegisterLibFunc(&events.setInteger, lib, "CGEventSetIntegerValueField")
	purego.RegisterLibFunc(&events.post, lib, "CGEventPost")
	purego.RegisterLibFunc(&events.create, lib, "CGEventCreate")
	purego.RegisterLibFunc(&events.location, lib, "CGEventGetLocation")
	purego.RegisterLibFunc(&events.warp, lib, "CGWarpMouseCursorPosition")
	purego.RegisterLibFunc(&events.release, cf, "CFRelease")
	return events
}

// mouse 发送测试鼠标事件；拖动事件记录相对位移，按下和松开记录单击状态。
func (e dragTestEvents) mouse(t *testing.T, kind uint32, point dragTestPoint, flags uint64, dx, dy int64) {
	t.Helper()
	event := e.createMouse(0, kind, point, 0)
	if event == 0 {
		t.Fatal("CGEventCreateMouseEvent failed")
	}
	e.setFlags(event, flags)
	if kind == 1 || kind == 2 || kind == 6 {
		e.setInteger(event, 1, 1) // kCGMouseEventClickState
	}
	if kind == 6 {
		e.setInteger(event, 4, dx) // kCGMouseEventDeltaX
		e.setInteger(event, 5, dy) // kCGMouseEventDeltaY
	}
	e.post(0, event)
	e.release(event)
}

func (e dragTestEvents) drag(t *testing.T, start, end dragTestPoint, flags uint64, releaseModifiers bool) {
	t.Helper()
	e.mouse(t, 5, start, 0, 0, 0)
	time.Sleep(100 * time.Millisecond)
	e.mouse(t, 1, start, flags, 0, 0)
	time.Sleep(100 * time.Millisecond)
	previous := start
	for i := 1; i <= 5; i++ {
		point := dragTestPoint{X: start.X + (end.X-start.X)*float64(i)/5, Y: start.Y + (end.Y-start.Y)*float64(i)/5}
		moveFlags := flags
		if releaseModifiers && i >= 3 {
			moveFlags = 0
		}
		e.mouse(t, 6, point, moveFlags, int64(point.X-previous.X), int64(point.Y-previous.Y))
		previous = point
		time.Sleep(50 * time.Millisecond)
	}
	e.mouse(t, 2, end, 0, 0, 0)
	time.Sleep(250 * time.Millisecond)
}

func (e dragTestEvents) click(t *testing.T, point dragTestPoint, flags uint64) {
	t.Helper()
	e.mouse(t, 5, point, 0, 0, 0)
	time.Sleep(100 * time.Millisecond)
	e.mouse(t, 1, point, flags, 0, 0)
	e.mouse(t, 2, point, flags, 0, 0)
	time.Sleep(200 * time.Millisecond)
}

// TestWindowDragMonitorOnRealWindow 验证监听器能够在真实 AppKit 窗口上安装和移除。
func TestWindowDragMonitorOnRealWindow(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("设置 GHOSTFISH_GUI=1 才打开真实窗口")
	}
	off := mygo.App.OnWindowAllClosed(func() {})
	defer off()
	win := mygo.NewWindow(mygo.WindowOptions{Title: "GhostFish window drag test", Hidden: true, Width: 320, Height: 240})
	a := &Controller{win: win}
	var installErr error
	mygo.RunOnMain(func() { installErr = a.installWindowDrag() })
	if installErr != nil {
		win.Destroy()
		t.Fatal(installErr)
	}
	win.Show()
	win.Destroy()
}

// TestWindowDragRealContent 使用独立页面和原生控件验证真实 AppKit 鼠标事件。
func TestWindowDragRealContent(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("设置 GHOSTFISH_GUI=1 才打开真实窗口")
	}
	off := mygo.App.OnWindowAllClosed(func() {})
	defer off()
	t.Run("baseline", func(t *testing.T) { runDragContent(t, false) })
	t.Run("monitor", func(t *testing.T) { runDragContent(t, true) })
}

func runDragContent(t *testing.T, install bool) {
	t.Helper()
	var view *mygo.WebView
	var clicks atomic.Int32
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: "GhostFish drag integration test", Hidden: true, Width: 500, Height: 400,
		TitleBarStyle: mygo.TitleBarHidden, AlwaysOnTop: true,
		Content: ui.View(func(c *ui.Context) {
			ui.Column(c).Fill().Children(func() {
				ui.Button(c, "测试控件").Width(140).Height(50).OnClick(func() { clicks.Add(1) })
				if view != nil {
					ui.WebView(c, view).Grow(1)
				}
			})
		}),
	})
	defer win.Destroy()
	loaded := make(chan struct{}, 1)
	var setupErr error
	a := &Controller{win: win}
	mygo.RunOnMain(func() {
		view, setupErr = win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: mygo.NewEphemeralSession()}})
		if setupErr != nil {
			return
		}
		view.Page().OnDidFinishLoad(func() {
			select {
			case loaded <- struct{}{}:
			default:
			}
		})
		view.Page().LoadHTML(`<html><body style="margin:0"><a href="#test" style="position:absolute;left:20px;top:20px;width:120px;height:30px">测试链接</a><input value="test" style="position:absolute;left:20px;top:70px;width:120px;height:30px"><p id="selection" style="position:absolute;left:20px;top:150px;font-size:20px;margin:0">可选择的测试文本和更多文字</p></body></html>`, "")
		if install {
			setupErr = a.installWindowDrag()
		}
		win.Invalidate()
	})
	if setupErr != nil {
		t.Fatal(setupErr)
	}
	win.Show()
	mygo.App.Focus()
	win.Focus()
	select {
	case <-loaded:
	case <-time.After(5 * time.Second):
		t.Fatal("test web view did not load")
	}
	time.Sleep(300 * time.Millisecond)
	var hitClose, hitWeb, hitNative bool
	var closeButton objc.ID
	var closeLocation windowDragPoint
	mygo.RunOnMain(func() {
		target := objc.ID(win.NativeHandle())
		closeButton = target.Send(objc.RegisterName("standardWindowButton:"), uintptr(0))
		hitWeb = windowDragLocationHitsContent(target, windowDragPoint{X: 250, Y: 220})
		hitNative = windowDragLocationHitsContent(target, windowDragPoint{X: 110, Y: 375})
		if closeButton != 0 {
			closeLocation = objc.Send[windowDragPoint](closeButton, objc.RegisterName("convertPoint:toView:"), windowDragPoint{X: 5, Y: 5}, objc.ID(0))
			hitClose = windowDragLocationHitsContent(target, closeLocation)
		}
	})
	if !hitWeb || !hitNative {
		t.Fatalf("content hit failed: web=%v native=%v", hitWeb, hitNative)
	}
	if closeButton == 0 || hitClose {
		t.Fatalf("native close button belongs to content: button=%#x hit=%v", closeButton, hitClose)
	}
	events := newDragTestEvents(t)
	current := events.create(0)
	if current == 0 {
		t.Fatal("CGEventCreate failed")
	}
	oldMouse := events.location(current)
	events.release(current)
	defer events.warp(oldMouse)
	locations := []struct {
		name string
		x, y int
	}{{"webview", 250, 180}, {"native control", 110, 25}, {"link", 60, 80}, {"input", 60, 130}}
	if !install {
		locations = nil
	}
	for _, location := range locations {
		before := win.Bounds()
		start := dragTestPoint{X: float64(before.X + location.x), Y: float64(before.Y + location.y)}
		end := dragTestPoint{X: start.X + 60, Y: start.Y + 40}
		events.drag(t, start, end, flagCommand, location.name == "native control")
		after := win.Bounds()
		if dx, dy := after.X-before.X, after.Y-before.Y; dx != 60 || dy != 40 {
			t.Fatalf("%s: window moved by (%d,%d), want (60,40); before=%+v after=%+v", location.name, dx, dy, before, after)
		}
		t.Logf("%s: before=%+v after=%+v", location.name, before, after)
	}
	if install {
		config := cfg.DefaultSettings()
		mygo.RunOnMain(func() { a.st.Settings = &config })
		for _, configured := range []struct {
			name      string
			modifiers cfg.DragModifiers
			flags     uint64
		}{
			{"Option+Shift", cfg.DragModifiers{Option: true, Shift: true}, flagOption | flagShift},
			{"Control", cfg.DragModifiers{Control: true}, flagControl},
		} {
			mygo.RunOnMain(func() { config.DragModifiers = configured.modifiers })
			before := win.Bounds()
			start := dragTestPoint{X: float64(before.X + 250), Y: float64(before.Y + 180)}
			end := dragTestPoint{X: start.X + 30, Y: start.Y + 20}
			events.drag(t, start, end, configured.flags, false)
			after := win.Bounds()
			if dx, dy := after.X-before.X, after.Y-before.Y; dx != 30 || dy != 20 {
				t.Fatalf("%s: window moved by (%d,%d), want (30,20)", configured.name, dx, dy)
			}
		}
		mygo.RunOnMain(func() { config.DragModifiers = cfg.DragModifiers{Command: true} })
	}
	if clicks.Load() != 0 {
		t.Fatalf("drag activated native control: %d", clicks.Load())
	}
	before := win.Bounds()
	control := dragTestPoint{X: float64(before.X + 110), Y: float64(before.Y + 25)}
	events.click(t, control, 0)
	events.click(t, control, flagCommand|flagShift)
	if got := clicks.Load(); got != 2 {
		t.Fatalf("unmodified and nonmatching clicks: got %d, want 2", got)
	}
	if after := win.Bounds(); after != before {
		t.Fatalf("nonmatching clicks moved window: before=%+v after=%+v", before, after)
	}
	_, err := view.Page().Eval(`window.__dragTest = []; ["mousedown", "mouseup", "mousemove"].forEach(name => document.addEventListener(name, e => window.__dragTest.push([e.type, e.clientX, e.clientY, e.buttons, e.movementX, e.movementY, e.detail, e.target.id, document.hasFocus()])));`)
	if err != nil {
		t.Fatal(err)
	}
	selectionStart := dragTestPoint{X: float64(before.X + 26), Y: float64(before.Y + 50 + 160)}
	selectionEnd := dragTestPoint{X: selectionStart.X + 130, Y: selectionStart.Y}
	// 第一次点击先将焦点交给网页，再进行独立的选择拖动。
	events.click(t, selectionStart, 0)
	deadline := time.Now().Add(2 * time.Second)
	for {
		focused, focusErr := view.Page().Eval("document.hasFocus()")
		if focusErr != nil {
			t.Fatal(focusErr)
		}
		if focused == true {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("test web view did not receive focus")
		}
		time.Sleep(25 * time.Millisecond)
	}
	events.drag(t, selectionStart, selectionEnd, 0, false)
	deadline = time.Now().Add(2 * time.Second)
	for {
		selection, err := view.Page().Eval("window.getSelection().toString()")
		if err != nil {
			t.Fatal(err)
		}
		if selection != nil && selection != "" {
			break
		}
		if time.Now().After(deadline) {
			detail, _ := view.Page().Eval(`JSON.stringify({events: window.__dragTest, rect: document.getElementById("selection").getBoundingClientRect().toJSON()})`)
			t.Fatalf("normal text selection remained empty: %v", detail)
		}
		time.Sleep(25 * time.Millisecond)
	}
	if after := win.Bounds(); after != before {
		t.Fatalf("ordinary text drag moved window: before=%+v after=%+v", before, after)
	}
	if install {
		var closeRequests atomic.Int32
		win.OnClose(func(event *mygo.CloseEvent) { closeRequests.Add(1); event.PreventDefault() })
		closePoint := dragTestPoint{X: float64(before.X) + closeLocation.X, Y: float64(before.Y+before.Height) - closeLocation.Y}
		events.click(t, closePoint, flagCommand)
		deadline = time.Now().Add(2 * time.Second)
		for closeRequests.Load() == 0 && time.Now().Before(deadline) {
			time.Sleep(25 * time.Millisecond)
		}
		if closeRequests.Load() != 1 {
			t.Fatalf("native close button did not receive click: %d", closeRequests.Load())
		}
		var underPresses atomic.Int32
		under := mygo.NewWindow(mygo.WindowOptions{
			Title: "GhostFish drag underlay test", Hidden: true, Width: before.Width + 60, Height: before.Height + 60,
			TitleBarStyle: mygo.TitleBarHidden,
			Content: ui.View(func(c *ui.Context) {
				ui.Box(c).Fill().HandleInput(func(event ui.InputEvent) bool {
					if event.Kind == ui.InputPointerDown {
						underPresses.Add(1)
					}
					return true
				})
			}),
		})
		defer under.Destroy()
		under.SetPosition(before.X-30, before.Y-30)
		under.Show()
		win.Focus()
		mygo.RunOnMain(func() { a.setIgnoreMouse(true) })
		throughStart := dragTestPoint{X: float64(before.X + 250), Y: float64(before.Y + 180)}
		throughEnd := dragTestPoint{X: throughStart.X + 60, Y: throughStart.Y + 40}
		events.drag(t, throughStart, throughEnd, flagCommand, false)
		mygo.RunOnMain(func() { a.setIgnoreMouse(false) })
		if after := win.Bounds(); after != before || underPresses.Load() == 0 {
			t.Fatalf("click-through drag: before=%+v after=%+v underlay presses=%d", before, after, underPresses.Load())
		}
	}
}
