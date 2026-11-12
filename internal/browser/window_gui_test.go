//go:build darwin && gui

package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	cfg "ghostfish/internal/config"
	"ghostfish/internal/desktop"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 使用内存会话验证隐藏和恢复不会重新创建网页或丢失页面脚本状态。
func TestTrayHideKeepsWebViewState(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("设置 GHOSTFISH_GUI=1 才打开真实窗口")
	}
	a := &Browser{}
	a.win = mygo.NewWindow(mygo.WindowOptions{Title: "GhostFish page test", Hidden: true, Width: 320, Height: 200, Content: ui.View(a.View)})
	controls := desktop.New(a.win, cfg.State{Opacity: .85}, filepath.Join(t.TempDir(), "state.json"))
	view, err := a.win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: mygo.NewEphemeralSession()}})
	if err != nil {
		t.Fatal(err)
	}
	defer view.Destroy()
	tab := a.tabs.add(view)
	ready := make(chan struct{}, 1)
	view.Page().OnDidFinishLoad(func() { ready <- struct{}{} })
	view.Page().LoadHTML("<html><body><p>GhostFish test</p></body></html>", "")
	a.win.Show()
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("测试网页加载超时")
	}
	if _, err := view.Page().Eval("window.ghostfishTest = 42"); err != nil {
		t.Fatal(err)
	}
	controls.ToggleHidden()
	controls.ToggleHidden()
	got, err := view.Page().Eval("window.ghostfishTest")
	if err != nil {
		t.Fatal(err)
	}
	if a.tabs.active != tab || tab.view != view || view.IsDestroyed() || fmt.Sprint(got) != "42" {
		t.Fatalf("隐藏后页面状态改变：value=%v, destroyed=%v", got, view.IsDestroyed())
	}
}
