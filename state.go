package main

import (
	"encoding/json"
	"math"
	"os"
)

const (
	defaultOpacity = 0.85
	minOpacity     = 0.20
	maxOpacity     = 1.00
	opacityStep    = 0.10
	// 浮点比较的容差，读写和算步进都靠它避免 0.8500000001 这种噪声。
	opacityEpsilon = 1e-9
)

// state 是跨重启保留的自定义状态。位置和尺寸由窗口自己的 StateKey 负责，不在这里。
type state struct {
	Opacity      float64 `json:"opacity"`
	ClickThrough bool    `json:"click_through"`
}

// clampOpacity 把任意输入夹到允许区间，NaN 之类的坏值回落到默认值。
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
		Opacity      float64 `json:"opacity"`
		ClickThrough bool    `json:"click_through"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return st
	}
	if raw.Opacity >= minOpacity-opacityEpsilon && raw.Opacity <= maxOpacity+opacityEpsilon {
		st.Opacity = round2(raw.Opacity)
	}
	st.ClickThrough = raw.ClickThrough
	return st
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
