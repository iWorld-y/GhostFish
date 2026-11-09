package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

const addressPollInterval = 500 * time.Millisecond

// createTab 创建独立网页视图并选中新的空白标签页。
func (a *app) createTab() (*browserTab, error) {
	view, err := a.win.NewWebView(mygo.WebViewOptions{})
	if err != nil {
		return nil, err
	}
	tab := a.tabs.add(view)
	a.watchTabView(tab, view)
	a.focusAddress = true
	a.revealActiveTab = true
	a.win.Invalidate()
	return tab, nil
}

// watchTabView 只处理当前网页视图的导航、失败和标题事件。
func (a *app) watchTabView(tab *browserTab, view *mygo.WebView) {
	view.Page().OnWillNavigate(func(event *mygo.NavigateEvent) {
		if tab.view == view {
			tab.pageWillNavigate(event.UserInitiated)
		}
	})
	view.Page().OnDidNavigate(func(rawURL string) {
		if tab.view != view {
			return
		}
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
	})
	view.Page().OnDidFailLoad(func(err *mygo.LoadError) {
		if tab.failPageLoad(view, fmt.Sprintf("载入网址失败：%s", err.Description)) {
			if a.tabs.active == tab {
				a.focusAddress = true
			}
			a.win.Invalidate()
		}
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
func (a *app) replaceTabView(tab *browserTab) error {
	view, err := a.win.NewWebView(mygo.WebViewOptions{})
	if err != nil {
		return err
	}
	old := tab.view
	tab.view = view
	a.watchTabView(tab, view)
	old.Destroy()
	a.win.Invalidate()
	return nil
}

// selectTab 显示已有标签页，并同步其当前网址。
func (a *app) selectTab(tab *browserTab) {
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
}

// closeTab 释放网页视图；关闭最后一个标签页时关闭窗口。
func (a *app) closeTab(tab *browserTab) {
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
}

// submitAddress 校验地址，并在后台准备认证后加载当前标签页。
func (a *app) submitAddress(tab *browserTab) {
	generation := tab.beginNavigation()
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
	tab.pending = true
	ctx, cancel := context.WithCancel(context.Background())
	tab.cancelNavigation = cancel
	go func() {
		defer cancel()
		host, cred := prepareAuth(ctx, rawURL)
		mygo.RunOnMain(func() {
			if !a.tabs.accepts(tab, generation) {
				return
			}
			tab.pending = false
			tab.cancelNavigation = nil
			page := tab.view.Page()
			if cred == nil {
				page.SetAuthHandler(nil)
			} else {
				page.SetAuthHandler(authHandler(host, cred))
			}
			tab.startPageLoad(rawURL, false)
			if err := page.LoadURL(rawURL); err != nil {
				tab.failPageLoad(tab.view, fmt.Sprintf("载入网址失败：%v", err))
			}
			a.win.Invalidate()
		})
	}()
}

// startAddressPolling 补充平台未发出事件的同文档网址变化。
func (a *app) startAddressPolling() {
	a.stopAddressPolling()
	generation := a.pollGeneration
	a.scheduleAddressPoll(generation)
}

// scheduleAddressPoll 只在窗口可见时同步当前标签页的网址。
func (a *app) scheduleAddressPoll(generation uint64) {
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
func (a *app) stopAddressPolling() {
	a.pollGeneration++
	if a.pollTimer != nil {
		a.pollTimer.Stop()
		a.pollTimer = nil
	}
}
