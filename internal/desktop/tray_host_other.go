//go:build !linux

package desktop

import "sync/atomic"

// watchTrayHost 在 macOS 和 Windows 上使用系统托盘创建结果。
func watchTrayHost(_ *atomic.Uint64, _ func(trayHostStatus, uint64)) (trayHostStatus, func()) {
	return trayHostAvailable, func() {}
}
