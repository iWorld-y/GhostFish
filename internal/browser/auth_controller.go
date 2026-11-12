package browser

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"ghostfish/internal/auth"

	"github.com/egoist/mygo"
)

// preparedAuth 只在当前导航中持有对应保护空间的临时凭据。
type preparedAuth struct {
	need       auth.Need
	url        string
	credential *mygo.AuthResponse
	verified   bool
	attempts   int
}

// authPrompt 是一个标签页的认证请求和输入窗口。
type authPrompt struct {
	tab              *browserTab
	view             *mygo.WebView
	generation       uint64
	url              string
	need             auth.Need
	ctx              context.Context
	cancel           context.CancelFunc
	window           *mygo.Window
	username         string
	password         string
	errorText        string
	ready            bool // 已决定显示输入窗口，可在切回标签时恢复。
	busy             bool
	awaitingDenial   bool
	queuedCredential *mygo.AuthResponse
	queuedVerified   bool
}

// authHooks 让认证流程的外部依赖可在隔离测试中替换。
type authHooks struct {
	resolve  func(context.Context, *auth.Need) *mygo.AuthResponse
	verify   func(context.Context, string, *mygo.AuthResponse) (bool, int, error)
	save     func(auth.Need, *mygo.AuthResponse) error
	dispatch func(func())
	show     func(*authPrompt)
	load     func(*browserTab, string) error
	pageURL  func(*mygo.WebView) string
}

func (a *Browser) resolveAuth(ctx context.Context, need *auth.Need) *mygo.AuthResponse {
	if a.authHooks != nil && a.authHooks.resolve != nil {
		return a.authHooks.resolve(ctx, need)
	}
	return auth.Resolve(ctx, need)
}

func (a *Browser) verifyAuth(ctx context.Context, rawURL string, cred *mygo.AuthResponse) (bool, int, error) {
	if a.authHooks != nil && a.authHooks.verify != nil {
		return a.authHooks.verify(ctx, rawURL, cred)
	}
	return auth.Verify(ctx, rawURL, cred)
}

func (a *Browser) saveAuth(need auth.Need, cred *mygo.AuthResponse) error {
	if a.authHooks != nil && a.authHooks.save != nil {
		return a.authHooks.save(need, cred)
	}
	return auth.Save(mygo.Secrets, need, cred)
}

func (a *Browser) dispatchAuth(fn func()) {
	if a.authHooks != nil && a.authHooks.dispatch != nil {
		a.authHooks.dispatch(fn)
		return
	}
	mygo.RunOnMain(fn)
}

func (a *Browser) showAuth(p *authPrompt) {
	if !a.validAuthPrompt(p) {
		return
	}
	p.ready = true
	if a.tabs.active != p.tab {
		return
	}
	if p.window != nil {
		if !p.window.IsVisible() {
			p.window.Show()
		}
		return
	}
	if a.authHooks != nil && a.authHooks.show != nil {
		a.authHooks.show(p)
		return
	}
	a.openAuthWindow(p)
}

// syncAuthTab 只显示当前标签的认证窗口，隐藏窗口时保留认证状态。
func (a *Browser) syncAuthTab(previous *browserTab) {
	if previous == a.tabs.active {
		return
	}
	if previous != nil && previous.authPrompt != nil && previous.authPrompt.window != nil {
		previous.authPrompt.window.Hide()
	}
	if current := a.tabs.active; current != nil && current.authPrompt != nil && current.authPrompt.ready {
		a.showAuth(current.authPrompt)
	}
}

func (a *Browser) loadAuthURL(tab *browserTab, rawURL string) error {
	if a.authHooks != nil && a.authHooks.load != nil {
		return a.authHooks.load(tab, rawURL)
	}
	return tab.view.Page().LoadURL(rawURL)
}

// matchesAuthURL 限制认证范围到当前主文档的协议、主机和端口。
func matchesAuthURL(rawURL string, req mygo.AuthRequest) bool {
	if req.IsProxy || !strings.EqualFold(req.Scheme, "basic") || req.Host == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.User != nil || !strings.EqualFold(u.Hostname(), req.Host) {
		return false
	}
	var defaultPort int
	switch strings.ToLower(u.Scheme) {
	case "http":
		defaultPort = 80
	case "https":
		defaultPort = 443
	default:
		return false
	}
	port := defaultPort
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return false
		}
	}
	return req.Port == port || (req.Port == 0 && port == defaultPort)
}

func sameAuthOrigin(left, right string) bool {
	l, err := url.Parse(left)
	if err != nil || l.User != nil {
		return false
	}
	r, err := url.Parse(right)
	if err != nil || r.User != nil || !strings.EqualFold(l.Scheme, r.Scheme) {
		return false
	}
	port := 0
	if r.Port() != "" {
		port, err = strconv.Atoi(r.Port())
		if err != nil {
			return false
		}
	}
	return matchesAuthURL(left, mygo.AuthRequest{Host: r.Hostname(), Port: port, Scheme: "basic"})
}

// sameAuthDocument 避免把旧版 WebKitGTK 的子框架导航误当成主文档。
func sameAuthDocument(expected, actual string) bool {
	left, err := url.Parse(expected)
	if err != nil || left.User != nil {
		return false
	}
	right, err := url.Parse(actual)
	if err != nil || right.User != nil || !sameAuthOrigin(expected, actual) {
		return false
	}
	leftPath, rightPath := left.EscapedPath(), right.EscapedPath()
	if leftPath == "" {
		leftPath = "/"
	}
	if rightPath == "" {
		rightPath = "/"
	}
	return leftPath == rightPath && left.RawQuery == right.RawQuery
}

// authDocumentAllowed 允许已确认的主文档导航使用尚未刷新的页面地址。
func authDocumentAllowed(expected, current string, mainFrame bool) bool {
	return current == "" || current == "about:blank" || sameAuthDocument(expected, current) || mainFrame
}

// installAuthHandler 同步回答 WebView 质询；需要交互时立即取消并在后台准备窗口。
func (a *Browser) installAuthHandler(tab *browserTab, view *mygo.WebView, rawURL string) {
	u, _ := url.Parse(rawURL)
	if u != nil && u.User != nil {
		view.Page().SetAuthHandler(nil)
		return
	}
	view.Page().SetAuthHandler(func(req mygo.AuthRequest) mygo.AuthResponse {
		return a.handleAuthChallenge(tab, view, req)
	})
}

func (a *Browser) handleAuthChallenge(tab *browserTab, view *mygo.WebView, req mygo.AuthRequest) mygo.AuthResponse {
	if tab.view != view || !a.tabs.accepts(tab, tab.generation) || !tab.authLoading || !matchesAuthURL(tab.authURL, req) {
		return mygo.AuthResponse{}
	}
	if view != nil {
		current := view.Page().URL()
		if a.authHooks != nil && a.authHooks.pageURL != nil {
			current = a.authHooks.pageURL(view)
		}
		if !authDocumentAllowed(tab.authURL, current, tab.authMainFrame) {
			return mygo.AuthResponse{}
		}
	}
	prepared := tab.authPrepared
	if prepared != nil && prepared.credential != nil && sameAuthOrigin(prepared.url, tab.authURL) &&
		strings.EqualFold(prepared.need.Host, req.Host) && prepared.need.Realm == req.Realm {
		if prepared.attempts == 0 {
			prepared.attempts++
			return *prepared.credential
		}
		tab.authPrepared = nil
		if p := tab.authPrompt; p != nil {
			p.busy = false
			p.awaitingDenial = true
			p.password = ""
			p.errorText = "账号或密码不正确，请重试。"
			go a.dispatchAuth(func() {
				if !a.validAuthPrompt(p) {
					return
				}
				if p.window == nil {
					a.showAuth(p)
				} else {
					a.invalidateAuth(p)
				}
			})
			return mygo.AuthResponse{}
		}
	}
	if tab.authPrompt != nil {
		return mygo.AuthResponse{}
	}
	need, ok := auth.NeedForURL(tab.authURL, req.Realm)
	if !ok {
		return mygo.AuthResponse{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &authPrompt{tab: tab, view: view, generation: tab.generation, url: tab.authURL,
		need: need, ctx: ctx, cancel: cancel, awaitingDenial: true}
	tab.authPrompt = p
	tried := prepared != nil && sameAuthOrigin(prepared.url, p.url) &&
		strings.EqualFold(prepared.need.Host, req.Host) && prepared.need.Realm == req.Realm
	if tried && prepared.credential != nil && prepared.attempts > 0 {
		p.errorText = "账号或密码不正确，请重试。"
	}
	// 重定向到其他源时，不自动把原网址的环境变量或钥匙串凭据交给新主机。
	if tried || tab.authSourceURL == "" || !sameAuthOrigin(tab.authSourceURL, p.url) {
		go a.dispatchAuth(func() {
			if a.validAuthPrompt(p) {
				a.showAuth(p)
			}
		})
	} else {
		go a.resolveChallenge(p)
	}
	return mygo.AuthResponse{}
}

// resolveChallenge 保留环境变量、钥匙串、终端的优先顺序。
func (a *Browser) resolveChallenge(p *authPrompt) {
	cred := a.resolveAuth(p.ctx, &p.need)
	if p.ctx.Err() != nil {
		return
	}
	if cred == nil {
		a.dispatchAuth(func() {
			if a.validAuthPrompt(p) {
				a.showAuth(p)
			}
		})
		return
	}
	ok, _, err := a.verifyAuth(p.ctx, p.url, cred)
	if p.ctx.Err() != nil {
		return
	}
	a.dispatchAuth(func() {
		if !a.validAuthPrompt(p) {
			return
		}
		a.retryAuth(p, cred, err == nil && ok)
	})
}

func (a *Browser) validAuthPrompt(p *authPrompt) bool {
	return p != nil && p.ctx.Err() == nil && p.tab.authPrompt == p && p.tab.view == p.view && a.tabs.accepts(p.tab, p.generation)
}

// submitAuth 验证用户输入；网络探测无法判断时仍让 WebView 处理实际质询。
func (a *Browser) submitAuth(p *authPrompt) {
	if !a.validAuthPrompt(p) || p.busy {
		return
	}
	if strings.TrimSpace(p.username) == "" {
		p.errorText = "请输入用户名。"
		a.invalidateAuth(p)
		return
	}
	cred := &mygo.AuthResponse{Username: p.username, Password: p.password}
	p.busy = true
	p.errorText = ""
	a.invalidateAuth(p)
	go func() {
		ok, _, err := a.verifyAuth(p.ctx, p.url, cred)
		if p.ctx.Err() != nil {
			return
		}
		a.dispatchAuth(func() {
			if !a.validAuthPrompt(p) {
				return
			}
			a.retryAuth(p, cred, err == nil && ok)
		})
	}()
}

func (a *Browser) retryAuth(p *authPrompt, cred *mygo.AuthResponse, verified bool) {
	if !a.validAuthPrompt(p) {
		return
	}
	p.busy = true
	if p.awaitingDenial {
		p.queuedCredential = cred
		p.queuedVerified = verified
		return
	}
	p.tab.authPrepared = &preparedAuth{need: p.need, url: p.url, credential: cred, verified: verified}
	p.tab.authURL = p.url
	p.tab.authLoading = true
	p.tab.authMainFrame = true
	if err := a.loadAuthURL(p.tab, p.url); err != nil {
		p.tab.authMainFrame = false
		p.tab.authPrepared = nil
		p.busy = false
		p.errorText = "无法重新加载页面，请重试。"
	}
	a.invalidateAuth(p)
}

func (a *Browser) invalidateAuth(p *authPrompt) {
	if a.validAuthPrompt(p) && p.window != nil {
		p.window.Invalidate()
	}
}

// finishAuthLoad 等待取消质询的旧导航结束，再按结果处理当前认证。
func (a *Browser) finishAuthLoad(tab *browserTab, view *mygo.WebView, failed bool) {
	if tab.view != view || !a.tabs.accepts(tab, tab.generation) {
		return
	}
	tab.authLoading = false
	tab.authMainFrame = false
	prepared := tab.authPrepared
	p := tab.authPrompt
	if p != nil && p.awaitingDenial {
		p.awaitingDenial = false
		if p.queuedCredential != nil {
			cred, verified := p.queuedCredential, p.queuedVerified
			p.queuedCredential = nil
			a.retryAuth(p, cred, verified)
		}
		return
	}
	if failed && p != nil && p.busy {
		tab.authPrepared = nil
		p.busy = false
		p.errorText = "无法加载认证页面，请重试。"
		a.invalidateAuth(p)
		return
	}
	if prepared == nil || prepared.credential == nil || prepared.attempts == 0 {
		return
	}
	a.saveVerifiedAuth(tab, prepared)
	if p != nil {
		a.cancelTabAuth(tab)
	} else {
		tab.authPrepared = nil
	}
}

// saveVerifiedAuth 只保存通过校验且已提交给 WebView 的凭据。
func (a *Browser) saveVerifiedAuth(tab *browserTab, prepared *preparedAuth) {
	if prepared == nil || prepared.credential == nil || !prepared.verified || prepared.attempts == 0 {
		return
	}
	cred := *prepared.credential
	need := prepared.need
	ctx, cancel := context.WithCancel(context.Background())
	tab.cancelNavigation = cancel
	go func() {
		defer cancel()
		if ctx.Err() == nil {
			if err := a.saveAuth(need, &cred); err != nil {
				fmt.Fprintln(os.Stderr, "ghostfish: 无法保存认证凭据")
			}
		}
	}()
}

// cancelTabAuth 关闭输入窗口并丢弃凭据和未完成的异步结果。
func (a *Browser) cancelTabAuth(tab *browserTab) {
	tab.authLoading = false
	tab.authMainFrame = false
	p := tab.authPrompt
	tab.authPrompt = nil
	tab.authPrepared = nil
	if p == nil {
		return
	}
	p.cancel()
	p.username, p.password = "", ""
	p.queuedCredential = nil
	if p.window != nil {
		p.window.Close()
	}
}

func authHostLabel(p *authPrompt) string {
	u, err := url.Parse(p.url)
	if err != nil {
		return p.need.Host
	}
	if u.Port() != "" {
		return net.JoinHostPort(p.need.Host, u.Port())
	}
	return p.need.Host
}
