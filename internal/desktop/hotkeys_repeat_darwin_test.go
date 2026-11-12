package desktop

import (
	"strings"
	"testing"
	"time"

	cfg "ghostfish/internal/config"
)

func TestLoadKeyState(t *testing.T) {
	if err := loadKeyState(); err != nil {
		t.Fatal(err)
	}
	_ = shortcutHeld(keyState(combinedSession, 0x1b), flagsState(combinedSession))
}

func TestShortcutHeld(t *testing.T) {
	tests := []struct {
		name  string
		down  bool
		flags uint64
		want  bool
	}{
		{"组合键按住", true, flagCommand | flagShift, true},
		{"主键松开", false, flagCommand | flagShift, false},
		{"Command 松开", true, flagShift, false},
		{"Shift 松开", true, flagCommand, false},
		{"额外按住 Control", true, flagCommand | flagShift | flagControl, false},
		{"额外按住 Option", true, flagCommand | flagShift | flagOption, false},
		{"大写锁定不影响组合键", true, flagCommand | flagShift | 1<<16, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortcutHeld(tt.down, tt.flags); got != tt.want {
				t.Fatalf("shortcutHeld(%v, %#x) = %v, want %v", tt.down, tt.flags, got, tt.want)
			}
		})
	}
}

func TestRepeatingShortcutStopsWhenReleased(t *testing.T) {
	held := true
	var calls int
	var scheduled []func()
	shortcut := repeatingShortcut{
		held:   func() bool { return held },
		action: func() { calls++ },
		after:  func(_ time.Duration, fn func()) { scheduled = append(scheduled, fn) },
	}

	shortcut.press()
	if calls != 1 || len(scheduled) != 1 {
		t.Fatalf("press: calls = %d, scheduled = %d", calls, len(scheduled))
	}
	scheduled[0]()
	if calls != 2 || len(scheduled) != 2 {
		t.Fatalf("held: calls = %d, scheduled = %d", calls, len(scheduled))
	}
	held = false
	scheduled[1]()
	if calls != 2 || len(scheduled) != 2 {
		t.Fatalf("released: calls = %d, scheduled = %d", calls, len(scheduled))
	}
}

func TestRepeatingShortcutIgnoresPreviousPress(t *testing.T) {
	var calls int
	var scheduled []func()
	shortcut := repeatingShortcut{
		held:   func() bool { return true },
		action: func() { calls++ },
		after:  func(_ time.Duration, fn func()) { scheduled = append(scheduled, fn) },
	}

	shortcut.press()
	shortcut.press()
	scheduled[0]()
	if calls != 2 || len(scheduled) != 2 {
		t.Fatalf("previous press: calls = %d, scheduled = %d", calls, len(scheduled))
	}
	scheduled[1]()
	if calls != 3 || len(scheduled) != 3 {
		t.Fatalf("current press: calls = %d, scheduled = %d", calls, len(scheduled))
	}
}

func TestConfiguredShortcutFlagsAndKeys(t *testing.T) {
	for _, key := range strings.Fields("A B C D E F G H I J K L M N O P Q R S T U V W X Y Z 0 1 2 3 4 5 6 7 8 9 - = Space F1 F2 F3 F4 F5 F6 F7 F8 F9 F10 F11 F12") {
		if _, ok := macKeyCodes[key]; !ok {
			t.Fatalf("missing key %s", key)
		}
	}
	binding := cfg.ShortcutBinding{Command: true, Control: true, Alt: true, Shift: true}
	if got := macFlags(binding); got != shortcutFlags {
		t.Fatalf("flags = %#x", got)
	}
}

func TestRepeatingShortcutCancelled(t *testing.T) {
	var calls int
	var scheduled []func()
	s := repeatingShortcut{held: func() bool { return true }, action: func() { calls++ }, after: func(_ time.Duration, fn func()) { scheduled = append(scheduled, fn) }}
	s.press()
	s.generation++
	scheduled[0]()
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}

// 换键或注销会调用 stop：已排队的重复任务不能再动窗口，之后重新按键仍能工作。
func TestRepeatingShortcutStopEndsOldRepeat(t *testing.T) {
	var calls int
	var scheduled []func()
	s := repeatingShortcut{held: func() bool { return true }, action: func() { calls++ }, after: func(_ time.Duration, fn func()) { scheduled = append(scheduled, fn) }}
	s.press()
	s.stop()
	scheduled[0]()
	if calls != 1 || len(scheduled) != 1 {
		t.Fatalf("stopped shortcut kept repeating: calls = %d, scheduled = %d", calls, len(scheduled))
	}
	s.press()
	scheduled[1]()
	if calls != 3 {
		t.Fatalf("shortcut should work again after stop: calls = %d", calls)
	}
}
