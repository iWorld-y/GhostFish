// Package desktop 管理窗口属性、托盘、快捷键与窗口恢复规则。
package desktop

import (
	"fmt"
	"os"
	"runtime"
	"sync/atomic"

	cfg "ghostfish/internal/config"

	"github.com/egoist/mygo"
)

// Controller 持有生效状态与系统控制入口；所有窗口操作在主线程执行。
type Controller struct {
	win                        *mygo.Window
	st                         cfg.State
	statePath                  string
	keys                       *shortcutManager
	registered                 *[4]bool
	tray                       *trayControls
	trayAvailable, trayPending bool
	trayUnknowns               int
	trayEvidence               atomic.Uint64
	trayWarning                func(string)
	settingsOpen               bool
	restoreError               string
	recoveryLost               bool
	ignoringMouse, visible     bool
	opacity                    float64
}

// New 连接主窗口和已加载的状态。
func New(win *mygo.Window, state cfg.State, path string) *Controller {
	return &Controller{win: win, st: state, statePath: path}
}

// Start 注册托盘和快捷键，返回应在窗口显示后报告的托盘错误。
func (a *Controller) Start() error {
	a.setOpacity(a.st.Opacity)
	trayErr := a.createTray(mygo.NewTray)
	if runtime.GOOS == "darwin" {
		a.CurrentSettings()
		a.keys = &shortcutManager{register: registerConfiguredShortcut}
		a.keys.actions = [4]func(){func() { a.changeOpacity(-cfg.OpacityStep) }, func() { a.changeOpacity(cfg.OpacityStep) }, a.toggleClickThrough, a.ToggleHidden}
		for _, failure := range a.keys.start(a.st.Settings.Shortcuts) {
			fmt.Fprintf(os.Stderr, "ghostfish: 快捷键注册失败：%s\n", failure)
		}
	} else {
		registered := a.registerHotkeys()
		a.registered = &registered
	}
	return trayErr
}

// ShowStartup 按已注册的恢复入口显示窗口。
func (a *Controller) ShowStartup() {
	var registered [4]bool
	if a.keys != nil {
		registered = a.keys.registered
	} else if a.registered != nil {
		registered = *a.registered
	}
	a.showStartupWindow(registered)
}

// FinishStartup 在窗口显示后报告托盘错误并安装原生拖动。
func (a *Controller) FinishStartup(trayErr error) {
	a.reportTrayFailure(trayErr)
	if err := a.installWindowDrag(); err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: 窗口拖动快捷键不可用：%v\n", err)
	}
	reportState(a.st)
}
