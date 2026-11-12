//go:build darwin && gui

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 本地服务和内存会话覆盖真实 WebView 的重定向、认证、错误提示和成功加载。
func TestAuthDialogWebViewBasic(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("设置 GHOSTFISH_GUI=1 才打开隔离测试窗口")
	}
	var leaked atomic.Bool
	secure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/after" {
			fmt.Fprint(w, "ghostfish-auth-success")
			return
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != "test-user" || password != "test-pass" {
			w.Header().Set("WWW-Authenticate", `Basic realm="local-test"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/after", http.StatusFound)
	}))
	defer secure.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			leaked.Store(true)
		}
		http.Redirect(w, r, secure.URL+"/private", http.StatusFound)
	}))
	defer redirect.Close()

	updates := make(chan struct{}, 64)
	prompts := make(chan *authPrompt, 4)
	saved := make(chan authNeed, 2)
	finished := make(chan struct{}, 8)
	a := &app{st: state{Opacity: 0}}
	a.authHooks = &authHooks{
		resolve: func(context.Context, *authNeed) *mygo.AuthResponse { return nil },
		save: func(need authNeed, cred *mygo.AuthResponse) error {
			if cred.Username != "test-user" || cred.Password != "test-pass" {
				return fmt.Errorf("保存了非预期凭据")
			}
			saved <- need
			return nil
		},
		dispatch: func(fn func()) {
			mygo.RunOnMain(func() {
				fn()
				select {
				case updates <- struct{}{}:
				default:
				}
			})
		},
		show: func(p *authPrompt) { a.openAuthWindow(p); prompts <- p },
	}
	a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Auth local test", Hidden: true, Width: 320, Height: 200, Content: ui.View(a.view)})
	view, err := a.win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: mygo.NewEphemeralSession()}})
	if err != nil {
		t.Fatal(err)
	}
	tab := a.tabs.add(view)
	a.watchTabView(tab, view)
	view.Page().OnDidFinishLoad(func() {
		select {
		case finished <- struct{}{}:
		default:
		}
	})
	a.installAuthHandler(tab, view, "")
	a.win.Hide()
	a.win.SetOpacity(0)
	a.win.SetIgnoreMouseEvents(true)
	mygo.RunOnMain(func() { a.startNavigation(tab, redirect.URL+"/start", true) })
	var prompt *authPrompt
	select {
	case prompt = <-prompts:
	case <-time.After(10 * time.Second):
		t.Fatal("重定向后的 WebView 质询没有打开认证窗口")
	}
	mygo.RunOnMain(func() {
		if prompt.need.Realm != "local-test" || prompt.window == nil || !prompt.window.IsVisible() || prompt.window.Opacity() != 1 || a.win.IsVisible() {
			t.Errorf("认证窗口不可见或认证范围错误：%q", prompt.need.Realm)
		}
		prompt.username = "test-user"
		prompt.password = "wrong"
		a.submitAuth(prompt)
	})
	deadline := time.After(10 * time.Second)
	for {
		var failed bool
		mygo.RunOnMain(func() { failed = !prompt.busy && strings.Contains(prompt.errorText, "不正确") })
		if failed {
			break
		}
		select {
		case <-updates:
		case <-deadline:
			t.Fatal("错误密码没有显示可重试的提示")
		}
	}
	mygo.RunOnMain(func() { prompt.password = "test-pass"; a.submitAuth(prompt) })
	select {
	case need := <-saved:
		expected, _ := authNeedForURL(secure.URL, "local-test")
		if need != expected {
			t.Fatalf("保存的保护空间错误：%+v", need)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("正确凭据未完成真实 WebView 认证")
	}
	var body any
	deadline = time.After(10 * time.Second)
	for {
		mygo.RunOnMain(func() { body, err = view.Page().Eval("document.body.innerText") })
		if err == nil && strings.Contains(fmt.Sprint(body), "ghostfish-auth-success") {
			break
		}
		select {
		case <-finished:
		case <-deadline:
			t.Fatalf("WebView 未显示认证后的页面：%v %v", body, err)
		}
	}
	if leaked.Load() {
		t.Fatal("凭据泄漏到重定向来源")
	}
}

// Go 探测被站点拒绝时，实际 WebView 质询仍可输入、纠错并登录。
func TestAuthDialogWhenProbeFails(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("设置 GHOSTFISH_GUI=1 才打开隔离测试窗口")
	}
	var tries atomic.Int32
	var total atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if total.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		tries.Add(1)
		user, password, ok := r.BasicAuth()
		if !ok || user != "test-user" || password != "test-pass" {
			w.Header().Set("WWW-Authenticate", `Basic realm="probe-failure"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, "ghostfish-fallback-success")
	}))
	defer server.Close()
	updates := make(chan struct{}, 64)
	prompts := make(chan *authPrompt, 4)
	finished := make(chan struct{}, 8)
	var saved atomic.Bool
	a := &app{st: state{Opacity: .85}}
	a.authHooks = &authHooks{
		resolve: func(context.Context, *authNeed) *mygo.AuthResponse { return nil },
		verify: func(context.Context, string, *mygo.AuthResponse) (bool, int, error) {
			return false, http.StatusUnauthorized, nil
		},
		save: func(authNeed, *mygo.AuthResponse) error { saved.Store(true); return nil },
		dispatch: func(fn func()) {
			mygo.RunOnMain(func() {
				fn()
				select {
				case updates <- struct{}{}:
				default:
				}
			})
		},
		show: func(p *authPrompt) { a.openAuthWindow(p); prompts <- p },
	}
	a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Auth fallback test", Hidden: true, Width: 320, Height: 200, Content: ui.View(a.view)})
	view, err := a.win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: mygo.NewEphemeralSession()}})
	if err != nil {
		t.Fatal(err)
	}
	tab := a.tabs.add(view)
	a.watchTabView(tab, view)
	a.installAuthHandler(tab, view, "")
	view.Page().OnDidFinishLoad(func() {
		select {
		case finished <- struct{}{}:
		default:
		}
	})
	a.win.Show()
	mygo.RunOnMain(func() { a.startNavigation(tab, server.URL+"/private", false) })
	var p *authPrompt
	select {
	case p = <-prompts:
	case <-time.After(10 * time.Second):
		t.Fatal("探测失败后实际质询没有打开窗口")
	}
	mygo.RunOnMain(func() { p.username, p.password = "test-user", "wrong"; a.submitAuth(p) })
	deadline := time.After(10 * time.Second)
	for {
		var failed bool
		mygo.RunOnMain(func() { failed = !p.busy && strings.Contains(p.errorText, "不正确") })
		if failed {
			break
		}
		select {
		case <-updates:
		case <-finished:
		case <-deadline:
			mygo.RunOnMain(func() {
				attempts := -1
				if tab.authPrepared != nil {
					attempts = tab.authPrepared.attempts
				}
				t.Logf("busy=%v err=%q loading=%v preparedAttempts=%d tabURL=%q viewURL=%q requests=%d", p.busy, p.errorText, tab.authLoading, attempts, tab.authURL, view.Page().URL(), tries.Load())
			})
			t.Fatal("WebView 拒绝错误密码后没有提示")
		}
	}
	mygo.RunOnMain(func() { p.password = "test-pass"; a.submitAuth(p) })
	deadline = time.After(10 * time.Second)
	for {
		var closed bool
		mygo.RunOnMain(func() { closed = tab.authPrompt == nil })
		if closed {
			break
		}
		select {
		case <-updates:
		case <-finished:
		case <-deadline:
			t.Fatal("探测失败后正确凭据未完成 WebView 认证")
		}
	}
	var body any
	mygo.RunOnMain(func() { body, err = view.Page().Eval("document.body.innerText") })
	if err != nil || !strings.Contains(fmt.Sprint(body), "ghostfish-fallback-success") {
		t.Fatalf("未打开受保护页面：%v %v", body, err)
	}
	if saved.Load() {
		t.Fatal("无法验证的凭据不能保存")
	}
	if tries.Load() > 8 {
		t.Fatalf("认证请求过多：%d", tries.Load())
	}
}

// 子资源发出质询时，不能读取凭据或打开认证窗口。
func TestWebViewSubresourceDoesNotTriggerAuth(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("需要隔离图形测试")
	}
	for _, crossOrigin := range []bool{false, true} {
		name := "同来源"
		if crossOrigin {
			name = "跨来源"
		}
		t.Run(name, func(t *testing.T) {
			var credentialsSeen atomic.Bool
			received := make(chan struct{}, 1)
			requireAuth := func(w http.ResponseWriter, r *http.Request) {
				select {
				case received <- struct{}{}:
				default:
				}
				if r.Header.Get("Authorization") != "" {
					credentialsSeen.Store(true)
				}
				w.Header().Set("WWW-Authenticate", `Basic realm="other"`)
				w.WriteHeader(http.StatusUnauthorized)
			}
			subresource := httptest.NewServer(http.HandlerFunc(requireAuth))
			defer subresource.Close()
			mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/image" {
					requireAuth(w, r)
					return
				}
				source := "/image"
				if crossOrigin {
					source = subresource.URL + "/image"
				}
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, `<html><body>main<img src="%s"></body></html>`, source)
			}))
			defer mainServer.Close()
			var resolved, shown atomic.Int32
			done := make(chan struct{}, 4)
			a := &app{st: state{Opacity: .85}, authHooks: &authHooks{
				resolve: func(context.Context, *authNeed) *mygo.AuthResponse {
					resolved.Add(1)
					return &mygo.AuthResponse{Username: "secret", Password: "secret"}
				},
				show: func(*authPrompt) { shown.Add(1) },
			}}
			a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Auth resource test", Hidden: true, Width: 320, Height: 200, Content: ui.View(a.view)})
			view, err := a.win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: mygo.NewEphemeralSession()}})
			if err != nil {
				t.Fatal(err)
			}
			tab := a.tabs.add(view)
			a.watchTabView(tab, view)
			a.installAuthHandler(tab, view, "")
			view.Page().OnDidFinishLoad(func() {
				select {
				case done <- struct{}{}:
				default:
				}
			})
			a.win.Show()
			mygo.RunOnMain(func() { a.startNavigation(tab, mainServer.URL+"/", false) })
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("测试主页面加载超时")
			}
			select {
			case <-received:
			case <-time.After(5 * time.Second):
				t.Fatal("测试子资源未被请求")
			}
			if credentialsSeen.Load() || resolved.Load() != 0 || shown.Load() != 0 {
				t.Fatalf("子资源触发了凭据处理：credentials=%v resolve=%d dialogs=%d", credentialsSeen.Load(), resolved.Load(), shown.Load())
			}
		})
	}
}

// 已加载普通页面内的链接进入主文档认证时，也应显示认证窗口。
func TestAuthDialogFromLoadedPage(t *testing.T) {
	if os.Getenv("GHOSTFISH_GUI") == "" {
		t.Skip("需要隔离图形测试")
	}
	for _, crossOrigin := range []bool{false, true} {
		name := "同来源不同路径"
		if crossOrigin {
			name = "跨来源"
		}
		t.Run(name, func(t *testing.T) {
			protectedHandler := func(w http.ResponseWriter, r *http.Request) {
				user, password, ok := r.BasicAuth()
				if ok && user == "linked-user" && password == "linked-pass" {
					fmt.Fprint(w, "linked-auth-success")
					return
				}
				w.Header().Set("WWW-Authenticate", `Basic realm="linked-page"`)
				w.WriteHeader(http.StatusUnauthorized)
			}
			protected := httptest.NewServer(http.HandlerFunc(protectedHandler))
			defer protected.Close()
			mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/private" {
					protectedHandler(w, r)
					return
				}
				target := "/private"
				if crossOrigin {
					target = protected.URL + "/private"
				}
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, `<html><body><a id="auth-link" href="%s">open</a></body></html>`, target)
			}))
			defer mainServer.Close()
			prompts := make(chan *authPrompt, 1)
			loaded := make(chan struct{}, 4)
			saved := make(chan authNeed, 1)
			a := &app{st: state{Opacity: .85}, authHooks: &authHooks{
				resolve: func(context.Context, *authNeed) *mygo.AuthResponse { return nil },
				show:    func(p *authPrompt) { prompts <- p },
				pageURL: func(*mygo.WebView) string { return mainServer.URL + "/ordinary" },
				save:    func(need authNeed, _ *mygo.AuthResponse) error { saved <- need; return nil },
			}}
			a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Auth link test", Hidden: true, Width: 320, Height: 200, Content: ui.View(a.view)})
			view, err := a.win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Session: mygo.NewEphemeralSession()}})
			if err != nil {
				t.Fatal(err)
			}
			tab := a.tabs.add(view)
			a.watchTabView(tab, view)
			a.installAuthHandler(tab, view, "")
			view.Page().OnDidFinishLoad(func() {
				select {
				case loaded <- struct{}{}:
				default:
				}
			})
			a.win.Show()
			mygo.RunOnMain(func() { a.startNavigation(tab, mainServer.URL+"/ordinary", false) })
			select {
			case <-loaded:
			case <-time.After(10 * time.Second):
				t.Fatal("普通页面加载超时")
			}
			if _, err := view.Page().Eval("document.getElementById('auth-link').click()"); err != nil {
				t.Fatal(err)
			}
			select {
			case prompt := <-prompts:
				if prompt.need.Realm != "linked-page" {
					t.Fatalf("错误 realm：%q", prompt.need.Realm)
				}
				mygo.RunOnMain(func() {
					prompt.username, prompt.password = "linked-user", "linked-pass"
					a.submitAuth(prompt)
				})
			case <-time.After(5 * time.Second):
				var current, target string
				mygo.RunOnMain(func() { current, target = view.Page().URL(), tab.authURL })
				t.Fatalf("链接认证未弹窗：page=%q target=%q", current, target)
			}
			var body any
			deadline := time.After(10 * time.Second)
			loadedProtectedPage := false
			for !loadedProtectedPage {
				select {
				case <-loaded:
					mygo.RunOnMain(func() { body, err = view.Page().Eval("document.body.innerText") })
					loadedProtectedPage = err == nil && strings.Contains(fmt.Sprint(body), "linked-auth-success")
				case <-deadline:
					t.Fatalf("受保护页面没有加载：body=%v err=%v", body, err)
				}
			}
			target := mainServer.URL + "/private"
			if crossOrigin {
				target = protected.URL + "/private"
			}
			expected, _ := authNeedForURL(target, "linked-page")
			select {
			case need := <-saved:
				if need != expected {
					t.Fatalf("保存的保护空间错误：%+v", need)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("正确凭据未保存到隔离替身")
			}
		})
	}
}
