package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

var settingsSupported = runtime.GOOS == "darwin"

// setSettingsSupported 让测试覆盖指定的平台分支，并在结束时还原。
func setSettingsSupported(t *testing.T, supported bool) {
	t.Helper()
	previous := settingsSupported
	settingsSupported = supported
	t.Cleanup(func() { settingsSupported = previous })
}

// GHOSTFISH_TEST_NO_SETTINGS=1 让整套测试按「平台不支持设置」运行，
// 方便在没有 Linux/Windows 机器时检查跨平台读取路径。
func init() {
	if os.Getenv("GHOSTFISH_TEST_NO_SETTINGS") != "" {
		settingsSupported = false
	}
}

func TestSettingsStateRoundTrip(t *testing.T) {
	setSettingsSupported(t, true)
	path := filepath.Join(t.TempDir(), "state.json")
	config := DefaultSettings()
	config.MinOpacity, config.MaxOpacity = 40, 70
	config.Shortcuts[0] = ShortcutBinding{Key: "A", Control: true}
	want := State{Opacity: .6, ClickThrough: true, Settings: &config}
	if err := SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(path, settingsSupported); !reflect.DeepEqual(got, want) {
		t.Fatalf("loadState = %+v, want %+v", got, want)
	}
}

func TestInvalidSettingsKeepOldState(t *testing.T) {
	setSettingsSupported(t, true)
	path := filepath.Join(t.TempDir(), "state.json")
	data := `{"opacity":0.6,"click_through":true,"settings":{"min_opacity":"bad","max_opacity":70,"shortcuts":null}}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	got := LoadState(path, settingsSupported)
	if got.Opacity != .6 || !got.ClickThrough || got.Settings == nil || *got.Settings != DefaultSettings() {
		t.Fatalf("unexpected fallback: %+v", got)
	}
}

func TestSettingsRangeClampsRestoredOpacity(t *testing.T) {
	setSettingsSupported(t, true)
	path := filepath.Join(t.TempDir(), "state.json")
	config := DefaultSettings()
	config.MinOpacity, config.MaxOpacity = 40, 70
	if err := SaveState(path, State{Opacity: .9, Settings: &config}); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(path, settingsSupported); got.Opacity != .7 {
		t.Fatalf("opacity = %v, want .7", got.Opacity)
	}
}

// Windows 和 Linux 不读设置字段：只恢复原有字段，也不按自定义范围限制不透明度。
func TestSettingsIgnoredWithoutPlatformSupport(t *testing.T) {
	setSettingsSupported(t, false)
	path := filepath.Join(t.TempDir(), "state.json")
	config := DefaultSettings()
	config.MinOpacity, config.MaxOpacity = 40, 70
	if err := SaveState(path, State{Opacity: .9, ClickThrough: true, Settings: &config}); err != nil {
		t.Fatal(err)
	}
	got := LoadState(path, settingsSupported)
	if got.Settings != nil {
		t.Fatalf("settings should be ignored: %+v", got.Settings)
	}
	if got.Opacity != .9 || !got.ClickThrough {
		t.Fatalf("legacy fields should still load: %+v", got)
	}
}

// 范围字段要区分合法 0、缺少字段和 null：null 不能被当成 0。
func TestSettingsRangeParsing(t *testing.T) {
	setSettingsSupported(t, true)
	tests := []struct {
		name     string
		Settings string
		wantMin  int
		wantMax  int
		wantOp   float64
	}{
		{"合法 0 与 0", `{"min_opacity":0,"max_opacity":0}`, 0, 0, 0},
		{"合法 0 与 100", `{"min_opacity":0,"max_opacity":100}`, 0, 100, .85},
		{"下限 null", `{"min_opacity":null,"max_opacity":70}`, 0, 100, .85},
		{"上下限都 null", `{"min_opacity":null,"max_opacity":null}`, 0, 100, .85},
		{"上限缺失", `{"min_opacity":0}`, 0, 100, .85},
		{"类型错误", `{"min_opacity":"0","max_opacity":70}`, 0, 100, .85},
		{"保留已保存的范围", `{"min_opacity":50,"max_opacity":60}`, 50, 60, .60},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(`{"opacity":0.85,"settings":`+tt.Settings+`}`), 0600); err != nil {
				t.Fatal(err)
			}
			got := LoadState(path, settingsSupported)
			if got.Settings == nil || got.Settings.MinOpacity != tt.wantMin || got.Settings.MaxOpacity != tt.wantMax {
				t.Fatalf("range = %+v, want %d-%d", got.Settings, tt.wantMin, tt.wantMax)
			}
			if got.Opacity != tt.wantOp {
				t.Fatalf("opacity = %v, want %v", got.Opacity, tt.wantOp)
			}
		})
	}
}

// 保存的范围是 [0,0] 时，只改 opacity 也没用：重启后仍被范围夹回 0。
func TestZeroRangeOverridesSavedOpacity(t *testing.T) {
	setSettingsSupported(t, true)
	dir := t.TempDir()
	tests := []struct {
		name    string
		content string
		want    float64
	}{
		{"范围 [0,0] 时 opacity 被夹到 0", `{"opacity":0.85,"settings":{"min_opacity":0,"max_opacity":0}}`, 0},
		{"放宽上限后才能生效", `{"opacity":0.5,"settings":{"min_opacity":0,"max_opacity":100}}`, .5},
		{"删掉 settings 后生效", `{"opacity":0.5}`, .5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, "state.json")
			if err := os.WriteFile(path, []byte(tt.content), 0600); err != nil {
				t.Fatal(err)
			}
			if got := LoadState(path, settingsSupported); got.Opacity != tt.want {
				t.Fatalf("opacity = %v, want %v", got.Opacity, tt.want)
			}
		})
	}
}

// 显式保存的 0% 与缺少 opacity 字段是两件事。
func TestSavedZeroOpacityVersusMissingField(t *testing.T) {
	setSettingsSupported(t, true)
	dir := t.TempDir()

	zeroPath := filepath.Join(dir, "zero.json")
	if err := os.WriteFile(zeroPath, []byte(`{"opacity":0,"click_through":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(zeroPath, settingsSupported); got.Opacity != 0 || !got.ClickThrough {
		t.Fatalf("显式 0%% 应当保留：%+v", got)
	}

	missingPath := filepath.Join(dir, "missing.json")
	if err := os.WriteFile(missingPath, []byte(`{"click_through":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(missingPath, settingsSupported); got.Opacity != DefaultOpacity {
		t.Fatalf("缺少 opacity 时应用默认值 %v，实际 %v", DefaultOpacity, got.Opacity)
	}
}

// 自定义范围含 0 时，重启后按该范围限制当前不透明度。
func TestZeroRangeClampsRestoredOpacity(t *testing.T) {
	setSettingsSupported(t, true)
	path := filepath.Join(t.TempDir(), "state.json")
	config := DefaultSettings()
	config.MinOpacity, config.MaxOpacity = 0, 0
	if err := SaveState(path, State{Opacity: .85, Settings: &config}); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(path, settingsSupported); got.Opacity != 0 {
		t.Fatalf("opacity = %v, want 0", got.Opacity)
	}
}

// 非 macOS 平台仍然不把 0 当作有效的不透明度。
func TestZeroOpacityIgnoredWithoutPlatformSupport(t *testing.T) {
	setSettingsSupported(t, false)
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"opacity":0}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(path, settingsSupported); got.Opacity != DefaultOpacity {
		t.Fatalf("非 macOS 应回落到默认值 %v，实际 %v", DefaultOpacity, got.Opacity)
	}
}

func TestSettingsStateWriteFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "state.json")
	config := DefaultSettings()
	if err := SaveState(path, State{Opacity: .85, Settings: &config}); err == nil {
		t.Fatal("expected write failure")
	}
}
