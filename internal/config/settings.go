package config

import (
	"fmt"
	"strconv"
	"strings"
)

// ShortcutBinding 描述一个主键和它需要的修饰键。
type ShortcutBinding struct {
	Key     string `json:"key"`
	Command bool   `json:"command"`
	Control bool   `json:"control"`
	Alt     bool   `json:"alt"`
	Shift   bool   `json:"shift"`
}

// DragModifiers 是移动窗口所需的修饰键组合。
type DragModifiers struct {
	Command bool `json:"command"`
	Control bool `json:"control"`
	Option  bool `json:"option"`
	Shift   bool `json:"shift"`
}

// valid 判断是否至少选择了一个修饰键。
func (m DragModifiers) Valid() bool {
	return m.Command || m.Control || m.Option || m.Shift
}

// matches 判断当前修饰键是否与非空的拖动组合完全一致。
func (m DragModifiers) Matches(held DragModifiers) bool {
	return m.Valid() && m == held
}

// Settings 保存不透明度范围和按动作顺序排列的快捷键。
type Settings struct {
	MinOpacity    int                `json:"min_opacity"`
	MaxOpacity    int                `json:"max_opacity"`
	Shortcuts     [4]ShortcutBinding `json:"shortcuts"`
	DragModifiers DragModifiers      `json:"drag_modifiers"`
}

const (
	ShortcutDown = iota
	ShortcutUp
	ShortcutThrough
	ShortcutHidden
)

var ShortcutLabels = [4]string{"降低不透明度", "提高不透明度", "切换穿透", "隐藏 / 显示"}

// 不透明度上下限允许的整数百分比范围。默认范围也是整个范围：
// 首次启动和旧状态文件里没有设置时，用户可以直接调到 0%。
const (
	SettingsMinOpacity = 0
	SettingsMaxOpacity = 100
)

func DefaultSettings() Settings {
	return Settings{
		MinOpacity:    SettingsMinOpacity,
		MaxOpacity:    SettingsMaxOpacity,
		DragModifiers: DragModifiers{Command: true},
		Shortcuts: [4]ShortcutBinding{
			{Key: "-", Command: true, Shift: true},
			{Key: "=", Command: true, Shift: true},
			{Key: "T", Command: true, Shift: true},
			{Key: "H", Command: true, Shift: true},
		},
	}
}

func ValidShortcutKey(key string) bool {
	if len(key) == 1 {
		return key >= "A" && key <= "Z" || key >= "0" && key <= "9" || key == "-" || key == "="
	}
	if key == "Space" {
		return true
	}
	if strings.HasPrefix(key, "F") {
		n, err := strconv.Atoi(key[1:])
		return err == nil && n >= 1 && n <= 12 && key == fmt.Sprintf("F%d", n)
	}
	return false
}

// ValidateSettings 返回第一项无效设置的说明。
func ValidateSettings(s Settings) error {
	if s.MinOpacity < SettingsMinOpacity || s.MaxOpacity > SettingsMaxOpacity || s.MinOpacity > s.MaxOpacity {
		return fmt.Errorf("请输入 0%% 至 100%% 的整数，且下限不能大于上限")
	}
	if !s.DragModifiers.Valid() {
		return fmt.Errorf("窗口拖动：请至少选择一个修饰键")
	}
	seen := make(map[ShortcutBinding]bool, len(s.Shortcuts))
	for i, key := range s.Shortcuts {
		if !key.Command && !key.Control && !key.Alt || !ValidShortcutKey(key.Key) {
			return fmt.Errorf("%s：请选择受支持的主键和 ⌘、Control 或 Option 修饰键", ShortcutLabels[i])
		}
		if seen[key] {
			return fmt.Errorf("%s：快捷键不能重复", ShortcutLabels[i])
		}
		seen[key] = true
	}
	return nil
}

func (s ShortcutBinding) Accelerator() string {
	parts := make([]string, 0, 5)
	if s.Command {
		parts = append(parts, "Command")
	}
	if s.Control {
		parts = append(parts, "Control")
	}
	if s.Alt {
		parts = append(parts, "Alt")
	}
	if s.Shift {
		parts = append(parts, "Shift")
	}
	return strings.Join(append(parts, s.Key), "+")
}

func (s ShortcutBinding) Display() string {
	var parts []string
	if s.Control {
		parts = append(parts, "⌃")
	}
	if s.Alt {
		parts = append(parts, "⌥")
	}
	if s.Shift {
		parts = append(parts, "⇧")
	}
	if s.Command {
		parts = append(parts, "⌘")
	}
	return strings.Join(append(parts, s.Key), " ")
}

func (s Settings) ClampOpacity(value float64) float64 {
	return Round2(max(float64(s.MinOpacity)/100, min(float64(s.MaxOpacity)/100, value)))
}
