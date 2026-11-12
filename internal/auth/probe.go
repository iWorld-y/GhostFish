// Package auth 处理 HTTP Basic 认证探测和凭据来源。
package auth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

const (
	probeTimeout = 8 * time.Second
	// probeUserAgent 是探测请求用的身份，和后端 webview 的默认 UA 一致。
	probeUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko)"
)

// Need 是探测到的认证需求。
type Need struct {
	Scheme string
	Host   string
	Port   int
	Realm  string
}

// probe 用 GET 请求一次网址，和浏览器做的事情一致。
// 不跟随跳转，否则 401 会被跳转掩盖。
func probe(ctx context.Context, rawURL string, cred *mygo.AuthResponse) (*http.Response, error) {
	client := &http.Client{
		Timeout: probeTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	// 用和 webview 一样的浏览器身份，少招一点按 UA 做的拦截。
	req.Header.Set("User-Agent", probeUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	if cred != nil {
		req.SetBasicAuth(cred.Username, cred.Password)
	}
	return client.Do(req)
}

// drain 读掉响应体开头的一小段再关掉，避免整页下载，也让连接能复用。
func drain(resp *http.Response) {
	io.CopyN(io.Discard, resp.Body, 4096)
	resp.Body.Close()
}

// Probe 先自己看一次目标站点，是否需要 HTTP Basic 认证。
// 不需要、拿不准、或网址里已经带凭据时返回 nil，把决定交给 webview。
func Probe(ctx context.Context, rawURL string) *Need {
	u, err := url.Parse(rawURL)
	if err != nil || u.User != nil {
		return nil
	}
	resp, err := probe(ctx, rawURL, nil)
	if err != nil {
		return nil
	}
	defer drain(resp)
	if resp.StatusCode != http.StatusUnauthorized {
		return nil
	}
	scheme, realm := parseChallenge(resp.Header.Get("WWW-Authenticate"))
	if !strings.EqualFold(scheme, "Basic") {
		return nil
	}
	need, ok := NeedForURL(rawURL, realm)
	if !ok {
		return nil
	}
	return &need
}

// parseChallenge 拆开 WWW-Authenticate，取出认证方式和 realm。
func parseChallenge(header string) (scheme, realm string) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", ""
	}
	parts := strings.SplitN(header, " ", 2)
	scheme = parts[0]
	if len(parts) == 1 {
		return scheme, ""
	}
	rest := parts[1]
	i := strings.Index(strings.ToLower(rest), "realm=")
	if i < 0 {
		return scheme, ""
	}
	v := strings.TrimSpace(rest[i+len("realm="):])
	if strings.HasPrefix(v, `"`) {
		if j := strings.Index(v[1:], `"`); j >= 0 {
			return scheme, v[1 : 1+j]
		}
		return scheme, strings.Trim(v, `"`)
	}
	if k := strings.IndexAny(v, ", "); k >= 0 {
		return scheme, v[:k]
	}
	return scheme, v
}

// Verify 拿凭据真请求一次，返回是否通过和看到的状态码，
// 状态码用来在失败时告诉用户服务器回了什么。
func Verify(ctx context.Context, rawURL string, cred *mygo.AuthResponse) (bool, int, error) {
	resp, err := probe(ctx, rawURL, cred)
	if err != nil {
		return false, 0, err
	}
	defer drain(resp)
	return resp.StatusCode >= 200 && resp.StatusCode < 400, resp.StatusCode, nil
}
