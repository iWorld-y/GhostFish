package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
)

const (
	defaultOpacity = 0.85
	// minOpacity 和 maxOpacity 是 Windows 和 Linux 的固定范围；
	// macOS 的设置在 settingsSupported 下可以调到 0。
	minOpacity  = 0.20
	maxOpacity  = 1.00
	opacityStep = 0.10
	// 浮点比较的容差，读写和算步进都靠它避免 0.8500000001 这种噪声。
	opacityEpsilon = 1e-9
)

// allowedMinOpacity 是不透明度允许的最小值：macOS 用户可以调到 0%，
// 其他平台沿用原来的 20%。
func allowedMinOpacity() float64 {
	if settingsSupported {
		return 0
	}
	return minOpacity
}

// settingsSupported 决定状态文件是否读写新增的设置：本次只有 macOS 提供设置界面，
// 其他平台继续只看不透明度和穿透状态。单独成变量，便于测试覆盖两条读取路径。
var settingsSupported = runtime.GOOS == "darwin"

// state 是跨重启保留的自定义状态。位置和尺寸由窗口自己的 StateKey 负责，不在这里。
type state struct {
	Opacity      float64   `json:"opacity"`
	ClickThrough bool      `json:"click_through"`
	Settings     *settings `json:"settings,omitempty"`
}

// clampOpacity 把任意输入夹到 Windows 和 Linux 的固定区间；
// macOS 按设置的范围限制，用 settings.clampOpacity。
func clampOpacity(v float64) float64 {
	if math.IsNaN(v) || v < minOpacity {
		return minOpacity
	}
	if v > maxOpacity {
		return maxOpacity
	}
	return round2(v)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// loadState 读取状态，文件缺失、损坏或取值越界时回落到默认值。
func loadState(path string) state {
	st := state{Opacity: defaultOpacity}
	if path == "" {
		return st
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	var raw struct {
		Opacity      *float64        `json:"opacity"`
		ClickThrough bool            `json:"click_through"`
		Settings     json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return st
	}
	// 显式保存的 0% 和缺少字段是两件事：缺少时继续用默认值。
	if raw.Opacity != nil {
		if v := *raw.Opacity; v >= allowedMinOpacity()-opacityEpsilon && v <= maxOpacity+opacityEpsilon {
			st.Opacity = round2(v)
		}
	}
	st.ClickThrough = raw.ClickThrough
	if settingsSupported && len(raw.Settings) != 0 {
		config, invalid := decodeSettings(raw.Settings)
		if invalid {
			fmt.Fprintln(os.Stderr, "ghostfish: 保存的设置无效，已使用对应默认值")
		}
		st.Settings = &config
		st.Opacity = config.clampOpacity(st.Opacity)
	}
	return st
}

// decodeSettings 分别读取范围和快捷键，损坏的一组使用默认值。
func decodeSettings(data json.RawMessage) (settings, bool) {
	config := defaultSettings()
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields == nil {
		return config, true
	}
	invalid := false
	// 用指针区分三种情况：合法数字（包括 0）、缺少字段、null 或类型错误。
	// json.Unmarshal 把 null 解到 int 会返回 nil 并留下零值，0 现在是合法下限，
	// 不分开就会把 null 当成 0。
	var low, high *int
	minErr := json.Unmarshal(fields["min_opacity"], &low)
	maxErr := json.Unmarshal(fields["max_opacity"], &high)
	switch {
	case minErr != nil || maxErr != nil || low == nil || high == nil:
		invalid = true
	case *low < settingsMinOpacity || *high > settingsMaxOpacity || *low > *high:
		invalid = true
	default:
		config.MinOpacity, config.MaxOpacity = *low, *high
	}
	var shortcuts [4]shortcutBinding
	if json.Unmarshal(fields["shortcuts"], &shortcuts) != nil {
		invalid = true
	} else {
		candidate := config
		candidate.Shortcuts = shortcuts
		if validateSettings(candidate) != nil {
			invalid = true
		} else {
			config.Shortcuts = shortcuts
		}
	}
	return config, invalid
}

// saveState 先写临时文件再改名，避免写一半留下坏文件。
func saveState(path string, st state) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
