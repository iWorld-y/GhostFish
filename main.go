// Command ghostfish 是摸鱼用的浮动浏览器：一个可调不透明度、可穿透鼠标的窗口，
// 只在终端启动，窗口状态由全局快捷键控制。
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	appName       = "GhostFish"
	stateFileName = "state.json"
	defaultWidth  = 960
	defaultHeight = 720
	background    = "light-dark(#f5f5f7, #1e1e1e)"
	stateKey      = "main"
)

// app 持有窗口和这一份运行期状态。所有回调都在主线程执行。
type app struct {
	target          string
	win             *mygo.Window
	st              state
	statePath       string
	tabs            tabSet
	focusAddress    bool
	revealActiveTab bool
	pollTimer       *time.Timer
	pollGeneration  uint64
}

// version 由发布构建注入：go build -ldflags "-X main.version=v0.1.0"。
// 从源码直接构建时是 dev，方便分辨手上这个是不是下载来的。
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Printf("ghostfish %s\n", version)
		return
	}

	target, err := targetURL(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: %v\n", err)
		fmt.Fprintln(os.Stderr, "用法：ghostfish [网址]")
		fmt.Fprintln(os.Stderr, "      ghostfish -version")
		fmt.Fprintln(os.Stderr, "例：ghostfish https://example.com")
		os.Exit(2)
	}

	mygo.App.SetName(appName)

	a := &app{target: target}
	mygo.App.WhenReady(a.start)
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: %v\n", err)
		os.Exit(1)
	}
}

func (a *app) start() {
	// 摸鱼场景里 Dock 图标本身就显眼，macOS 上不占 Dock。
	if runtime.GOOS == "darwin" {
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	}

	a.statePath = stateFilePath()
	a.st = loadState(a.statePath)

	// 启动网址仍在窗口显示前准备认证；空白启动不会发起请求。
	authHost, cred := prepareAuth(context.Background(), a.target)
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:           appName,
		Hidden:          true,
		Width:           defaultWidth,
		Height:          defaultHeight,
		StateKey:        stateKey,
		AlwaysOnTop:     true,
		Opacity:         a.st.Opacity,
		BackgroundColor: background,
		TitleBarStyle:   mygo.TitleBarHidden,
		Content:         ui.View(a.view),
	})

	tab, err := a.createTab()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: 创建标签页失败：%v\n", err)
		a.win.Close()
		return
	}
	if a.target != "" {
		a.focusAddress = false
		tab.startPageLoad(a.target, true)
		if cred != nil {
			tab.view.Page().SetAuthHandler(authHandler(authHost, cred))
		}
		if err := tab.view.Page().LoadURL(a.target); err != nil {
			tab.failPageLoad(tab.view, fmt.Sprintf("载入网址失败：%v", err))
			fmt.Fprintf(os.Stderr, "ghostfish: 载入网址失败：%v\n", err)
		}
	}
	a.win.OnShow(a.startAddressPolling)
	a.win.OnHide(a.stopAddressPolling)
	a.win.OnClosed(a.stopAddressPolling)
	a.win.Show()
	if tab.showsAddress() {
		mygo.App.Focus()
		a.win.Focus()
		a.focusAddress = true
		a.win.Invalidate()
	}

	var forced bool
	a.st, forced = resolveStartState(a.registerHotkeys(), a.st)
	if forced {
		fmt.Fprintln(os.Stderr, "ghostfish: 所有快捷键都注册失败，已强制以不穿透状态启动")
		a.save()
	}
	a.win.SetIgnoreMouseEvents(a.st.ClickThrough)

	reportState(a.st)
}

func (a *app) changeOpacity(delta float64) {
	next := clampOpacity(round2(a.st.Opacity + delta))
	if next == a.st.Opacity {
		return
	}
	a.st.Opacity = next
	a.win.SetOpacity(next)
	a.save()
	reportState(a.st)
}

func (a *app) toggleClickThrough() {
	a.st.ClickThrough = !a.st.ClickThrough
	a.win.SetIgnoreMouseEvents(a.st.ClickThrough)
	a.save()
	reportState(a.st)
}

func (a *app) toggleHidden() {
	if a.win.IsVisible() {
		a.win.Hide()
		fmt.Fprintln(os.Stderr, "ghostfish: 窗口已隐藏，按同一个快捷键恢复")
		return
	}
	a.win.Show()
	fmt.Fprintln(os.Stderr, "ghostfish: 窗口已恢复")
}

func (a *app) save() {
	if err := saveState(a.statePath, a.st); err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: 保存状态失败：%v\n", err)
	}
}

// reportState 把当前状态写到终端，充当本版唯一的状态显示入口。
func reportState(st state) {
	through := "关"
	if st.ClickThrough {
		through = "开"
	}
	fmt.Fprintf(os.Stderr, "ghostfish: 不透明度 %d%% · 穿透 %s\n", int(st.Opacity*100+0.5), through)
}

func stateFilePath() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil || dir == "" {
		fmt.Fprintf(os.Stderr, "ghostfish: 拿不到用户数据目录，本次不保存状态：%v\n", err)
		return ""
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: 创建数据目录失败，本次不保存状态：%v\n", err)
		return ""
	}
	return filepath.Join(dir, stateFileName)
}
