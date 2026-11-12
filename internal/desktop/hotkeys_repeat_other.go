//go:build !darwin

package desktop

import (
	cfg "ghostfish/internal/config"

	"github.com/egoist/mygo"
)

// registerRepeatShortcut 在其他平台沿用框架的快捷键行为。
func registerRepeatShortcut(acc string, fn func()) error {
	return mygo.GlobalShortcut.Register(acc, fn)
}

// 仅供共同代码编译；其他平台继续使用原有快捷键。
func registerConfiguredShortcut(_ int, binding cfg.ShortcutBinding, fn func()) (func(), error) {
	acc := binding.Accelerator()
	if err := mygo.GlobalShortcut.Register(acc, fn); err != nil {
		return nil, err
	}
	return func() { mygo.GlobalShortcut.Unregister(acc) }, nil
}
