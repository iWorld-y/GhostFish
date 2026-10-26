package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestClampOpacity(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{"低于下限夹到 20%", 0.05, minOpacity},
		{"正好下限", 0.20, 0.20},
		{"中间值", 0.65, 0.65},
		{"正好上限", 1.00, 1.00},
		{"高于上限夹到 100%", 1.5, maxOpacity},
		{"NaN 回落到下限", math.NaN(), minOpacity},
		{"浮点噪声收敛", 0.1 + 0.2, 0.3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampOpacity(tt.in); got != tt.want {
				t.Fatalf("clampOpacity(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := state{Opacity: 0.6, ClickThrough: true}
	if err := saveState(path, want); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	if got := loadState(path); got != want {
		t.Fatalf("loadState = %+v, want %+v", got, want)
	}
}

func TestLoadStateFallsBack(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    state
	}{
		{"文件不存在", "", state{Opacity: defaultOpacity}},
		{"JSON 损坏", "{not json", state{Opacity: defaultOpacity}},
		{"字段缺失", "{}", state{Opacity: defaultOpacity}},
		{"不透明度过低", `{"opacity":0.01}`, state{Opacity: defaultOpacity}},
		{"不透明度过高", `{"opacity":3}`, state{Opacity: defaultOpacity}},
		{"穿透字段类型错误", `{"opacity":0.6,"click_through":"yes"}`, state{Opacity: defaultOpacity}},
		{"正常值", `{"opacity":0.6,"click_through":true}`, state{Opacity: 0.6, ClickThrough: true}},
		{"下限值", `{"opacity":0.2}`, state{Opacity: 0.2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if tt.content != "" {
				if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
					t.Fatalf("write fixture: %v", err)
				}
			}
			if got := loadState(path); got != tt.want {
				t.Fatalf("loadState = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestEmptyPathKeepsDefaults(t *testing.T) {
	if got := loadState(""); got.Opacity != defaultOpacity || got.ClickThrough {
		t.Fatalf("loadState(\"\") = %+v, want defaults", got)
	}
	if err := saveState("", state{}); err != nil {
		t.Fatalf("saveState(\"\") should be a no-op, got %v", err)
	}
}
