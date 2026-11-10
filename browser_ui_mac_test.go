package main

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

// rgba 把主题颜色转成测试比对的像素值。
func rgba(c ui.Color) color.RGBA { return color.RGBA{c.R, c.G, c.B, c.A} }

// tabStart 是标签栏内容的左边界：标题栏左侧留 78 给窗口按钮，再加 8 的内边距。
const tabStart = 78 + 8

func macBarTester(a *app, width int) *ui.Tester {
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(c.Theme().Background).Children(func() { a.macTabBar(c) })
	}, width, 60)
	tester.SetTitleBar(ui.TitleBar{Height: 36, Left: 78})
	return tester
}

func macTabs(a *app, titles ...string) {
	for _, title := range titles {
		tab := a.tabs.add(nil)
		tab.title = title
	}
}

// 窄窗口和多个标签时，新建按钮、拖动区域和设置入口都要留着。
func TestMacTabBarKeepsActionsInNarrowWindow(t *testing.T) {
	var a app
	macTabs(&a, "Tab 01", "Tab 02", "Tab 03", "Tab 04", "Tab 05", "Tab 06")
	tester := macBarTester(&a, 320)
	plus, foundPlus := tester.Find("新建标签页")
	gear, foundGear := tester.Find("设置")
	if !foundPlus || !foundGear {
		t.Fatalf("plus=%v gear=%v", foundPlus, foundGear)
	}
	if plus.X < tabStart || plus.X+plus.W > 320 {
		t.Fatalf("new tab button outside the bar: %+v", plus)
	}
	if gear.X+gear.W > 320 || gear.X < plus.X+plus.W {
		t.Fatalf("settings entry outside the bar: gear=%+v plus=%+v", gear, plus)
	}
	if gap := gear.X - (plus.X + plus.W); gap < 32 {
		t.Fatalf("drag area squeezed out: gap=%v", gap)
	}
	if plus.H != 36 || gear.H != 36 {
		t.Fatalf("bar buttons should fill the bar height: plus=%+v gear=%+v", plus, gear)
	}
}

// 当前标签是直角的背景块，不能被画成强调色，并且要占满标签栏高度。
func TestMacSelectedTabIsSquareAndUnfilled(t *testing.T) {
	var a app
	macTabs(&a, "Tab 1", "Tab 2", "Tab 3")
	a.tabs.selectTab(a.tabs.items[1])
	tester := macBarTester(&a, 800)
	active, ok := tester.Find("Tab 2")
	if !ok {
		t.Fatal("selected tab title missing")
	}
	theme := ui.LightTheme()
	img := tester.Image()
	x := int(active.X + 2)
	for _, y := range []int{1, 34} {
		if got := img.RGBAAt(x, y); got == rgba(theme.Accent) {
			t.Fatalf("selected tab painted with the accent color at y=%d", y)
		} else if got != rgba(theme.Background) {
			t.Fatalf("selected tab should use the window background: y=%d got %v", y, got)
		}
	}
	// 竖向分隔线从标签栏顶部开始，说明当前标签是直角而不是圆角。
	border := -1
	for bx := tabStart; bx < int(active.X); bx++ {
		if img.RGBAAt(bx, 1) == rgba(theme.Border) {
			border = bx
		}
	}
	if border < 0 {
		t.Fatal("selected tab has no leading divider")
	}
	if got := img.RGBAAt(border, 0); got != rgba(theme.Border) {
		t.Fatalf("divider should reach the top of the bar: %v", got)
	}
}

// 悬停反馈覆盖整个标签：左右边距和上下高度都算在标签内，四角保持直角。
func TestMacTabHoverCoversWholeTab(t *testing.T) {
	var a app
	macTabs(&a, "Tab 1", "Tab 2", "Tab 3")
	a.tabs.selectTab(a.tabs.items[1])
	tester := macBarTester(&a, 800)
	first, ok := tester.Find("Tab 1")
	if !ok {
		t.Fatal("inactive tab title missing")
	}
	theme := ui.LightTheme()
	before := tester.Image()
	if got := before.RGBAAt(tabStart+2, 1); got != rgba(theme.Background) {
		t.Fatalf("bar background changed before hovering: %v", got)
	}
	tester.Move(first.X+first.W/2, first.Y+first.H/2)
	img := tester.Image()
	for _, point := range [][2]int{{tabStart, 0}, {tabStart, 34}, {int(first.X + first.W - 1), 0}, {int(first.X + first.W - 1), 34}} {
		if got := img.RGBAAt(point[0], point[1]); got != rgba(theme.SurfaceHover) {
			t.Fatalf("hover feedback does not cover the whole tab at %v: %v", point, got)
		}
	}
}

// 关闭图标在按钮内水平、垂直居中。
func TestMacCloseIconCentered(t *testing.T) {
	var a app
	macTabs(&a, "Tab 1", "Tab 2")
	a.tabs.selectTab(a.tabs.items[1])
	tester := macBarTester(&a, 800)
	button, ok := tester.Find("关闭标签页")
	if !ok {
		t.Fatal("close button missing")
	}
	icon, ok := tester.Find("×")
	if !ok {
		t.Fatal("close icon missing")
	}
	dx := (icon.X + icon.W/2) - (button.X + button.W/2)
	dy := (icon.Y + icon.H/2) - (button.Y + button.H/2)
	if fmt.Sprintf("%.1f", dx) != "0.0" || dy < -1 || dy > 1 {
		t.Fatalf("close icon off center: dx=%.2f dy=%.2f button=%+v icon=%+v", dx, dy, button, icon)
	}
	if button.H != 30 {
		t.Fatalf("close button height = %v", button.H)
	}
}
