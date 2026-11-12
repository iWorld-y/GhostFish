package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"golang.org/x/term"
)

const (
	// authEnv 让用户不落磁盘地给凭据，格式 user:pass。
	authEnv = "GHOSTFISH_AUTH"
	// secretPrefix 使用独立版本，避免自动读取无法确认保护空间的旧条目。
	secretPrefix = "http-auth:v2:"
	probeTimeout = 8 * time.Second
	// probeUserAgent 是探测请求用的身份，和后端 webview 的默认 UA 一致。
	probeUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko)"
)

// authNeed 是探测到的认证需求。
type authNeed struct {
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

// probeAuth 先自己看一次目标站点，是否需要 HTTP Basic 认证。
// 不需要、拿不准、或网址里已经带凭据时返回 nil，把决定交给 webview。
func probeAuth(ctx context.Context, rawURL string) *authNeed {
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
	need, ok := authNeedForURL(rawURL, realm)
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

// verifyCredential 拿凭据真请求一次，返回是否通过和看到的状态码，
// 状态码用来在失败时告诉用户服务器回了什么。
func verifyCredential(ctx context.Context, rawURL string, cred *mygo.AuthResponse) (bool, int, error) {
	resp, err := probe(ctx, rawURL, cred)
	if err != nil {
		return false, 0, err
	}
	defer drain(resp)
	return resp.StatusCode >= 200 && resp.StatusCode < 400, resp.StatusCode, nil
}

// resolveCredential 依次尝试环境变量、系统钥匙串、终端输入。
func resolveCredential(ctx context.Context, need *authNeed) *mygo.AuthResponse {
	if ctx.Err() != nil {
		return nil
	}
	if c := credentialFromEnv(); c != nil {
		fmt.Fprintln(os.Stderr, "ghostfish: 用环境变量里的凭据")
		return c
	}
	if ctx.Err() != nil {
		return nil
	}
	if c := credentialFromKeychain(mygo.Secrets, *need); c != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: 用钥匙串里 %s 的凭据\n", need.Host)
		return c
	}
	return promptCredential(ctx, need)
}

func credentialFromEnv() *mygo.AuthResponse {
	raw := strings.TrimSpace(os.Getenv(authEnv))
	if raw == "" {
		return nil
	}
	user, pass, ok := strings.Cut(raw, ":")
	if !ok || user == "" {
		return nil
	}
	return &mygo.AuthResponse{Username: user, Password: pass}
}

// authSecretStore 对应系统凭据存储；测试使用内存替身。
type authSecretStore interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
}

// authNeedForURL 将网址归一化为包含协议、主机、端口和 realm 的保护空间。
func authNeedForURL(rawURL, realm string) (authNeed, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.User != nil || u.Hostname() == "" {
		return authNeed{}, false
	}
	scheme := strings.ToLower(u.Scheme)
	port := 0
	switch scheme {
	case "http":
		port = 80
	case "https":
		port = 443
	default:
		return authNeed{}, false
	}
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return authNeed{}, false
		}
	}
	return authNeed{Scheme: scheme, Host: strings.ToLower(u.Hostname()), Port: port, Realm: realm}, true
}

// secretKey 使用固定长度摘要，避免很长的 realm 超出系统存储键长度限制。
func secretKey(need authNeed) (string, error) {
	if (need.Scheme != "http" && need.Scheme != "https") || need.Host == "" || need.Port < 1 || need.Port > 65535 {
		return "", fmt.Errorf("无效的认证保护空间")
	}
	data, err := json.Marshal(need)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return secretPrefix + hex.EncodeToString(digest[:]), nil
}

func credentialFromKeychain(store authSecretStore, need authNeed) *mygo.AuthResponse {
	key, err := secretKey(need)
	if err != nil {
		return nil
	}
	raw, err := store.Get(key)
	if err != nil {
		return nil
	}
	var c mygo.AuthResponse
	if err := json.Unmarshal(raw, &c); err != nil || c.Username == "" {
		return nil
	}
	return &c
}

func saveCredential(store authSecretStore, need authNeed, cred *mygo.AuthResponse) error {
	key, err := secretKey(need)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	return store.Set(key, raw)
}

// promptMu 防止多个标签页同时向同一终端读取凭据。
var promptMu sync.Mutex

// serialPrompt 按顺序执行终端输入，避免不同站点的提示交错。
func serialPrompt(ctx context.Context, read func() *mygo.AuthResponse) *mygo.AuthResponse {
	promptMu.Lock()
	defer promptMu.Unlock()
	if ctx.Err() != nil {
		return nil
	}
	cred := read()
	if ctx.Err() != nil {
		return nil
	}
	return cred
}

// promptCredential 在终端里问用户名和密码，密码不回显。
func promptCredential(ctx context.Context, need *authNeed) *mygo.AuthResponse {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil
	}
	return serialPrompt(ctx, func() *mygo.AuthResponse {
		fmt.Fprintf(os.Stderr, "%s（realm %q）需要登录\n用户名：", need.Host, need.Realm)
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return nil
		}
		user := strings.TrimSpace(line)
		if user == "" || ctx.Err() != nil {
			return nil
		}
		fmt.Fprint(os.Stderr, "密码（不回显）：")
		pass, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return nil
		}
		return &mygo.AuthResponse{Username: user, Password: string(pass)}
	})
}

// prepareAuth 在后台探测并查找凭据，不在验证成功之前保存。
func (a *app) prepareAuth(ctx context.Context, rawURL string) *preparedAuth {
	if rawURL == "" || ctx.Err() != nil {
		return nil
	}
	need := probeAuth(ctx, rawURL)
	if need == nil || ctx.Err() != nil {
		return nil
	}
	cred := a.resolveAuth(ctx, need)
	if cred == nil || ctx.Err() != nil {
		return &preparedAuth{need: *need, url: rawURL}
	}
	ok, _, err := a.verifyAuth(ctx, rawURL, cred)
	if ctx.Err() != nil {
		return nil
	}
	return &preparedAuth{need: *need, url: rawURL, credential: cred, verified: err == nil && ok}
}
