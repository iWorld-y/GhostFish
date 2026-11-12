package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestMatchesAuthURL(t *testing.T) {
	for _, tt := range []struct {
		name, url string
		req       mygo.AuthRequest
		want      bool
	}{
		{"匹配主机和端口", "http://example.test:8080/private", mygo.AuthRequest{Host: "EXAMPLE.test", Port: 8080, Scheme: "basic"}, true},
		{"默认 HTTPS 端口", "https://example.test/private", mygo.AuthRequest{Host: "example.test", Port: 443, Scheme: "basic"}, true},
		{"其他主机", "https://example.test", mygo.AuthRequest{Host: "other.test", Port: 443, Scheme: "basic"}, false},
		{"其他端口", "http://example.test:8080", mygo.AuthRequest{Host: "example.test", Port: 8081, Scheme: "basic"}, false},
		{"代理", "https://example.test", mygo.AuthRequest{Host: "example.test", Port: 443, Scheme: "basic", IsProxy: true}, false},
		{"非 Basic", "https://example.test", mygo.AuthRequest{Host: "example.test", Port: 443, Scheme: "digest"}, false},
		{"网址自带凭据", "https://user:pass@example.test", mygo.AuthRequest{Host: "example.test", Port: 443, Scheme: "basic"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesAuthURL(tt.url, tt.req); got != tt.want {
				t.Fatalf("matchesAuthURL = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSameAuthDocument(t *testing.T) {
	for _, tt := range []struct {
		left, right string
		want        bool
	}{
		{"https://example.test", "https://EXAMPLE.test/", true},
		{"https://example.test/a?x=1#frag", "https://example.test/a?x=1", true},
		{"https://example.test/main", "https://example.test/iframe", false},
		{"https://example.test/main?x=1", "https://example.test/main?x=2", false},
		{"https://example.test/main", "https://other.test/main", false},
	} {
		if got := sameAuthDocument(tt.left, tt.right); got != tt.want {
			t.Fatalf("sameAuthDocument(%q, %q) = %v", tt.left, tt.right, got)
		}
	}
}

// 尚未提交的新页面只在主文档导航得到确认时允许处理质询。
func TestAuthDocumentAllowed(t *testing.T) {
	expected := "https://example.test/private"
	previous := "https://example.test/ordinary"
	if authDocumentAllowed(expected, previous, false) {
		t.Fatal("未确认主文档导航时不应接受旧页面地址")
	}
	if !authDocumentAllowed(expected, previous, true) {
		t.Fatal("主文档导航时应允许地址暂未更新")
	}
	if !authDocumentAllowed(expected, expected, false) {
		t.Fatal("当前页面地址匹配时应允许处理质询")
	}
}

// 凭据只返回给当前主文档对应的主机、端口和 realm，且同一份凭据只尝试一次。
func TestAuthChallengeScopeAndRetry(t *testing.T) {
	shown := make(chan *authPrompt, 1)
	a := &app{authHooks: &authHooks{dispatch: func(fn func()) { fn() }, show: func(p *authPrompt) { shown <- p }}}
	tab := a.tabs.add(nil)
	tab.authURL = "https://example.test/private"
	tab.authLoading = true
	tab.authPrepared = &preparedAuth{need: authNeed{Host: "example.test", Realm: "realm-a"}, url: tab.authURL, credential: &mygo.AuthResponse{Username: "test-user", Password: "test-pass"}}
	for _, req := range []mygo.AuthRequest{
		{Host: "other.test", Port: 443, Scheme: "basic"},
		{Host: "example.test", Port: 444, Scheme: "basic"},
		{Host: "example.test", Port: 443, Scheme: "basic", IsProxy: true},
		{Host: "example.test", Port: 443, Scheme: "digest"},
	} {
		if got := a.handleAuthChallenge(tab, nil, req); got != (mygo.AuthResponse{}) {
			t.Fatal("向不匹配的认证质询返回了凭据")
		}
	}
	req := mygo.AuthRequest{Host: "example.test", Port: 443, Scheme: "basic", Realm: "realm-a"}
	if got := a.handleAuthChallenge(tab, nil, req); got.Username != "test-user" {
		t.Fatal("匹配的认证质询未收到凭据")
	}
	if got := a.handleAuthChallenge(tab, nil, req); got != (mygo.AuthResponse{}) {
		t.Fatal("被拒绝的凭据不应自动再次发送")
	}
	select {
	case p := <-shown:
		if !strings.Contains(p.errorText, "不正确") {
			t.Fatal("被拒绝后应显示重试提示")
		}
		a.cancelTabAuth(tab)
	case <-time.After(time.Second):
		t.Fatal("被拒绝后没有打开输入窗口")
	}
	if got := a.handleAuthChallenge(tab, nil, req); got != (mygo.AuthResponse{}) || tab.authPrompt != nil {
		t.Fatal("取消后不应自动重新弹窗")
	}
}

func TestDifferentRealmDoesNotReceivePreparedCredential(t *testing.T) {
	shown := make(chan *authPrompt, 1)
	a := &app{authHooks: &authHooks{
		resolve:  func(context.Context, *authNeed) *mygo.AuthResponse { return nil },
		dispatch: func(fn func()) { fn() }, show: func(p *authPrompt) { shown <- p },
	}}
	tab := a.tabs.add(nil)
	tab.authURL = "https://example.test/private"
	tab.authLoading = true
	tab.authPrepared = &preparedAuth{need: authNeed{Host: "example.test", Realm: "first"}, url: tab.authURL, credential: &mygo.AuthResponse{Username: "first-user", Password: "first-pass"}}
	req := mygo.AuthRequest{Host: "example.test", Port: 443, Scheme: "basic", Realm: "second"}
	if got := a.handleAuthChallenge(tab, nil, req); got != (mygo.AuthResponse{}) {
		t.Fatal("不同 realm 不应收到之前的凭据")
	}
	select {
	case p := <-shown:
		if p.need.Realm != "second" || p.errorText != "" {
			t.Fatal("新 realm 应显示独立输入窗口")
		}
		a.cancelTabAuth(tab)
	case <-time.After(time.Second):
		t.Fatal("没有处理新 realm")
	}
}

// 异步结果入队后即使用户取消，也不能加载页面或保存凭据。
func TestCanceledAuthDiscardsQueuedResult(t *testing.T) {
	callbacks := make(chan func(), 1)
	loads := make(chan string, 1)
	a := &app{authHooks: &authHooks{
		verify:   func(context.Context, string, *mygo.AuthResponse) (bool, int, error) { return true, 200, nil },
		dispatch: func(fn func()) { callbacks <- fn },
		load:     func(_ *browserTab, raw string) error { loads <- raw; return nil },
	}}
	tab := a.tabs.add(nil)
	ctx, cancel := context.WithCancel(context.Background())
	p := &authPrompt{tab: tab, generation: tab.generation, url: "https://example.test/private", need: authNeed{Host: "example.test", Realm: "test"}, ctx: ctx, cancel: cancel, username: "user", password: "secret"}
	tab.authPrompt = p
	a.submitAuth(p)
	var callback func()
	select {
	case callback = <-callbacks:
	case <-time.After(time.Second):
		t.Fatal("验证结果没有入队")
	}
	a.cancelTabAuth(tab)
	tab.beginNavigation()
	callback()
	select {
	case <-loads:
		t.Fatal("取消后旧回调仍加载了页面")
	default:
	}
	if p.password != "" || tab.authPrompt != nil || tab.authPrepared != nil {
		t.Fatal("取消时未清除临时凭据")
	}
}

// 取消发生在异步来源完成之前时，过期结果不能弹出窗口。
func TestCanceledAuthResolutionDoesNotShow(t *testing.T) {
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	shown := make(chan struct{}, 1)
	a := &app{authHooks: &authHooks{
		resolve:  func(context.Context, *authNeed) *mygo.AuthResponse { close(started); <-release; return nil },
		dispatch: func(fn func()) { fn() }, show: func(*authPrompt) { shown <- struct{}{} },
	}}
	tab := a.tabs.add(nil)
	ctx, cancel := context.WithCancel(context.Background())
	p := &authPrompt{tab: tab, generation: tab.generation, url: "https://example.test/", need: authNeed{Host: "example.test"}, ctx: ctx, cancel: cancel}
	tab.authPrompt = p
	go func() { defer close(done); a.resolveChallenge(p) }()
	<-started
	a.cancelTabAuth(tab)
	close(release)
	<-done
	select {
	case <-shown:
		t.Fatal("取消后显示了过期窗口")
	default:
	}
}

func TestAuthDialogMasksPassword(t *testing.T) {
	a := &app{}
	p := &authPrompt{url: "https://example.test/", need: authNeed{Host: "example.test", Realm: "members"}, password: "secret-value"}
	tester := ui.NewTester(func(c *ui.Context) { a.authDialog(c, p) }, 420, 340)
	info, ok := tester.Access("密码")
	if !ok || !info.Password || strings.Contains(info.Value, "secret-value") {
		t.Fatalf("密码输入未遮挡：%+v", info)
	}
	for _, text := range tester.Texts() {
		if strings.Contains(text, "secret-value") {
			t.Fatal("界面泄露了明文密码")
		}
	}
	if !tester.HasText("主机：example.test") || !tester.HasText("认证范围：members") {
		t.Fatal("认证窗口没有显示主机和 realm")
	}
}

func TestVerifyCredentialRejectsForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer srv.Close()
	ok, status, err := verifyCredential(context.Background(), srv.URL, &mygo.AuthResponse{Username: "user", Password: "pass"})
	if err != nil || ok || status != http.StatusForbidden {
		t.Fatalf("403 不应被视为凭据验证成功：ok=%v status=%d err=%v", ok, status, err)
	}
}

func TestPreparationCanceledAfterResolve(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := &app{authHooks: &authHooks{resolve: func(context.Context, *authNeed) *mygo.AuthResponse {
		t.Fatal("已取消的准备不应读取凭据")
		return nil
	}}}
	if got := a.prepareAuth(ctx, "https://example.test"); got != nil {
		t.Fatal("已取消的准备返回了凭据")
	}
}

func TestAuthDialogsAreIsolatedPerTab(t *testing.T) {
	shown := make(chan *authPrompt, 2)
	callbacks := make(chan func(), 2)
	a := &app{authHooks: &authHooks{
		resolve:  func(context.Context, *authNeed) *mygo.AuthResponse { return nil },
		dispatch: func(fn func()) { callbacks <- fn }, show: func(p *authPrompt) { shown <- p },
	}}
	first := a.tabs.add(nil)
	second := a.tabs.add(nil)
	first.authURL, first.authLoading = "https://first.test/", true
	second.authURL, second.authLoading = "https://second.test/", true
	a.handleAuthChallenge(first, nil, mygo.AuthRequest{Host: "first.test", Port: 443, Scheme: "basic", Realm: "first"})
	a.handleAuthChallenge(second, nil, mygo.AuthRequest{Host: "second.test", Port: 443, Scheme: "basic", Realm: "second"})
	for i := 0; i < 2; i++ {
		select {
		case callback := <-callbacks:
			callback()
		case <-time.After(time.Second):
			t.Fatal("认证窗口请求没有入队")
		}
	}
	select {
	case p := <-shown:
		if p != second.authPrompt {
			t.Fatal("后台标签不应显示认证窗口")
		}
	default:
		t.Fatal("当前标签未显示认证窗口")
	}
	select {
	case <-shown:
		t.Fatal("后台标签不应显示认证窗口")
	default:
	}
	previous := a.tabs.active
	a.tabs.selectTab(first)
	a.syncAuthTab(previous)
	select {
	case p := <-shown:
		if p != first.authPrompt {
			t.Fatal("切换后应显示对应标签的认证窗口")
		}
	default:
		t.Fatal("切换后未显示认证窗口")
	}
	first.authPrepared = &preparedAuth{need: authNeed{Host: "first.test", Realm: "first"}, url: first.authURL, credential: &mygo.AuthResponse{Username: "first-user", Password: "first-pass"}}
	if got := a.handleAuthChallenge(second, nil, mygo.AuthRequest{Host: "second.test", Port: 443, Scheme: "basic", Realm: "second"}); got != (mygo.AuthResponse{}) {
		t.Fatal("一个标签页的凭据泄漏到另一个标签页")
	}
	a.cancelTabAuth(first)
	if second.authPrompt == nil {
		t.Fatal("取消一个标签页不应关闭另一个标签页的认证")
	}
	a.cancelTabAuth(second)
}

// 标签切回时，如果凭据来源仍在查询，不能提前显示输入窗口。
func TestSelectingTabWaitsForCredentialResolution(t *testing.T) {
	for _, tt := range []struct {
		name       string
		credential *mygo.AuthResponse
		switchAway bool
	}{
		{"没有已有凭据", nil, false},
		{"没有凭据且已经切走", nil, true},
		{"已有可用凭据", &mygo.AuthResponse{Username: "saved-user", Password: "saved-pass"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			callbacks := make(chan func(), 1)
			shown := make(chan *authPrompt, 2)
			loads := make(chan string, 1)
			a := &app{authHooks: &authHooks{
				resolve: func(context.Context, *authNeed) *mygo.AuthResponse {
					close(entered)
					<-release
					return tt.credential
				},
				verify:   func(context.Context, string, *mygo.AuthResponse) (bool, int, error) { return true, 200, nil },
				dispatch: func(fn func()) { callbacks <- fn },
				show:     func(p *authPrompt) { shown <- p },
				load:     func(_ *browserTab, raw string) error { loads <- raw; return nil },
			}}
			first := a.tabs.add(nil)
			second := a.tabs.add(nil)
			first.authURL, first.authSourceURL, first.authLoading = "https://example.test/private", "https://example.test/private", true
			req := mygo.AuthRequest{Host: "example.test", Port: 443, Scheme: "basic", Realm: "members"}
			if got := a.handleAuthChallenge(first, nil, req); got != (mygo.AuthResponse{}) {
				t.Fatal("尚未解析凭据时不能直接返回凭据")
			}
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("凭据来源未开始查询")
			}
			previous := a.tabs.active
			a.tabs.selectTab(first)
			a.syncAuthTab(previous)
			select {
			case <-shown:
				close(release)
				t.Fatal("凭据来源尚未返回时不应提前弹窗")
			default:
			}
			if tt.switchAway {
				previous = a.tabs.active
				a.tabs.selectTab(second)
				a.syncAuthTab(previous)
			}
			close(release)
			select {
			case callback := <-callbacks:
				callback()
			case <-time.After(time.Second):
				t.Fatal("凭据查询结果没有回到界面")
			}
			if tt.credential != nil {
				select {
				case <-shown:
					t.Fatal("已有凭据时不应显示输入窗口")
				default:
				}
				a.finishAuthLoad(first, nil, true)
				select {
				case raw := <-loads:
					if raw != first.authURL {
						t.Fatal("已有凭据重试了错误网址")
					}
				default:
					t.Fatal("已有凭据应继续认证")
				}
			} else if tt.switchAway {
				select {
				case <-shown:
					t.Fatal("后台标签不应显示输入窗口")
				default:
				}
				previous = a.tabs.active
				a.tabs.selectTab(first)
				a.syncAuthTab(previous)
				select {
				case p := <-shown:
					if p != first.authPrompt {
						t.Fatal("切回后显示了错误的认证请求")
					}
				default:
					t.Fatal("切回后应显示已准备好的输入窗口")
				}
			} else {
				select {
				case p := <-shown:
					if p != first.authPrompt {
						t.Fatal("显示了错误的认证请求")
					}
				default:
					t.Fatal("没有已有凭据时应显示输入窗口")
				}
			}
			a.cancelTabAuth(first)
		})
	}
}

func TestAuthDialogCancelClearsState(t *testing.T) {
	a := &app{}
	tab := a.tabs.add(nil)
	ctx, cancel := context.WithCancel(context.Background())
	p := &authPrompt{tab: tab, ctx: ctx, cancel: cancel, url: "https://example.test", need: authNeed{Host: "example.test"}, username: "user", password: "secret"}
	tab.authPrompt, tab.authLoading = p, true
	tester := ui.NewTester(func(c *ui.Context) { a.authDialog(c, p) }, 420, 340)
	if err := tester.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if tab.authPrompt != nil || tab.authLoading || p.password != "" || ctx.Err() == nil {
		t.Fatal("取消后应关闭认证流程并清除临时凭据")
	}
}

// 启动参数的网络探测被阻塞时，窗口调度不应等待探测完成。
func TestStartNavigationDoesNotBlockOnProbe(t *testing.T) {
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
	}))
	defer server.Close()
	a := &app{}
	tab := a.tabs.add(nil)
	returned := make(chan struct{})
	go func() { a.startNavigation(tab, server.URL, true); close(returned) }()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("启动导航阻塞在认证探测")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("未启动后台探测")
	}
	a.cancelTabAuth(tab)
	tab.beginNavigation()
}

// 跨来源重定向后只能由用户明确输入，不自动复用原网址的凭据来源。
func TestCrossOriginRedirectDoesNotResolveExistingCredential(t *testing.T) {
	shown := make(chan *authPrompt, 1)
	resolved := make(chan struct{}, 1)
	a := &app{authHooks: &authHooks{
		resolve: func(context.Context, *authNeed) *mygo.AuthResponse {
			resolved <- struct{}{}
			return &mygo.AuthResponse{Username: "old-user", Password: "old-password"}
		},
		dispatch: func(fn func()) { fn() }, show: func(p *authPrompt) { shown <- p },
	}}
	tab := a.tabs.add(nil)
	tab.authSourceURL = "https://first.test/start"
	tab.authURL, tab.authLoading = "https://second.test/private", true
	if got := a.handleAuthChallenge(tab, nil, mygo.AuthRequest{Host: "second.test", Port: 443, Scheme: "basic", Realm: "new"}); got != (mygo.AuthResponse{}) {
		t.Fatal("跨来源质询不应获得旧凭据")
	}
	select {
	case <-shown:
	case <-time.After(time.Second):
		t.Fatal("跨来源认证没有要求用户输入")
	}
	select {
	case <-resolved:
		t.Fatal("跨来源认证自动读取了现有凭据")
	default:
	}
	a.cancelTabAuth(tab)
}
