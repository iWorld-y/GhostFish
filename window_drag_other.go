//go:build !darwin

package main

// installWindowDrag 在其他平台不启用窗口拖动快捷键。
func (a *app) installWindowDrag() error { return nil }
