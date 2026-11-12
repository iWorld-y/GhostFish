// Package config 定义生效配置、校验规则与状态持久化。
package config

import (
	"math"
)

const (
	DefaultOpacity = 0.85
	// minOpacity 和 maxOpacity 是 Windows 和 Linux 的固定范围；
	// 支持设置的平台允许将下限调到 0。
	MinOpacity  = 0.20
	MaxOpacity  = 1.00
	OpacityStep = 0.10
	// 浮点比较的容差，读写和算步进都靠它避免 0.8500000001 这种噪声。
	opacityEpsilon = 1e-9
)

// allowedMinOpacity 是不透明度允许的最小值：macOS 用户可以调到 0%，
// 其他平台沿用原来的 20%。
func allowedMinOpacity(settingsSupported bool) float64 {
	if settingsSupported {
		return 0
	}
	return MinOpacity
}

// State 是跨重启保留的自定义状态。位置和尺寸由窗口自己的 StateKey 负责，不在这里。
type State struct {
	Opacity      float64   `json:"opacity"`
	ClickThrough bool      `json:"click_through"`
	Settings     *Settings `json:"settings,omitempty"`
}

// ClampOpacity 把任意输入夹到 Windows 和 Linux 的固定区间；
// macOS 按设置的范围限制，用 Settings.ClampOpacity。
func ClampOpacity(v float64) float64 {
	if math.IsNaN(v) || v < MinOpacity {
		return MinOpacity
	}
	if v > MaxOpacity {
		return MaxOpacity
	}
	return Round2(v)
}

func Round2(v float64) float64 {
	return math.Round(v*100) / 100
}
