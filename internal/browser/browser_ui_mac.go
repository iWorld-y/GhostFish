package browser

import (
	"fmt"
	"os"

	"github.com/egoist/mygo/ui"
)

var addTabIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>`))
var settingsIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><circle cx="12" cy="12" r="3"/><path d="M12 2.75v2.1m0 14.3v2.1M2.75 12h2.1m14.3 0h2.1M5.46 5.46l1.49 1.49m10.1 10.1 1.49 1.49m0-13.08-1.49 1.49M6.95 17.05l-1.49 1.49"/><circle cx="12" cy="12" r="7.15"/></svg>`))

// macTabBar 根据设计稿绘制完整高度的标签和右侧设置入口。
func (a *Browser) macTabBar(c *ui.Context) {
	bar := c.TitleBar()
	height := max(bar.Height, 36)
	theme := c.Theme()
	ui.Row(c).Height(height).Padding(0, bar.Right+8, 0, bar.Left+8).AlignItems(ui.Center).
		Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - 1, W: r.W, H: 1}, theme.Border, 0)
		}).Children(func() {
		ui.ScrollHorizontal(c).Shrink(1).MinWidth(0).Height(height).AlignItems(ui.Center).Children(func() {
			for i, item := range a.tabs.items {
				item := item
				active := item == a.tabs.active
				row := ui.Row(c.Key(item.id)).Height(height).AlignItems(ui.Center)
				// 当前标签用直角背景和两侧竖线，其他标签只用竖线分隔。
				fill := ui.Color{}
				if active {
					fill = theme.Background
					row.BorderWidth(0, 1, 0, 1).BorderColor(theme.Border)
					if a.revealActiveTab {
						row.ScrollIntoView()
						a.revealActiveTab = false
					}
				} else if i+1 >= len(a.tabs.items) || a.tabs.items[i+1] != a.tabs.active {
					row.BorderWidth(0, 1, 0, 0).BorderColor(theme.Border)
				}
				if row.Hovered() {
					fill = theme.SurfaceHover
				}
				if fill.A > 0 {
					row.Background(fill)
				}
				row.Children(func() {
					// 背景要显式给：滚动容器里的 Selected 标签按钮否则会被画成强调色底。
					ui.ButtonBase(c).Role(ui.RoleTab).Selected(active).Label(item.title).Tooltip(item.title).
						Padding(0, 12).Height(height - 6).MaxWidth(160).Background(fill).OnClick(func() { a.selectTab(item) }).
						Children(func() { ui.Text(c, item.title).SingleLine().TextColor(theme.Text) })
					if active {
						ui.ButtonBase(c).Width(28).Height(height - 6).Center().Label("关闭标签页").OnClick(func() { a.closeTab(item) }).
							Children(func() { ui.Text(c, "×").FontSize(18) })
					}
				})
			}
		})
		plus := ui.ButtonBase(c).Width(32).Height(height).Center().Label("新建标签页").OnClick(func() {
			if _, err := a.createTab(); err != nil {
				fmt.Fprintf(os.Stderr, "ghostfish: 创建标签页失败：%v\n", err)
			}
		})
		if plus.Hovered() {
			plus.Background(theme.SurfaceHover)
		}
		plus.Children(func() { ui.Icon(c, addTabIcon).Size(18, 18) })
		ui.Box(c).Grow(1).MinWidth(32).Height(height).DragWindow()
		gear := ui.ButtonBase(c).Width(36).Height(height).Center().Label("设置").Tooltip("设置").OnClick(a.openSettings)
		if gear.Hovered() {
			gear.Background(theme.SurfaceHover)
		}
		gear.Children(func() { ui.Icon(c, settingsIcon).Size(18, 18) })
	})
}
