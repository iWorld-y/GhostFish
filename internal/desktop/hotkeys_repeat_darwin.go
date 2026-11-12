package desktop

import (
	"fmt"
	"sync"
	"time"

	cfg "ghostfish/internal/config"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
)

const (
	repeatDelay    = 400 * time.Millisecond
	repeatInterval = 80 * time.Millisecond

	combinedSession = 0
	flagShift       = 1 << 17
	flagControl     = 1 << 18
	flagOption      = 1 << 19
	flagCommand     = 1 << 20
	shortcutFlags   = flagShift | flagControl | flagOption | flagCommand
	requiredFlags   = flagShift | flagCommand
)

var (
	loadKeyStateOnce sync.Once
	loadKeyStateErr  error
	keyState         func(int32, uint16) bool
	flagsState       func(int32) uint64
)

// loadKeyState 加载读取当前会话按键状态所需的系统函数。
func loadKeyState() error {
	loadKeyStateOnce.Do(func() {
		var lib uintptr
		lib, loadKeyStateErr = purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if loadKeyStateErr != nil {
			return
		}
		purego.RegisterLibFunc(&keyState, lib, "CGEventSourceKeyState")
		purego.RegisterLibFunc(&flagsState, lib, "CGEventSourceFlagsState")
	})
	return loadKeyStateErr
}

// repeatingShortcut 只在快捷键仍被按住时重复执行动作，字段均在主线程访问。
type repeatingShortcut struct {
	held       func() bool
	action     func()
	after      func(time.Duration, func())
	generation uint64
}

func (s *repeatingShortcut) press() {
	s.generation++
	generation := s.generation
	s.action()
	s.after(repeatDelay, func() { s.repeat(generation) })
}

func (s *repeatingShortcut) repeat(generation uint64) {
	if generation != s.generation || !s.held() {
		return
	}
	s.action()
	s.after(repeatInterval, func() { s.repeat(generation) })
}

// stop 让已排队和以后的重复任务失效，换键、注销或非透明动作不再连发。
func (s *repeatingShortcut) stop() {
	s.generation++
}

// shortcutHeld 判断主键和修饰键是否仍构成快捷键。
func shortcutHeld(keyDown bool, flags uint64) bool {
	return keyDown && flags&shortcutFlags == requiredFlags
}

// registerRepeatShortcut 注册快捷键，并在组合键按住期间重复执行动作。
func registerRepeatShortcut(acc string, fn func()) error {
	var code uint16
	// 使用与 MyGo 注册快捷键时相同的 macOS 虚拟键码。
	switch acc {
	case "CmdOrCtrl+Shift+-":
		code = 0x1b
	case "CmdOrCtrl+Shift+=":
		code = 0x18
	default:
		return fmt.Errorf("不支持长按连发的快捷键 %s", acc)
	}
	if err := loadKeyState(); err != nil {
		return fmt.Errorf("读取按键状态失败：%w", err)
	}
	shortcut := &repeatingShortcut{
		held: func() bool {
			return shortcutHeld(keyState(combinedSession, code), flagsState(combinedSession))
		},
		action: fn,
		after: func(delay time.Duration, fn func()) {
			time.AfterFunc(delay, func() { mygo.RunOnMain(fn) })
		},
	}
	return mygo.GlobalShortcut.Register(acc, shortcut.press)
}

// registerConfiguredShortcut 注册配置的组合键，不透明度动作支持长按连发。
func registerConfiguredShortcut(index int, binding cfg.ShortcutBinding, fn func()) (func(), error) {
	acc := binding.Accelerator()
	var repeating *repeatingShortcut
	callback := fn
	if index == cfg.ShortcutDown || index == cfg.ShortcutUp {
		code, ok := macKeyCodes[binding.Key]
		if !ok {
			return nil, fmt.Errorf("不支持的主键 %s", binding.Key)
		}
		if err := loadKeyState(); err != nil {
			return nil, fmt.Errorf("读取按键状态失败：%w", err)
		}
		flags := macFlags(binding)
		repeating = &repeatingShortcut{
			held: func() bool {
				return keyState(combinedSession, code) && flagsState(combinedSession)&shortcutFlags == flags
			},
			action: fn,
			after: func(delay time.Duration, fn func()) {
				time.AfterFunc(delay, func() { mygo.RunOnMain(fn) })
			},
		}
		callback = repeating.press
	}
	if err := mygo.GlobalShortcut.Register(acc, callback); err != nil {
		return nil, err
	}
	return func() {
		if repeating != nil {
			repeating.stop()
		}
		mygo.GlobalShortcut.Unregister(acc)
	}, nil
}

func macFlags(s cfg.ShortcutBinding) uint64 {
	var flags uint64
	if s.Command {
		flags |= flagCommand
	}
	if s.Control {
		flags |= flagControl
	}
	if s.Alt {
		flags |= flagOption
	}
	if s.Shift {
		flags |= flagShift
	}
	return flags
}

// macKeyCodes 与 macOS 原生虚拟键码及 MyGo 注册时使用的键码一致。
var macKeyCodes = map[string]uint16{
	"A": 0x00, "S": 0x01, "D": 0x02, "F": 0x03, "H": 0x04, "G": 0x05,
	"Z": 0x06, "X": 0x07, "C": 0x08, "V": 0x09, "B": 0x0b, "Q": 0x0c,
	"W": 0x0d, "E": 0x0e, "R": 0x0f, "Y": 0x10, "T": 0x11,
	"1": 0x12, "2": 0x13, "3": 0x14, "4": 0x15, "6": 0x16, "5": 0x17,
	"=": 0x18, "9": 0x19, "7": 0x1a, "-": 0x1b, "8": 0x1c, "0": 0x1d,
	"O": 0x1f, "U": 0x20, "I": 0x22, "P": 0x23, "L": 0x25, "J": 0x26,
	"K": 0x28, "N": 0x2d, "M": 0x2e, "Space": 0x31,
	"F1": 0x7a, "F2": 0x78, "F3": 0x63, "F4": 0x76, "F5": 0x60, "F6": 0x61,
	"F7": 0x62, "F8": 0x64, "F9": 0x65, "F10": 0x6d, "F11": 0x67, "F12": 0x6f,
}
