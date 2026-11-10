package main

import "testing"

func TestValidateSettings(t *testing.T) {
	valid := defaultSettings()
	if err := validateSettings(valid); err != nil {
		t.Fatal(err)
	}
	for _, bounds := range [][2]int{{0, 0}, {0, 100}, {0, 35}, {35, 35}, {35, 100}, {100, 100}} {
		ranged := valid
		ranged.MinOpacity, ranged.MaxOpacity = bounds[0], bounds[1]
		if err := validateSettings(ranged); err != nil {
			t.Fatalf("%v should be valid: %v", bounds, err)
		}
	}
	tests := map[string]func(*settings){
		"下限越界":   func(s *settings) { s.MinOpacity = -1 },
		"上限越界":   func(s *settings) { s.MaxOpacity = 101 },
		"范围倒置":   func(s *settings) { s.MinOpacity, s.MaxOpacity = 80, 70 },
		"无修饰键":   func(s *settings) { s.Shortcuts[0] = shortcutBinding{Key: "A", Shift: true} },
		"重复快捷键":  func(s *settings) { s.Shortcuts[1] = s.Shortcuts[0] },
		"不支持的主键": func(s *settings) { s.Shortcuts[0].Key = "F13" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			got := valid
			mutate(&got)
			if err := validateSettings(got); err == nil {
				t.Fatal("应当拒绝无效设置")
			}
		})
	}
}

func TestSettingsClampOpacity(t *testing.T) {
	s := defaultSettings()
	s.MinOpacity, s.MaxOpacity = 35, 85
	for _, tc := range []struct{ in, want float64 }{{0, .35}, {.2, .35}, {.6, .6}, {.95, .85}} {
		if got := s.clampOpacity(tc.in); got != tc.want {
			t.Fatalf("clampOpacity(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
	zero := defaultSettings()
	zero.MinOpacity, zero.MaxOpacity = 0, 0
	if got := zero.clampOpacity(.85); got != 0 {
		t.Fatalf("clampOpacity(.85) with [0,0] = %v, want 0", got)
	}
}
