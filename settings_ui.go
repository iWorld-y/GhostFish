package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// settingsDialog 显示设置草稿；关闭时释放录入状态并恢复原快捷键。
func (a *app) settingsDialog(c *ui.Context) {
	if !a.settingsOpen {
		return
	}
	width, height := c.Size()
	a.settingsModalOpen = true
	ui.Modal(c, &a.settingsModalOpen, func() {
		ui.Column(c).Width(min(400, max(160, width-64))).MaxHeight(max(120, height-64)).Gap(12).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "设置").FontSize(18).Bold().Grow(1)
				ui.Button(c, "×").Label("关闭设置").OnClick(a.closeSettings)
			})
			ui.Divider(c)
			ui.Scroll(c).Shrink(1).Children(func() {
				ui.Column(c).FillWidth().Gap(12).Children(func() {
					ui.Text(c, "不透明度范围").Bold()
					ui.Row(c).FillWidth().Gap(16).Children(func() {
						ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
							ui.Text(c, "下限")
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
								input := ui.TextInput(c.Key("min"), &a.settingsMin).Label("不透明度下限").Grow(1).MinWidth(0)
								if a.settingsFocus {
									input.Focus()
									a.settingsFocus = false
								}
								ui.Text(c, "%")
							})
						})
						ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
							ui.Text(c, "上限")
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
								ui.TextInput(c.Key("max"), &a.settingsMax).Label("不透明度上限").Grow(1).MinWidth(0)
								ui.Text(c, "%")
							})
						})
					})
					ui.Text(c, "快捷键").Bold()
					ui.Text(c, shortcutSharingHint).FontSize(12).Wrap().TextColor(c.Theme().TextMuted)
					for i, label := range shortcutLabels {
						i, label := i, label
						ui.Row(c.Key(i)).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
							ui.Text(c, label).Grow(1)
							if a.keys != nil && !a.keys.registered[i] && a.recording < 0 {
								ui.Text(c, "不可用").TextColor(c.Theme().Danger)
							}
							text := a.settingsDraft.Shortcuts[i].display()
							if a.recording == i {
								text = "请按快捷键"
							}
							button := ui.Button(c, text).Label(label + "快捷键").MinWidth(110).
								OnClick(func() { a.beginRecording(i) }).HandleInput(func(event ui.InputEvent) bool {
								if a.recording != i || event.Kind != ui.InputKeyDown {
									return false
								}
								if event.Key == ui.KeyEscape {
									a.endRecording()
									return true
								}
								binding, ok := shortcutFromInput(event)
								if !ok {
									a.settingsError = "请选择受支持的主键和 ⌘、Control 或 Option 修饰键"
									return true
								}
								a.settingsDraft.Shortcuts[i] = binding
								a.settingsError = ""
								a.endRecording()
								return true
							})
							if a.recording == i && a.recordingFocus {
								button.Focus()
								a.recordingFocus = false
							}
						})
						ui.Divider(c)
					}
					if a.restoreError != "" {
						ui.Text(c, a.restoreError).Wrap().TextColor(c.Theme().Danger)
					}
					if a.settingsError != "" {
						ui.Text(c, a.settingsError).TextColor(c.Theme().Danger)
					}
				})
			})
			ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
				ui.Button(c, "取消").OnClick(a.closeSettings)
				ui.PrimaryButton(c, "保存").OnClick(func() {
					if err := a.saveSettings(); err != nil {
						a.settingsError = err.Error()
					}
				})
			})
		})
	})
	// Modal 自己会因 Escape 或点击背板关掉指针，界面之外的状态要在这里收尾。
	if !a.settingsModalOpen && a.settingsOpen {
		a.closeSettings()
	}
}

// shortcutSharingHint 说明 macOS 的快捷键是跨应用共享的，注册成功不代表一定收到按键。
const shortcutSharingHint = "快捷键可能与其他应用共享，或被系统、其他应用拦截；注册成功也不代表本应用一定能收到按键。若按键被拦截，可能无法用快捷键恢复穿透或隐藏，建议选择不会被占用的组合。"

func shortcutFromInput(event ui.InputEvent) (shortcutBinding, bool) {
	key := event.PhysicalKey
	if key == ui.KeyUnknown {
		key = event.Key
	}
	name, ok := inputKeyNames[key]
	binding := shortcutBinding{Key: name, Command: event.Mods&ui.Super != 0, Control: event.Mods&ui.Ctrl != 0, Alt: event.Mods&ui.Alt != 0, Shift: event.Mods&ui.Shift != 0}
	return binding, ok && (binding.Command || binding.Control || binding.Alt)
}

var inputKeyNames = func() map[ui.Key]string {
	keys := map[ui.Key]string{ui.KeyMinus: "-", ui.KeyEqual: "=", ui.KeySpace: "Space"}
	letters := []ui.Key{ui.KeyA, ui.KeyB, ui.KeyC, ui.KeyD, ui.KeyE, ui.KeyF, ui.KeyG, ui.KeyH, ui.KeyI, ui.KeyJ, ui.KeyK, ui.KeyL, ui.KeyM, ui.KeyN, ui.KeyO, ui.KeyP, ui.KeyQ, ui.KeyR, ui.KeyS, ui.KeyT, ui.KeyU, ui.KeyV, ui.KeyW, ui.KeyX, ui.KeyY, ui.KeyZ}
	for i, key := range letters {
		keys[key] = string(rune('A' + i))
	}
	digits := []ui.Key{ui.Key0, ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9}
	for i, key := range digits {
		keys[key] = fmt.Sprint(i)
	}
	functions := []ui.Key{ui.KeyF1, ui.KeyF2, ui.KeyF3, ui.KeyF4, ui.KeyF5, ui.KeyF6, ui.KeyF7, ui.KeyF8, ui.KeyF9, ui.KeyF10, ui.KeyF11, ui.KeyF12}
	for i, key := range functions {
		keys[key] = fmt.Sprintf("F%d", i+1)
	}
	return keys
}()
