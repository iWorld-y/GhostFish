package desktop

import (
	"fmt"
	"os"

	cfg "ghostfish/internal/config"
)

// reportTrayFailure 在窗口显示并处理初始焦点后提示托盘创建失败。
func (a *Controller) reportTrayFailure(err error) {
	if err != nil {
		a.warnTrayUnavailable(err.Error())
	}
}

func (a *Controller) changeOpacity(delta float64) {
	next := cfg.Round2(a.st.Opacity + delta)
	if a.st.Settings != nil {
		next = a.st.Settings.ClampOpacity(next)
	} else {
		next = cfg.ClampOpacity(next)
	}
	if next == a.st.Opacity {
		return
	}
	a.st.Opacity = next
	a.setOpacity(next)
	a.save()
	reportState(a.st)
}

func (a *Controller) toggleClickThrough() {
	if !a.st.ClickThrough && !a.canClickThrough() {
		fmt.Fprintln(os.Stderr, "ghostfish: 托盘和穿透快捷键均不可用，无法开启穿透")
		return
	}
	a.st.ClickThrough = !a.st.ClickThrough
	a.applyClickThrough()
	a.save()
	reportState(a.st)
}

// ToggleHidden 按可用恢复入口切换窗口显示状态。
func (a *Controller) ToggleHidden() {
	if a.windowShown() && !a.trayAvailable && !a.shortcutAvailable(cfg.ShortcutHidden) {
		return
	}
	if a.windowShown() {
		a.hideWindow()
		fmt.Fprintln(os.Stderr, "ghostfish: 窗口已隐藏，可通过托盘或快捷键恢复")
		return
	}
	a.showWindow()
	fmt.Fprintln(os.Stderr, "ghostfish: 窗口已恢复")
}

func (a *Controller) save() {
	if err := cfg.SaveState(a.statePath, a.st); err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: 保存状态失败：%v\n", err)
	}
}

// reportState 把当前状态写到终端。
func reportState(st cfg.State) {
	through := "关"
	if st.ClickThrough {
		through = "开"
	}
	fmt.Fprintf(os.Stderr, "ghostfish: 不透明度 %d%% · 穿透 %s\n", int(st.Opacity*100+0.5), through)
}
