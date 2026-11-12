package desktop

import "ghostfish/internal/settings"

// settingsFixture 组装真实设置界面与桌面控制器，验证跨组件恢复行为。
type settingsFixture struct {
	*Controller
	dialog *settings.Controller
}

func newSettingsFixture(controls *Controller) *settingsFixture {
	return &settingsFixture{Controller: controls, dialog: settings.New(controls)}
}
