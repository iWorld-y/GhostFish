//go:build darwin && gui

package desktop

import (
	"os"
	"testing"

	"github.com/egoist/mygo"
)

// MyGo 的 WindowOptions.Opacity 对 <= 0 会退回 1，只有 SetOpacity(0) 能真正设成 0；
// 应用在显示前显式调用它，所以这里验证真实窗口的两个事实。
func TestZeroOpacityOnRealWindow(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		// 没有 GHOSTFISH_GUI 时 TestMain 不启动 mygo 主循环，
		// NewWindow 会一直等 App ready，所以这里必须先跳过。
		t.Skip("设置 GHOSTFISH_GUI=1 才打开真实窗口")
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:   "opacity probe",
		Hidden:  true,
		Width:   320,
		Height:  200,
		Opacity: 0,
	})
	defer win.Destroy()

	if got := win.Opacity(); got != 1 {
		t.Fatalf("WindowOptions 的 0 应被框架换成 1，实际是 %v", got)
	}
	t.Logf("WindowOptions{Opacity: 0} 的实际值 = %v（框架把 <=0 换成 1）", win.Opacity())
	a := &Controller{win: win}
	a.setOpacity(0)
	if a.opacity != 0 {
		t.Fatalf("应用记录的不透明度 = %v，want 0", a.opacity)
	}
	win.Show()
	if got := win.Opacity(); got != 0 {
		t.Fatalf("显示后窗口不透明度 = %v，want 0", got)
	}
	if !win.IsVisible() {
		t.Fatal("0% 不透明度的窗口应当仍然可见")
	}
	t.Logf("SetOpacity(0) 后显示的真实窗口不透明度 = %v，IsVisible = %v", win.Opacity(), win.IsVisible())
}
