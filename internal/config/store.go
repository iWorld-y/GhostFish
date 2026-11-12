package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadState 读取状态，settingsSupported 决定是否读取 macOS 设置并允许 0% 不透明度。
// 文件缺失、损坏或取值越界时回落到默认值。
func LoadState(path string, settingsSupported bool) State {
	st := State{Opacity: DefaultOpacity}
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
		if v := *raw.Opacity; v >= allowedMinOpacity(settingsSupported)-opacityEpsilon && v <= MaxOpacity+opacityEpsilon {
			st.Opacity = Round2(v)
		}
	}
	st.ClickThrough = raw.ClickThrough
	if settingsSupported && len(raw.Settings) != 0 {
		config, invalid := decodeSettings(raw.Settings)
		if invalid {
			fmt.Fprintln(os.Stderr, "ghostfish: 保存的设置无效，已使用对应默认值")
		}
		st.Settings = &config
		st.Opacity = config.ClampOpacity(st.Opacity)
	}
	return st
}

// decodeSettings 分别读取范围和快捷键，损坏的一组使用默认值。
func decodeSettings(data json.RawMessage) (Settings, bool) {
	config := DefaultSettings()
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
	case *low < SettingsMinOpacity || *high > SettingsMaxOpacity || *low > *high:
		invalid = true
	default:
		config.MinOpacity, config.MaxOpacity = *low, *high
	}
	var shortcuts [4]ShortcutBinding
	if json.Unmarshal(fields["shortcuts"], &shortcuts) != nil {
		invalid = true
	} else {
		candidate := config
		candidate.Shortcuts = shortcuts
		if ValidateSettings(candidate) != nil {
			invalid = true
		} else {
			config.Shortcuts = shortcuts
		}
	}
	if raw, ok := fields["drag_modifiers"]; ok {
		drag, Valid := decodeDragModifiers(raw)
		if !Valid {
			invalid = true
		} else {
			config.DragModifiers = drag
		}
	}
	return config, invalid
}

// decodeDragModifiers 校验每个已保存的修饰键，并拒绝未知键与 null。
func decodeDragModifiers(data json.RawMessage) (DragModifiers, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields == nil {
		return DragModifiers{}, false
	}
	var result DragModifiers
	for name, raw := range fields {
		var value *bool
		if json.Unmarshal(raw, &value) != nil || value == nil {
			return DragModifiers{}, false
		}
		switch name {
		case "command":
			result.Command = *value
		case "control":
			result.Control = *value
		case "option":
			result.Option = *value
		case "shift":
			result.Shift = *value
		default:
			return DragModifiers{}, false
		}
	}
	return result, result.Valid()
}

// SaveState 先写临时文件再改名，避免写一半留下坏文件。
func SaveState(path string, st State) error {
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
