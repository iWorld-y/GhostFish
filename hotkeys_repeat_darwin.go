package main

import (
	"fmt"
	"sync"
	"time"

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
