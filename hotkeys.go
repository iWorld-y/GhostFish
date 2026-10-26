package main

import (
	"fmt"
	"os"

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

// hotkeys 是本版的全部控制入口。窗口穿透或隐藏时，用户只剩这些键可用。
func (a *app) hotkeys() []hotkey {
	return []hotkey{
		{"CmdOrCtrl+Shift+-", "降低不透明度", true, func() { a.changeOpacity(-opacityStep) }},
		{"CmdOrCtrl+Shift+=", "提高不透明度", true, func() { a.changeOpacity(opacityStep) }},
		{"CmdOrCtrl+Shift+T", "切换点击穿透", false, a.toggleClickThrough},
		{"CmdOrCtrl+Shift+H", "隐藏/显示窗口", false, a.toggleHidden},
	}
}

// registerHotkeys 逐个注册，返回成功的数量；失败的键位写到终端，
// 因为本版没有菜单栏图标可以挂提示。
func (a *app) registerHotkeys() int {
	registered := 0
	for _, h := range a.hotkeys() {
		register := mygo.GlobalShortcut.Register
		if h.repeat {
			register = mygo.GlobalShortcut.RegisterRepeat
		}
		if err := register(h.acc, h.fn); err != nil {
			fmt.Fprintf(os.Stderr, "ghostfish: 快捷键 %s（%s）注册失败：%v\n", h.acc, h.label, err)
			continue
		}
		registered++
	}
	if registered == 0 {
		fmt.Fprintln(os.Stderr, "ghostfish: 警告：没有一个快捷键注册成功，你将无法切换穿透或隐藏窗口")
	}
	return registered
}

// resolveStartState 决定启动时真正生效的状态：没有任何快捷键可用时，
// 用户无法自己退出穿透，所以不允许带着穿透状态启动。
func resolveStartState(registered int, st state) (state, bool) {
	if registered == 0 && st.ClickThrough {
		st.ClickThrough = false
		return st, true
	}
	return st, false
}
