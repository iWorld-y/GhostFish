//go:build !darwin

package desktop

// installWindowDrag 在其他平台不启用窗口拖动快捷键。
func (a *Controller) installWindowDrag() error { return nil }
