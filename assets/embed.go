// Package assets 提供应用内使用的托盘图标。
package assets

import _ "embed"

// TrayIcon 是 macOS 和 Windows 使用的模板图标。
//
//go:embed logo/ghostfish-tray.png
var TrayIcon []byte

// TrayLinuxIcon 是 Linux 托盘使用的带底色图标。
//
//go:embed logo/ghostfish-tray-linux.png
var TrayLinuxIcon []byte
