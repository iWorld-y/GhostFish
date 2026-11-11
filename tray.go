package main

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"

	"github.com/egoist/mygo"
)

//go:embed assets/logo/ghostfish-tray.png
var trayIcon []byte

//go:embed assets/logo/ghostfish-tray-linux.png
var trayLinuxIcon []byte

// trayControls 持有系统托盘及其需要随窗口状态更新的菜单项。
type trayControls struct {
	icon    *mygo.Tray
	through *mygo.MenuItem
	visible *mygo.MenuItem
	stop    func()
}

// createTray 创建托盘；Linux 还需要确认桌面托盘宿主可用。
func (a *app) createTray(newIcon func(mygo.TrayOptions) (*mygo.Tray, error)) error {
	controls := &trayControls{}
	controls.through = &mygo.MenuItem{
		Label: "鼠标穿透", Type: mygo.MenuItemCheckbox,
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.recordTrayAction()
			a.toggleClickThrough()
		},
	}
	controls.visible = &mygo.MenuItem{
		Label: "隐藏窗口",
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.recordTrayAction()
			a.toggleHidden()
		},
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		controls.through,
		controls.visible,
		mygo.Separator(),
		{Label: "退出", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
	iconImage, template := trayIcon, true
	if runtime.GOOS == "linux" {
		// AppIndicator 不按面板主题着色，用黑白底板兼顾浅色和深色背景。
		iconImage, template = trayLinuxIcon, false
	}
	icon, err := newIcon(mygo.TrayOptions{
		ID: "ghostfish", Icon: iconImage, IconIsTemplate: template,
		ToolTip: "GhostFish", Menu: menu,
	})
	if err != nil {
		return err
	}
	controls.icon = icon
	a.tray = controls
	var stopped atomic.Bool
	status, stop := watchTrayHost(&a.trayEvidence, func(status trayHostStatus, evidence uint64) {
		mygo.RunOnMain(func() {
			if !stopped.Load() {
				a.setTrayHostObservation(status, evidence)
			}
		})
	})
	controls.stop = func() { stopped.Store(true); stop() }
	a.trayAvailable = status == trayHostAvailable
	a.trayPending = status == trayHostUnknown
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { controls.stop() })
	a.syncTrayMenu()
	if !a.trayAvailable && !a.trayPending {
		return fmt.Errorf("无法确认系统托盘宿主可用")
	}
	return nil
}

// syncTrayMenu 按应用状态更新菜单，不保存第二份窗口状态。
func (a *app) syncTrayMenu() {
	if a.tray == nil {
		return
	}
	a.tray.through.SetChecked(a.st.ClickThrough)
	if a.settingsOpen && a.st.ClickThrough {
		a.tray.through.SetSubtitle("设置打开时暂时暂停")
	} else if a.trayPending && a.st.ClickThrough && !a.shortcutAvailable(shortcutThrough) {
		a.tray.through.SetSubtitle("正在确认托盘")
	} else {
		a.tray.through.SetSubtitle("")
	}
	if a.windowShown() {
		a.tray.visible.SetLabel("隐藏窗口")
	} else {
		a.tray.visible.SetLabel("显示窗口")
	}
}

// shortcutAvailable 报告对应动作的已注册快捷键是否可用。
func (a *app) shortcutAvailable(index int) bool {
	if a.keys != nil {
		return a.keys.registered[index]
	}
	if a.registered != nil {
		return a.registered[index]
	}
	// 没有实际注册器的状态测试沿用窗口动作的默认行为。
	return true
}

// recordTrayAction 将可点击菜单作为最新的托盘可用证据。
func (a *app) recordTrayAction() {
	a.trayEvidence.Add(1)
	a.setTrayAvailable(true)
}

// setTrayHostObservation 丢弃菜单操作之前开始的旧探测结果。
func (a *app) setTrayHostObservation(status trayHostStatus, evidence uint64) {
	if evidence == a.trayEvidence.Load() {
		a.setTrayHostStatus(status)
	}
}

// setTrayHostStatus 在短暂查询失败时保留可用证据，连续失败时保护窗口。
func (a *app) setTrayHostStatus(status trayHostStatus) {
	switch status {
	case trayHostAvailable:
		a.setTrayAvailable(true)
	case trayHostUnavailable:
		a.setTrayAvailable(false)
	case trayHostUnknown:
		a.trayUnknowns++
		if a.trayUnknowns >= 3 {
			a.setTrayAvailable(false)
		}
	}
}

// setTrayAvailable 在托盘宿主丢失时按各动作的恢复入口保护窗口。
func (a *app) setTrayAvailable(available bool) {
	wasPending := a.trayPending
	a.trayPending = false
	if available {
		a.trayUnknowns = 0
	}
	if a.trayAvailable == available && !wasPending {
		return
	}
	a.trayAvailable = available
	if !available {
		if a.st.ClickThrough && !a.shortcutAvailable(shortcutThrough) {
			a.st.ClickThrough = false
			a.applyClickThrough()
			a.save()
		}
		if !a.windowShown() && !a.shortcutAvailable(shortcutHidden) {
			a.showWindow()
		}
		a.warnTrayUnavailable("托盘宿主或图标不可用")
	} else if wasPending {
		a.applyClickThrough()
	}
	a.syncTrayMenu()
}

// warnTrayUnavailable 在没有终端和窗口透明时也能显示错误。
func (a *app) warnTrayUnavailable(reason string) {
	message := "托盘不可用：" + reason + "。请检查系统托盘或快捷键。若窗口完全透明，请按提高不透明度快捷键；若快捷键不可用或设置的范围上限为 0%，请退出后修改或重置状态文件。"
	fmt.Fprintln(os.Stderr, "ghostfish:", message)
	if a.trayWarning != nil {
		a.trayWarning(message)
	} else if a.win != nil {
		go mygo.Dialog.Error("GhostFish 托盘不可用", message)
	}
}
