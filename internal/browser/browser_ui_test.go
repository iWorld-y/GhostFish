package browser

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNewTabAddressBarFocusAndBlur(t *testing.T) {
	a := &Browser{focusAddress: true}
	tab := a.tabs.add(nil)
	tab.url = "https://example.com/start"
	tab.input = tab.url
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			a.addressBar(c, tab)
			ui.Button(c, "其他控件")
		})
	}, 640, 120)
	if !tester.Focused("地址栏") {
		t.Fatal("new blank tab should focus the address field")
	}
	tester.Type("draft")
	if !tab.editing {
		t.Fatal("typing should mark the input as being edited")
	}
	if err := tester.Click("其他控件"); err != nil {
		t.Fatal(err)
	}
	if !tab.editing || tab.input != "https://example.com/startdraft" {
		t.Fatalf("blur should preserve a new tab's draft: %+v", tab)
	}
}

func TestBrowserHeaderReservesWindowControls(t *testing.T) {
	a := &Browser{}
	for _, title := range []string{"Tab 1", "Tab 2", "Tab 3"} {
		tab := a.tabs.add(nil)
		tab.title = title
	}
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			a.tabBar(c)
			a.addressBar(c, a.tabs.active)
		})
	}, 360, 120)
	tester.SetTitleBar(ui.TitleBar{Height: 36, Left: 78})
	first, ok := tester.Find("Tab 1")
	if !ok || first.X < 78 {
		t.Fatalf("first tab overlaps window controls: %+v, found %v", first, ok)
	}
	plus, ok := tester.Find("新建标签页")
	if !ok || plus.W <= 0 || plus.X+plus.W > 360 {
		t.Fatalf("new tab button should remain visible: %+v, found %v", plus, ok)
	}
	address, ok := tester.Find("地址栏")
	if !ok || address.Y < 36 {
		t.Fatalf("address bar should be below tabs: %+v, found %v", address, ok)
	}
}

func TestNewTabButtonFollowsTabs(t *testing.T) {
	var a Browser
	tab := a.tabs.add(nil)
	tab.title = "标签页"
	tester := ui.NewTester(func(c *ui.Context) { a.tabBar(c) }, 960, 40)
	tester.SetTitleBar(ui.TitleBar{Height: 36, Left: 78})
	closeButton, ok := tester.Find("关闭标签页")
	if !ok {
		t.Fatal("close button should be visible")
	}
	plus, ok := tester.Find("新建标签页")
	if !ok || plus.X < closeButton.X+closeButton.W || plus.X-(closeButton.X+closeButton.W) > 8 {
		t.Fatalf("new tab button should immediately follow the last tab: close=%+v plus=%+v", closeButton, plus)
	}
	if plus.X+plus.W > 960-32 {
		t.Fatalf("remaining title bar should be available for dragging: %+v", plus)
	}
}

func TestTabBarKeepsNewTabButtonWhenTabsOverflow(t *testing.T) {
	var a Browser
	for i := 0; i < 12; i++ {
		tab := a.tabs.add(nil)
		tab.title = fmt.Sprintf("Tab %02d", i)
	}
	tester := ui.NewTester(func(c *ui.Context) { a.tabBar(c) }, 360, 40)
	tester.SetTitleBar(ui.TitleBar{Height: 36, Left: 78})
	plus, ok := tester.Find("新建标签页")
	if !ok || plus.W <= 0 || plus.X < 78 || plus.X+plus.W > 360-32 {
		t.Fatalf("new tab button and drag area should remain visible: %+v, found %v", plus, ok)
	}
	tester.Scroll(120, 18, 1500, 0)
	last, ok := tester.Find("Tab 11")
	if !ok || last.X < 78 || last.X+last.W > plus.X {
		t.Fatalf("last tab should be reachable by scrolling: last=%+v, found=%v, plus=%+v", last, ok, plus)
	}
}

func TestNewActiveTabIsRevealedWhenTabsOverflow(t *testing.T) {
	var a Browser
	for i := 0; i < 12; i++ {
		tab := a.tabs.add(nil)
		tab.title = fmt.Sprintf("Tab %02d", i)
	}
	tester := ui.NewTester(func(c *ui.Context) { a.tabBar(c) }, 360, 40)
	tester.SetTitleBar(ui.TitleBar{Height: 36, Left: 78})
	a.revealActiveTab = true
	tester.Frame()
	plus, _ := tester.Find("新建标签页")
	last, ok := tester.Find("Tab 11")
	if !ok || last.X < 78 || last.X+last.W > plus.X {
		t.Fatalf("new active tab should be visible: last=%+v, found=%v, plus=%+v", last, ok, plus)
	}
}

func TestOnlySelectedTabShowsCloseButton(t *testing.T) {
	var a Browser
	first := a.tabs.add(nil)
	first.title = "Tab 1"
	second := a.tabs.add(nil)
	second.title = "Tab 2"
	third := a.tabs.add(nil)
	third.title = "Tab 3"
	a.tabs.selectTab(second)
	tester := ui.NewTester(func(c *ui.Context) { a.tabBar(c) }, 800, 40)
	tester.SetTitleBar(ui.TitleBar{Height: 36, Left: 78})
	assertCloseInside := func(tabTitle, nextTitle string) {
		t.Helper()
		tab, okTab := tester.Find(tabTitle)
		next, okNext := tester.Find(nextTitle)
		closeButton, okClose := tester.Find("关闭标签页")
		if !okTab || !okNext || !okClose || closeButton.X < tab.X+tab.W || closeButton.X+closeButton.W > next.X {
			t.Fatalf("close button should be within selected %s: tab=%+v close=%+v next=%+v", tabTitle, tab, closeButton, next)
		}
		count := 0
		for _, text := range tester.Texts() {
			if text == "×" {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("want one close button for selected tab, got %d", count)
		}
	}
	assertCloseInside("Tab 2", "Tab 3")
	a.tabs.selectTab(first)
	tester.Frame()
	assertCloseInside("Tab 1", "Tab 2")
}

func TestSelectedTabConnectsToContentAtBaseline(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			var a Browser
			for _, title := range []string{"Tab 1", "Tab 2", "Tab 3"} {
				tab := a.tabs.add(nil)
				tab.title = title
			}
			a.tabs.selectTab(a.tabs.items[1])
			tester := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Fill().Background(c.Theme().Background).Children(func() { a.tabBar(c) })
			}, 800, 60)
			tester.SetTitleBar(ui.TitleBar{Height: 36, Left: 78})
			tester.SetDark(dark)
			inactive, _ := tester.Find("Tab 1")
			active, _ := tester.Find("Tab 2")
			img := tester.Image()
			xInactive := int(inactive.X + inactive.W/2)
			xActive := int(active.X + active.W/2)
			if img.RGBAAt(xInactive, 35) == img.RGBAAt(xInactive, 37) {
				t.Fatal("inactive tabs should have a bottom baseline")
			}
			if img.RGBAAt(xActive, 35) != img.RGBAAt(xActive, 37) {
				t.Fatal("selected tab should connect to the content below the baseline")
			}
		})
	}
}

func TestAddressBarAppearsOnlyForNewTabs(t *testing.T) {
	var a Browser
	newTab := a.tabs.add(nil)
	tester := ui.NewTester(a.View, 800, 600)
	if _, ok := tester.Find("地址栏"); !ok {
		t.Fatal("new tab should show an address bar")
	}
	newTab.startPageLoad("https://example.com", false)
	newTab.finishPageLoad("https://example.com")
	tester.Frame()
	if _, ok := tester.Find("地址栏"); ok {
		t.Fatal("opened tab should hide its address bar")
	}
	a.tabs.add(nil)
	tester.Frame()
	if _, ok := tester.Find("地址栏"); !ok {
		t.Fatal("another new tab should show its address bar")
	}
	a.tabs.selectTab(newTab)
	tester.Frame()
	if _, ok := tester.Find("地址栏"); ok {
		t.Fatal("switching back to an opened tab should keep the bar hidden")
	}
}

func TestSelectedTabTitleHasVisibleContrast(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			var a Browser
			for _, title := range []string{"Tab 1", "Selected", "Tab 3"} {
				tab := a.tabs.add(nil)
				tab.title = title
			}
			a.tabs.selectTab(a.tabs.items[1])
			tester := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Fill().Background(c.Theme().Background).Children(func() { a.tabBar(c) })
			}, 800, 60)
			tester.SetTitleBar(ui.TitleBar{Height: 36, Left: 78})
			tester.SetDark(dark)
			title, ok := tester.Find("Selected")
			if !ok {
				t.Fatal("selected title is missing")
			}
			img := tester.Image()
			background := img.RGBAAt(int(title.X+title.W/2), 37)
			var contrast int
			for y := int(title.Y) + 3; y < int(title.Y+title.H)-3; y++ {
				for x := int(title.X) + 3; x < int(title.X+title.W)-3; x++ {
					if img.RGBAAt(x, y) != background {
						contrast++
					}
				}
			}
			if contrast == 0 {
				t.Fatal("selected title is invisible against its background")
			}
		})
	}
}
