// Command ghostfish 是摸鱼用的浮动浏览器：一个可调不透明度、可穿透鼠标的窗口，
// 只在终端启动，控制全部走全局快捷键。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
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
	target    string
	win       *mygo.Window
	st        state
	statePath string
	// authHost 是拿到凭据的主机，只有它自己的质询才用这份凭据。
	authHost string
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
		fmt.Fprintln(os.Stderr, "用法：ghostfish <网址>")
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

	// 认证要在开窗口之前谈好：窗口一出来就撞 401 的话，用户看到的是报错页。
	cred := a.prepareAuth()

	// 先建一个不显示的窗口：认证处理器必须在第一次请求之前就位，
	// 否则质询会先落到 webview 的默认处理上。
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:           appName,
		Hidden:          true,
		Width:           defaultWidth,
		Height:          defaultHeight,
		StateKey:        stateKey,
		AlwaysOnTop:     true,
		Opacity:         a.st.Opacity,
		BackgroundColor: background,
	})

	if cred != nil {
		host := a.authHost
		attempts, warned := 0, false
		a.win.Page().SetAuthenticationHandler(func(req mygo.AuthenticationRequest) *mygo.Credential {
			if !strings.EqualFold(req.Host, host) {
				return nil
			}
			// 同一份凭据给两次就够：再被质询说明服务器不认，
			// 一直给只会让 webview 无限重试，把服务器刷成重放攻击。
			attempts++
			if attempts > 2 {
				if !warned {
					warned = true
					fmt.Fprintln(os.Stderr, "ghostfish: 凭据被服务器拒了，已停止重试；用户名或密码大概率不对")
				}
				return nil
			}
			return cred
		})
	}

	a.win.Show()
	if err := a.win.Page().LoadURL(a.target); err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: 载入 %s 失败：%v\n", a.target, err)
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
