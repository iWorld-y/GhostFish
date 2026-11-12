package desktop

import (
	"fmt"
	"os"

	cfg "ghostfish/internal/config"

	"github.com/egoist/mygo"
)

// hotkey 是一个全局快捷键和它触发的动作。
type hotkey struct {
	acc string
	// label 是给人看的中文名字，注册失败时指向具体动作。
	label string
	// repeat 为真时按住不放会连续触发，调透明度要，切换类不要。
	repeat bool
	fn     func()
}

// hotkeys 定义跨平台默认的全局快捷键。
func (a *Controller) hotkeys() []hotkey {
	return []hotkey{
		{"CmdOrCtrl+Shift+-", "降低不透明度", true, func() { a.changeOpacity(-cfg.OpacityStep) }},
		{"CmdOrCtrl+Shift+=", "提高不透明度", true, func() { a.changeOpacity(cfg.OpacityStep) }},
		{"CmdOrCtrl+Shift+T", "切换点击穿透", false, a.toggleClickThrough},
		{"CmdOrCtrl+Shift+H", "隐藏/显示窗口", false, a.ToggleHidden},
	}
}

// registerHotkeys 逐个注册，并按动作记录注册结果。
func (a *Controller) registerHotkeys() [4]bool {
	var registered [4]bool
	for i, h := range a.hotkeys() {
		register := mygo.GlobalShortcut.Register
		if h.repeat {
			register = registerRepeatShortcut
		}
		if err := register(h.acc, h.fn); err != nil {
			fmt.Fprintf(os.Stderr, "ghostfish: 快捷键 %s（%s）注册失败：%v\n", h.acc, h.label, err)
			continue
		}
		registered[i] = true
	}
	return registered
}

// shortcutAvailable 报告对应动作的已注册快捷键是否可用。
func (a *Controller) shortcutAvailable(index int) bool {
	if a.keys != nil {
		return a.keys.registered[index]
	}
	if a.registered != nil {
		return a.registered[index]
	}
	// 没有实际注册器的状态测试沿用窗口动作的默认行为。
	return true
}
