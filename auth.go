package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"golang.org/x/term"
)

const (
	// authEnv 让用户不落磁盘地给凭据，格式 user:pass。
	authEnv = "GHOSTFISH_AUTH"
	// secretPrefix 是钥匙串里条目的前缀，后面接主机名。
	secretPrefix = "http-auth:"
	probeTimeout = 8 * time.Second
	// probeUserAgent 是探测请求用的身份，和后端 webview 的默认 UA 一致。
	probeUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko)"
)

// authNeed 是探测到的认证需求。
type authNeed struct {
	Host  string
	Realm string
}

// probe 用 GET 请求一次网址，和浏览器做的事情一致。
// 不跟随跳转，否则 401 会被跳转掩盖。
func probe(rawURL string, cred *mygo.Credential) (*http.Response, error) {
	client := &http.Client{
		Timeout: probeTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
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
func probeAuth(rawURL string) *authNeed {
	u, err := url.Parse(rawURL)
	if err != nil || u.User != nil {
		return nil
	}
	resp, err := probe(rawURL, nil)
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
	// 质询里带的是不含端口的主机名，这里就用它，钥匙串的键也用它。
	return &authNeed{Host: u.Hostname(), Realm: realm}
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
func verifyCredential(rawURL string, cred *mygo.Credential) (bool, int, error) {
	resp, err := probe(rawURL, cred)
	if err != nil {
		return false, 0, err
	}
	defer drain(resp)
	return resp.StatusCode != http.StatusUnauthorized, resp.StatusCode, nil
}

// resolveCredential 依次尝试环境变量、系统钥匙串、终端输入。
func resolveCredential(need *authNeed) *mygo.Credential {
	if c := credentialFromEnv(); c != nil {
		fmt.Fprintln(os.Stderr, "ghostfish: 用环境变量里的凭据")
		return c
	}
	if c := credentialFromKeychain(need.Host); c != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: 用钥匙串里 %s 的凭据\n", need.Host)
		return c
	}
	return promptCredential(need)
}

func credentialFromEnv() *mygo.Credential {
	raw := strings.TrimSpace(os.Getenv(authEnv))
	if raw == "" {
		return nil
	}
	user, pass, ok := strings.Cut(raw, ":")
	if !ok || user == "" {
		return nil
	}
	return &mygo.Credential{Username: user, Password: pass}
}

func secretKey(host string) string { return secretPrefix + host }

func credentialFromKeychain(host string) *mygo.Credential {
	raw, err := mygo.Secrets.Get(secretKey(host))
	if err != nil {
		return nil
	}
	var c mygo.Credential
	if err := json.Unmarshal(raw, &c); err != nil || c.Username == "" {
		return nil
	}
	return &c
}

func saveCredential(host string, cred *mygo.Credential) error {
	raw, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	return mygo.Secrets.Set(secretKey(host), raw)
}

// promptCredential 在终端里问用户名和密码，密码不回显。
func promptCredential(need *authNeed) *mygo.Credential {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil
	}
	fmt.Fprintf(os.Stderr, "%s（realm %q）需要登录\n用户名：", need.Host, need.Realm)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return nil
	}
	user := strings.TrimSpace(line)
	if user == "" {
		return nil
	}
	fmt.Fprint(os.Stderr, "密码（不回显）：")
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil
	}
	return &mygo.Credential{Username: user, Password: string(pass)}
}

// prepareAuth 在开窗口之前把认证准备好，避免窗口出来后卡在 401。
func (a *app) prepareAuth() *mygo.Credential {
	need := probeAuth(a.target)
	if need == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, "ghostfish: %s 要求认证（realm %q）\n", need.Host, need.Realm)
	cred := resolveCredential(need)
	if cred == nil {
		fmt.Fprintln(os.Stderr, "ghostfish: 没拿到凭据，交给 webview 自己处理")
		return nil
	}
	// 校验只是尽力而为：有些服务器对带凭据的探测请求另有脾气，
	// 校验失败也把凭据交给窗口，让真正的页面加载去试。
	ok, status, err := verifyCredential(a.target, cred)
	if !ok {
		if err != nil {
			fmt.Fprintf(os.Stderr, "ghostfish: 凭据校验请求失败（%v），仍然交给窗口试\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "ghostfish: 凭据校验返回 %d，仍然交给窗口试\n", status)
		}
		fmt.Fprintf(os.Stderr, "ghostfish: 如果页面仍是 401，多半是用户名或密码不对\n")
		a.authHost = need.Host
		return cred
	}
	if err := saveCredential(need.Host, cred); err != nil {
		fmt.Fprintf(os.Stderr, "ghostfish: 存进钥匙串失败（下次还要再输）：%v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ghostfish: 凭据已存进钥匙串，下次不用再输\n")
	}
	a.authHost = need.Host
	return cred
}
