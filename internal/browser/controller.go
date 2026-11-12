// Package browser 管理标签页、网页导航和网站认证窗口。
package browser

import (
	"time"

	"github.com/egoist/mygo"
)

// Browser 持有浏览器运行状态，字段与回调均在主线程访问。
type Browser struct {
	win                           *mygo.Window
	tabs                          tabSet
	focusAddress, revealActiveTab bool
	pollTimer                     *time.Timer
	pollGeneration                uint64
	authHooks                     *authHooks
	openSettings                  func()
}

// New 将浏览器连接到主窗口，并接收设置入口动作。
func New(win *mygo.Window, openSettings func()) *Browser {
	return &Browser{win: win, openSettings: openSettings}
}

// Start 创建首个标签页并接入窗口生命周期。
func (a *Browser) Start() error {
	_, err := a.createTab()
	if err != nil {
		return err
	}
	a.win.OnShow(a.startAddressPolling)
	a.win.OnHide(a.stopAddressPolling)
	a.win.OnClosed(func() {
		a.stopAddressPolling()
		for _, item := range a.tabs.items {
			a.cancelTabAuth(item)
			item.beginNavigation()
		}
	})
	return nil
}

// FocusAddress 在空白首标签显示后聚焦地址输入。
func (a *Browser) FocusAddress() {
	if tab := a.tabs.active; tab != nil && tab.showsAddress() {
		mygo.App.Focus()
		a.win.Focus()
		a.focusAddress = true
		a.win.Invalidate()
	}
}

// LoadStartup 在系统控制入口注册后加载启动网址。
func (a *Browser) LoadStartup(target string) {
	if target != "" {
		a.startNavigation(a.tabs.active, target, true)
	}
}
