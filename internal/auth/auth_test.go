package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
			got := Probe(context.Background(), srv.URL)
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
	if got := Probe(context.Background(), withCred); got != nil {
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
	if got := Probe(context.Background(), srv.URL); got == nil {
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

	ok, status, err := Verify(context.Background(), srv.URL, &mygo.AuthResponse{Username: "user", Password: "pass"})
	if err != nil || !ok || status != http.StatusOK {
		t.Fatalf("verifyCredential(right) = (%v, %d, %v), want (true, 200, nil)", ok, status, err)
	}
	ok, status, err = Verify(context.Background(), srv.URL, &mygo.AuthResponse{Username: "user", Password: "wrong"})
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
		result <- promptCredential(context.Background(), &Need{Host: "example.test", Realm: "test"})
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
	got := Resolve(context.Background(), &Need{Host: "example.test"})
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
	base, ok := NeedForURL("https://EXAMPLE.test/private", "members")
	if !ok {
		t.Fatal("无法解析保护空间")
	}
	cred := &mygo.AuthResponse{Username: "local-user", Password: "local-pass"}
	if err := Save(store, base, cred); err != nil {
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
			need, ok := NeedForURL(tt.url, tt.realm)
			if !ok {
				t.Fatal("无法解析保护空间")
			}
			got := FromKeychain(store, need)
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
