package desktop

// 窗口属性的写入都经过这里，并在 Controller 上留下最近一次的值。
// 测试无法创建真实窗口，而穿透、可见和不透明度都必须能在没有窗口时验证恢复路径。

// setIgnoreMouse 设置窗口是否忽略鼠标，并记录结果。
func (a *Controller) setIgnoreMouse(ignore bool) {
	a.ignoringMouse = ignore
	if a.win != nil {
		a.win.SetIgnoreMouseEvents(ignore)
	}
}

// windowShown 报告窗口是否可见；没有窗口时用最后一次设置的结果。
func (a *Controller) windowShown() bool {
	if a.win != nil {
		return a.win.IsVisible()
	}
	return a.visible
}

// showWindow 显示窗口并记录可见状态。
func (a *Controller) showWindow() {
	a.visible = true
	defer a.syncTrayMenu()
	if a.win != nil {
		a.win.Show()
	}
}

// hideWindow 隐藏窗口并记录可见状态。
func (a *Controller) hideWindow() {
	a.visible = false
	defer a.syncTrayMenu()
	if a.win != nil {
		a.win.Hide()
	}
}

// setOpacity 设置窗口不透明度，并记录最近一次生效的值。
func (a *Controller) setOpacity(value float64) {
	a.opacity = value
	if a.win != nil {
		a.win.SetOpacity(value)
	}
}
