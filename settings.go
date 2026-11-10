package main

import (
	"fmt"
	"strconv"
	"strings"
)

// shortcutBinding 描述一个主键和它需要的修饰键。
type shortcutBinding struct {
	Key     string `json:"key"`
	Command bool   `json:"command"`
	Control bool   `json:"control"`
	Alt     bool   `json:"alt"`
	Shift   bool   `json:"shift"`
}

// settings 保存不透明度范围和按动作顺序排列的快捷键。
type settings struct {
	MinOpacity int                `json:"min_opacity"`
	MaxOpacity int                `json:"max_opacity"`
	Shortcuts  [4]shortcutBinding `json:"shortcuts"`
}

const (
	shortcutDown = iota
	shortcutUp
	shortcutThrough
	shortcutHidden
)

var shortcutLabels = [4]string{"降低不透明度", "提高不透明度", "切换穿透", "隐藏 / 显示"}

// 不透明度上下限允许的整数百分比范围。默认范围也是整个范围：
// 首次启动和旧状态文件里没有设置时，用户可以直接调到 0%。
const (
	settingsMinOpacity = 0
	settingsMaxOpacity = 100
)

func defaultSettings() settings {
	return settings{
		MinOpacity: settingsMinOpacity,
		MaxOpacity: settingsMaxOpacity,
		Shortcuts: [4]shortcutBinding{
			{Key: "-", Command: true, Shift: true},
			{Key: "=", Command: true, Shift: true},
			{Key: "T", Command: true, Shift: true},
			{Key: "H", Command: true, Shift: true},
		},
	}
}

func validShortcutKey(key string) bool {
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

// validateSettings 返回第一项无效设置的说明。
func validateSettings(s settings) error {
	if s.MinOpacity < settingsMinOpacity || s.MaxOpacity > settingsMaxOpacity || s.MinOpacity > s.MaxOpacity {
		return fmt.Errorf("请输入 0%% 至 100%% 的整数，且下限不能大于上限")
	}
	seen := make(map[shortcutBinding]bool, len(s.Shortcuts))
	for i, key := range s.Shortcuts {
		if !key.Command && !key.Control && !key.Alt || !validShortcutKey(key.Key) {
			return fmt.Errorf("%s：请选择受支持的主键和 ⌘、Control 或 Option 修饰键", shortcutLabels[i])
		}
		if seen[key] {
			return fmt.Errorf("%s：快捷键不能重复", shortcutLabels[i])
		}
		seen[key] = true
	}
	return nil
}

func (s shortcutBinding) accelerator() string {
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

func (s shortcutBinding) display() string {
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

func (s settings) clampOpacity(value float64) float64 {
	return round2(max(float64(s.MinOpacity)/100, min(float64(s.MaxOpacity)/100, value)))
}
