package main

import (
	"fmt"
	"os"

	"github.com/egoist/mygo/ui"
)

// view 绘制标签栏、地址栏和当前标签页的网页。
func (a *app) view(c *ui.Context) {
	tab := a.tabs.active
	if tab == nil {
		return
	}
	ui.Column(c).Fill().Background(c.Theme().Background).AlignItems(ui.Stretch).Children(func() {
		a.tabBar(c)
		if tab.showsAddress() {
			a.addressBar(c, tab)
		}
		ui.WebView(c, tab.view).Grow(1)
	})
}

// tabBar 为平台窗口按钮留出空间，并绘制可滚动的标签页。
func (a *app) tabBar(c *ui.Context) {
	bar := c.TitleBar()
	barHeight := max(bar.Height, 36)
	theme := c.Theme()
	ui.Row(c).Height(barHeight).Padding(0, bar.Right+8, 0, bar.Left+8).Gap(4).AlignItems(ui.End).
		Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - 1, W: r.W, H: 1}, theme.Border, 0)
		}).Children(func() {
		ui.ScrollHorizontal(c).Shrink(1).MinWidth(0).Height(barHeight).AlignItems(ui.End).Children(func() {
			for i, item := range a.tabs.items {
				item := item
				active := item == a.tabs.active
				if !active && (i == 0 || a.tabs.items[i-1] != a.tabs.active) {
					ui.Box(c).Width(1).Height(20).Margin(0, 0, 8, 0).Background(theme.Border)
				}
				row := ui.Row(c.Key(item.id)).Height(barHeight - 4).AlignItems(ui.Center)
				if active {
					row.Background(theme.Background).BorderWidth(1, 1, 0, 1).BorderColor(theme.Border).Radius(8, 8, 0, 0)
					if a.revealActiveTab {
						row.ScrollIntoView()
						a.revealActiveTab = false
					}
				}
				row.Children(func() {
					ui.ButtonBase(c).Role(ui.RoleTab).Selected(active).Label(item.title).Tooltip(item.title).
						Padding(0, 12).Height(barHeight - 6).MaxWidth(160).Background(theme.Background).OnClick(func() { a.selectTab(item) }).
						Children(func() { ui.Text(c, item.title).SingleLine().TextColor(theme.Text) })
					if active {
						ui.ButtonBase(c).Width(28).Height(barHeight - 6).Label("关闭标签页").
							OnClick(func() { a.closeTab(item) }).Children(func() { ui.Text(c, "×") })
					}
				})
			}
		})
		ui.ButtonBase(c).Width(32).Height(barHeight - 4).Label("新建标签页").OnClick(func() {
			if _, err := a.createTab(); err != nil {
				fmt.Fprintf(os.Stderr, "ghostfish: 创建标签页失败：%v\n", err)
			}
		}).Children(func() { ui.Text(c, "+") })
		ui.Box(c).Grow(1).MinWidth(32).Height(barHeight).DragWindow()
	})
}

// addressBar 绘制并管理当前标签页的地址输入。
func (a *app) addressBar(c *ui.Context, tab *browserTab) {
	ui.Row(c).Padding(4, 8).Gap(8).AlignItems(ui.Center).Children(func() {
		input := ui.TextInput(c.Key(tab.id), &tab.input).Grow(1).Placeholder("输入网址").Label("地址栏")
		input.OnChange(func() {
			tab.editing = true
			tab.errorText = ""
		}).OnSubmit(func() { a.submitAddress(tab) })
		focused := input.Focused()
		if a.focusAddress {
			input.Focus()
			a.focusAddress = false
			focused = true
		}
		if tab.setFocused(focused) {
			c.Invalidate()
		}
	})
	if tab.errorText != "" {
		ui.Text(c, tab.errorText).Padding(0, 8, 4)
	}
}
