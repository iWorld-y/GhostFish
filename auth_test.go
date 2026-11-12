package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestParseChallenge(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantSchem string
		wantRealm string
	}{
		{"带 charset", `Basic realm="Luker", charset="UTF-8"`, "Basic", "Luker"},
		{"只有 realm", `Basic realm="Luker"`, "Basic", "Luker"},
		{"Digest", `Digest realm="admin", qop="auth"`, "Digest", "admin"},
		{"realm 没有引号", `Basic realm=Luker`, "Basic", "Luker"},
		{"没有 realm", `Basic`, "Basic", ""},
		{"空头部", ``, "", ""},
		{"大小写", `basic REALM="Luker"`, "basic", "Luker"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme, realm := parseChallenge(tt.header)
			if scheme != tt.wantSchem || realm != tt.wantRealm {
				t.Fatalf("parseChallenge(%q) = (%q, %q), want (%q, %q)",
					tt.header, scheme, realm, tt.wantSchem, tt.wantRealm)
			}
		})
	}
}

func TestProbeAuth(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		header  string
		wantNil bool
	}{
		{"需要 Basic", http.StatusUnauthorized, `Basic realm="Luker"`, false},
		{"需要 Digest", http.StatusUnauthorized, `Digest realm="Luker", qop="auth"`, true},
		{"不需要认证", http.StatusOK, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.header != "" {
					w.Header().Set("WWW-Authenticate", tt.header)
				}
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()
			got := probeAuth(context.Background(), srv.URL)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("probeAuth = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("probeAuth = nil, want a challenge")
			}
			if got.Host != "127.0.0.1" {
				t.Fatalf("host = %q, want 127.0.0.1 without the port", got.Host)
			}
			if got.Realm != "Luker" {
				t.Fatalf("realm = %q, want Luker", got.Realm)
			}
		})
	}
}

func TestProbeAuthSkipsURLWithCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("probeAuth should not request a URL that already carries credentials")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	withCred := strings.Replace(srv.URL, "http://", "http://user:pass@", 1)
	if got := probeAuth(context.Background(), withCred); got != nil {
		t.Fatalf("probeAuth = %+v, want nil for a URL with credentials", got)
	}
}

func TestProbeUsesGET(t *testing.T) {
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.Header().Set("WWW-Authenticate", `Basic realm="Luker"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	if got := probeAuth(context.Background(), srv.URL); got == nil {
		t.Fatalf("probeAuth = nil, methods seen: %v", methods)
	}
	if len(methods) != 1 || methods[0] != http.MethodGet {
		t.Fatalf("methods = %v, want exactly one GET", methods)
	}
}

func TestVerifyCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "pass" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Luker"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ok, status, err := verifyCredential(context.Background(), srv.URL, &mygo.AuthResponse{Username: "user", Password: "pass"})
	if err != nil || !ok || status != http.StatusOK {
		t.Fatalf("verifyCredential(right) = (%v, %d, %v), want (true, 200, nil)", ok, status, err)
	}
	ok, status, err = verifyCredential(context.Background(), srv.URL, &mygo.AuthResponse{Username: "user", Password: "wrong"})
	if err != nil || ok || status != http.StatusUnauthorized {
		t.Fatalf("verifyCredential(wrong) = (%v, %d, %v), want (false, 401, nil)", ok, status, err)
	}
}

func TestCredentialFromEnv(t *testing.T) {
	t.Setenv(authEnv, "user:pa:ss")
	got := credentialFromEnv()
	if got == nil || got.Username != "user" || got.Password != "pa:ss" {
		t.Fatalf("credentialFromEnv = %+v, want user:pa:ss", got)
	}

	t.Setenv(authEnv, "")
	if got := credentialFromEnv(); got != nil {
		t.Fatalf("credentialFromEnv = %+v, want nil for an empty variable", got)
	}

	t.Setenv(authEnv, "no-colon")
	if got := credentialFromEnv(); got != nil {
		t.Fatalf("credentialFromEnv = %+v, want nil without a colon", got)
	}
}

func TestPrepareAuthEmpty(t *testing.T) {
	if got := (&app{}).prepareAuth(context.Background(), ""); got != nil {
		t.Fatalf("prepareAuth empty = %+v; want nil", got)
	}
}

func TestPrepareAuthForAddress(t *testing.T) {
	t.Setenv(authEnv, "user:wrong")
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	got := (&app{}).prepareAuth(context.Background(), srv.URL)
	if got == nil || got.need.Host != "127.0.0.1" || got.credential == nil || got.credential.Username != "user" || got.credential.Password != "wrong" || got.verified {
		t.Fatal("prepareAuth should retain the unverified environment credential for the target")
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d; want one probe and one credential validation", got)
	}
}

func TestSerialPrompt(t *testing.T) {
	var running atomic.Int32
	var overlap atomic.Bool
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			serialPrompt(context.Background(), func() *mygo.AuthResponse {
				if running.Add(1) != 1 {
					overlap.Store(true)
				}
				time.Sleep(5 * time.Millisecond)
				running.Add(-1)
				return nil
			})
		}()
	}
	close(start)
	wg.Wait()
	if overlap.Load() {
		t.Fatal("terminal prompts overlapped")
	}
}

func TestCanceledPromptDoesNotStartAfterWaiting(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		serialPrompt(context.Background(), func() *mygo.AuthResponse {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	result := make(chan *mygo.AuthResponse, 1)
	var prompted atomic.Bool
	go func() {
		close(started)
		result <- serialPrompt(ctx, func() *mygo.AuthResponse {
			prompted.Store(true)
			return &mygo.AuthResponse{Username: "unexpected"}
		})
	}()
	<-started
	cancel()
	close(release)
	<-firstDone
	if got := <-result; got != nil || prompted.Load() {
		t.Fatal("a canceled request must not start a queued terminal prompt")
	}
}

func TestCanceledPromptDiscardsCompletedRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	entered, release := make(chan struct{}), make(chan struct{})
	result := make(chan *mygo.AuthResponse, 1)
	go func() {
		result <- serialPrompt(ctx, func() *mygo.AuthResponse {
			close(entered)
			<-release
			return &mygo.AuthResponse{Username: "stale"}
		})
	}()
	<-entered
	cancel()
	close(release)
	if got := <-result; got != nil {
		t.Fatal("credentials from a canceled prompt must be discarded")
	}
}

func TestCanceledProbeReturnsWithoutCredential(t *testing.T) {
	entered := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan *preparedAuth, 1)
	go func() {
		result <- (&app{}).prepareAuth(ctx, srv.URL)
	}()
	<-entered
	cancel()
	select {
	case cred := <-result:
		if cred != nil {
			t.Fatal("canceled probe returned credentials")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled probe did not return")
	}
}

// 没有终端时不得等待输入。
func TestPromptCredentialWithoutTerminal(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	original := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = original }()
	result := make(chan *mygo.AuthResponse, 1)
	go func() {
		result <- promptCredential(context.Background(), &authNeed{Host: "example.test", Realm: "test"})
	}()
	select {
	case cred := <-result:
		if cred != nil {
			t.Fatal("没有终端时不应得到凭据")
		}
	case <-time.After(time.Second):
		writer.Close()
		<-result
		t.Fatal("没有终端时等待了输入")
	}
}

// 环境变量可用时不需要读取系统钥匙串。
func TestResolveCredentialUsesEnvironmentFirst(t *testing.T) {
	t.Setenv(authEnv, "local-user:local-pass")
	got := resolveCredential(context.Background(), &authNeed{Host: "example.test"})
	if got == nil || got.Username != "local-user" || got.Password != "local-pass" {
		t.Fatal("没有优先使用环境变量凭据")
	}
}

type memoryAuthStore struct {
	entries map[string][]byte
	reads   []string
}

func (m *memoryAuthStore) Get(key string) ([]byte, error) {
	m.reads = append(m.reads, key)
	value, ok := m.entries[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return append([]byte(nil), value...), nil
}

func (m *memoryAuthStore) Set(key string, value []byte) error {
	m.entries[key] = append([]byte(nil), value...)
	return nil
}

// 钥匙串只按完整保护空间复用，旧的仅主机条目保留但不自动读取。
func TestCredentialStoreProtectionSpace(t *testing.T) {
	store := &memoryAuthStore{entries: map[string][]byte{"http-auth:example.test": []byte(`{"Username":"legacy","Password":"old"}`)}}
	base, ok := authNeedForURL("https://EXAMPLE.test/private", "members")
	if !ok {
		t.Fatal("无法解析保护空间")
	}
	cred := &mygo.AuthResponse{Username: "local-user", Password: "local-pass"}
	if err := saveCredential(store, base, cred); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, url, realm string
		match            bool
	}{
		{"默认端口等价", "https://example.test:443/other", "members", true},
		{"不同端口", "https://example.test:8443/private", "members", false},
		{"HTTPS 到 HTTP", "http://example.test:443/private", "members", false},
		{"不同 realm", "https://example.test/private", "other", false},
		{"不同主机", "https://other.test/private", "members", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			need, ok := authNeedForURL(tt.url, tt.realm)
			if !ok {
				t.Fatal("无法解析保护空间")
			}
			got := credentialFromKeychain(store, need)
			if (got != nil) != tt.match {
				t.Fatalf("保护空间匹配结果错误：%v", got != nil)
			}
		})
	}
	for _, key := range store.reads {
		if key == "http-auth:example.test" {
			t.Fatal("不应读取旧的仅主机条目")
		}
	}
	if string(store.entries["http-auth:example.test"]) != `{"Username":"legacy","Password":"old"}` {
		t.Fatal("不应修改旧的仅主机条目")
	}
}

// 本机服务确认错误保护空间不会在校验请求中收到 Authorization。
func TestPrepareAuthDoesNotSendOtherProtectionSpaceCredential(t *testing.T) {
	for _, tt := range []struct {
		name, storedScheme, storedRealm string
		differentPort, legacy, wantSend bool
	}{
		{"相同保护空间", "http", "members", false, false, true},
		{"不同端口", "http", "members", true, false, false},
		{"HTTPS 凭据到 HTTP", "https", "members", false, false, false},
		{"不同 realm", "http", "other", false, false, false},
		{"旧的仅主机条目", "http", "members", false, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var authorized atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					authorized.Add(1)
					w.WriteHeader(http.StatusOK)
					return
				}
				w.Header().Set("WWW-Authenticate", `Basic realm="members"`)
				w.WriteHeader(http.StatusUnauthorized)
			}))
			defer server.Close()
			u, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			u.Scheme = tt.storedScheme
			if tt.differentPort {
				u.Host = net.JoinHostPort(u.Hostname(), "1")
			}
			need, ok := authNeedForURL(u.String(), tt.storedRealm)
			if !ok {
				t.Fatal("无法解析已保存保护空间")
			}
			store := &memoryAuthStore{entries: make(map[string][]byte)}
			if tt.legacy {
				store.entries["http-auth:"+need.Host] = []byte(`{"Username":"local-user","Password":"local-pass"}`)
			} else if err := saveCredential(store, need, &mygo.AuthResponse{Username: "local-user", Password: "local-pass"}); err != nil {
				t.Fatal(err)
			}
			a := &app{authHooks: &authHooks{resolve: func(_ context.Context, need *authNeed) *mygo.AuthResponse {
				return credentialFromKeychain(store, *need)
			}}}
			prepared := a.prepareAuth(context.Background(), server.URL)
			if prepared == nil {
				t.Fatal("应识别目标站点的 Basic 质询")
			}
			if got := authorized.Load() > 0; got != tt.wantSend {
				t.Fatalf("授权头发送结果不符合预期：%v", got)
			}
			if (prepared.credential != nil) != tt.wantSend {
				t.Fatal("读取的凭据与保护空间不符")
			}
		})
	}
}
