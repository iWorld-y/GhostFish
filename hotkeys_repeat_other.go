//go:build !darwin

package main

import "github.com/egoist/mygo"

// registerRepeatShortcut 在其他平台沿用框架的快捷键行为。
func registerRepeatShortcut(acc string, fn func()) error {
	return mygo.GlobalShortcut.Register(acc, fn)
}
