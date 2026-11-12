// Package app 组装浏览器、设置和桌面组件，并管理应用启动。
package app

import (
	"fmt"
	"os"
	"runtime"

	"ghostfish/internal/browser"
	cfg "ghostfish/internal/config"
	"ghostfish/internal/desktop"
	"ghostfish/internal/settings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Run 启动应用主循环，返回框架报告的错误。
func Run(target string) error {
	mygo.App.SetName("GhostFish")
	mygo.App.WhenReady(func() { start(target) })
	return mygo.App.Run()
}

// start 按窗口、浏览器、系统控制入口的顺序组装应用。
func start(target string) {
	if runtime.GOOS == "darwin" {
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	}
	path := stateFilePath()
	state := cfg.LoadState(path, runtime.GOOS == "darwin")
	var pages *browser.Browser
	var dialog *settings.Controller
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: "GhostFish", Hidden: true, Width: 960, Height: 720, StateKey: "main", AlwaysOnTop: true,
		Opacity: state.Opacity, BackgroundColor: "light-dark(#f5f5f7, #1e1e1e)", TitleBarStyle: mygo.TitleBarHidden,
		Content: ui.View(func(c *ui.Context) {
			pages.View(c)
			if runtime.GOOS == "darwin" {
				dialog.View(c)
			}
		}),
	})
	controls := desktop.New(win, state, path)
	dialog = settings.New(controls)
	pages = browser.New(win, dialog.Open)
	if err := pages.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: 创建标签页失败：%v\n", err)
		win.Close()
		return
	}
	trayErr := controls.Start()
	pages.LoadStartup(target)
	controls.ShowStartup()
	pages.FocusAddress()
	controls.FinishStartup(trayErr)
}
