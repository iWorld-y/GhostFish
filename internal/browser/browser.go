package browser

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

const addressPollInterval = 500 * time.Millisecond

// createTab 创建独立网页视图并选中新的空白标签页。
func (a *Browser) createTab() (*browserTab, error) {
	view, err := a.win.NewWebView(mygo.WebViewOptions{})
	if err != nil {
		return nil, err
	}
	return a.addTab(view), nil
}

// addTab 接入网页视图并将其选为当前标签。
func (a *Browser) addTab(view *mygo.WebView) *browserTab {
	previous := a.tabs.active
	tab := a.tabs.add(view)
	a.watchTabView(tab, view)
	a.installAuthHandler(tab, view, "")
	a.focusAddress = true
	a.revealActiveTab = true
	a.win.Invalidate()
	a.syncAuthTab(previous)
	return tab
}

// watchTabView 只处理当前网页视图的导航、失败和标题事件。
func (a *Browser) watchTabView(tab *browserTab, view *mygo.WebView) {
	view.Page().OnWillNavigate(func(event *mygo.NavigateEvent) {
		if tab.view == view {
			prepared := tab.authPrepared
			a.cancelTabAuth(tab)
			tab.pageWillNavigate(event.UserInitiated)
			if !event.UserInitiated {
				a.saveVerifiedAuth(tab, prepared)
			}
			tab.authURL = event.URL
			tab.authLoading = true
			// 旧版 WebKitGTK 的导航事件可能来自子框架，不能据此确认主文档。
			tab.authMainFrame = runtime.GOOS == "darwin" || runtime.GOOS == "windows"
			a.installAuthHandler(tab, view, event.URL)
		}
	})
	view.Page().OnDidNavigate(func(rawURL string) {
		if tab.view != view {
			return
		}
		tab.authLoading = false
		tab.authMainFrame = false
		changed := tab.commitPageLoad(rawURL)
		if changed && a.tabs.active == tab {
			a.focusAddress = false
		}
		if tab.observeURL(rawURL) || changed {
			a.win.Invalidate()
		}
	})
	view.Page().OnDidFinishLoad(func() {
		if tab.view == view && tab.finishPageLoad(view.Page().URL()) {
			a.win.Invalidate()
		}
		a.finishAuthLoad(tab, view, false)
	})
	view.Page().OnDidFailLoad(func(err *mygo.LoadError) {
		if tab.failPageLoad(view, fmt.Sprintf("载入网址失败：%s", err.Description)) {
			if a.tabs.active == tab {
				a.focusAddress = true
			}
			a.win.Invalidate()
		}
		a.finishAuthLoad(tab, view, true)
	})
	view.Page().OnPageTitleUpdated(func(event *mygo.TitleEvent) {
		if tab.view != view {
			return
		}
		title := strings.TrimSpace(event.Title)
		if title == "" {
			title = "新标签页"
		}
		tab.title = title
		a.win.Invalidate()
	})
}

// replaceTabView 使未打开网页的旧加载回调与新请求隔离。
func (a *Browser) replaceTabView(tab *browserTab) error {
	view, err := a.win.NewWebView(mygo.WebViewOptions{})
	if err != nil {
		return err
	}
	old := tab.view
	tab.view = view
	a.watchTabView(tab, view)
	a.installAuthHandler(tab, view, "")
	old.Destroy()
	a.win.Invalidate()
	return nil
}

// selectTab 显示已有标签页，并同步其当前网址。
func (a *Browser) selectTab(tab *browserTab) {
	previous := a.tabs.active
	if a.tabs.active == tab || !a.tabs.selectTab(tab) {
		return
	}
	tab.setFocused(false)
	if tab.openedPage {
		tab.updateURL(tab.view.Page().URL())
	}
	a.focusAddress = tab.showsAddress()
	a.revealActiveTab = true
	a.win.Invalidate()
	a.syncAuthTab(previous)
}

// closeTab 释放网页视图；关闭最后一个标签页时关闭窗口。
func (a *Browser) closeTab(tab *browserTab) {
	previous := a.tabs.active
	a.cancelTabAuth(tab)
	wasActive := a.tabs.active == tab
	removed, last := a.tabs.remove(tab)
	if !removed {
		return
	}
	view := tab.view
	tab.view = nil
	view.Destroy()
	if last {
		a.win.Close()
		return
	}
	if wasActive {
		active := a.tabs.active
		active.setFocused(false)
		if active.openedPage {
			active.updateURL(active.view.Page().URL())
		}
		a.focusAddress = active.showsAddress()
		a.revealActiveTab = true
	}
	a.win.Invalidate()
	a.syncAuthTab(previous)
}

// submitAddress 校验地址，并在后台准备认证后加载当前标签页。
func (a *Browser) submitAddress(tab *browserTab) {
	a.cancelTabAuth(tab)
	tab.beginNavigation()
	tab.pending = false
	if tab.cancelPageLoad() && tab.view != nil {
		if !tab.openedPage || tab.openingFirstPage {
			if err := a.replaceTabView(tab); err != nil {
				tab.view.Page().Stop()
				tab.errorText = fmt.Sprintf("重试失败：%v", err)
				return
			}
		} else {
			tab.view.Page().Stop()
		}
	}
	if strings.TrimSpace(tab.input) == "" {
		tab.errorText = ""
		return
	}
	rawURL, err := parseURL(tab.input)
	if err != nil {
		tab.errorText = err.Error()
		return
	}
	tab.updateURL(tab.view.Page().URL())
	tab.input = rawURL
	tab.editing = true
	tab.errorText = ""
	a.startNavigation(tab, rawURL, false)
}

// startNavigation 在后台准备认证，始终在主线程检查标签与导航后才加载。
func (a *Browser) startNavigation(tab *browserTab, rawURL string, startup bool) {
	a.cancelTabAuth(tab)
	generation := tab.beginNavigation()
	view := tab.view
	tab.authURL = rawURL
	tab.authSourceURL = rawURL
	tab.authLoading = true
	tab.pending = true
	tab.startPageLoad(rawURL, startup)
	ctx, cancel := context.WithCancel(context.Background())
	tab.cancelNavigation = cancel
	go func() {
		defer cancel()
		prepared := a.prepareAuth(ctx, rawURL)
		if ctx.Err() != nil {
			return
		}
		a.dispatchAuth(func() {
			if ctx.Err() != nil || !a.tabs.accepts(tab, generation) || tab.view != view {
				return
			}
			tab.pending = false
			tab.cancelNavigation = nil
			tab.authPrepared = prepared
			a.installAuthHandler(tab, view, rawURL)
			tab.authMainFrame = true
			if err := a.loadAuthURL(tab, rawURL); err != nil {
				tab.authLoading = false
				tab.authMainFrame = false
				tab.failPageLoad(view, fmt.Sprintf("载入网址失败：%v", err))
			}
			if a.win != nil {
				a.win.Invalidate()
			}
		})
	}()
}

// startAddressPolling 补充平台未发出事件的同文档网址变化。
func (a *Browser) startAddressPolling() {
	a.stopAddressPolling()
	generation := a.pollGeneration
	a.scheduleAddressPoll(generation)
}

// scheduleAddressPoll 只在窗口可见时同步当前标签页的网址。
func (a *Browser) scheduleAddressPoll(generation uint64) {
	a.pollTimer = time.AfterFunc(addressPollInterval, func() {
		mygo.RunOnMain(func() {
			if generation != a.pollGeneration || !a.win.IsVisible() {
				return
			}
			if tab := a.tabs.active; tab != nil && tab.observeURL(tab.view.Page().URL()) {
				a.win.Invalidate()
			}
			a.scheduleAddressPoll(generation)
		})
	})
}

// stopAddressPolling 取消窗口隐藏或关闭后的地址读取。
func (a *Browser) stopAddressPolling() {
	a.pollGeneration++
	if a.pollTimer != nil {
		a.pollTimer.Stop()
		a.pollTimer = nil
	}
}
