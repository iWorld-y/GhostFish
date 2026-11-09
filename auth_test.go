package main

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestAuthHandler(t *testing.T) {
	cred := &mygo.AuthResponse{Username: "user", Password: "pass"}
	handle := authHandler("example.com", cred)
	requests := []struct {
		name    string
		request mygo.AuthRequest
		want    mygo.AuthResponse
	}{
		{"其他主机不提供凭据", mygo.AuthRequest{Host: "other.example.com", Scheme: "basic"}, mygo.AuthResponse{}},
		{"代理不提供凭据", mygo.AuthRequest{Host: "example.com", Scheme: "basic", IsProxy: true}, mygo.AuthResponse{}},
		{"其他认证方式不提供凭据", mygo.AuthRequest{Host: "example.com", Scheme: "digest"}, mygo.AuthResponse{}},
		{"主机名忽略大小写", mygo.AuthRequest{Host: "EXAMPLE.COM", Scheme: "basic"}, *cred},
		{"相同主机再试一次", mygo.AuthRequest{Host: "example.com", Scheme: "basic"}, *cred},
		{"第三次停止提供", mygo.AuthRequest{Host: "example.com", Scheme: "basic"}, mygo.AuthResponse{}},
		{"后续继续停止提供", mygo.AuthRequest{Host: "example.com", Scheme: "basic"}, mygo.AuthResponse{}},
	}
	for _, req := range requests {
		t.Run(req.name, func(t *testing.T) {
			if got := handle(req.request); got != req.want {
				t.Fatalf("handle(%+v) = %+v, want %+v", req.request, got, req.want)
			}
		})
	}
}

func TestPrepareAuthEmpty(t *testing.T) {
	host, credential := prepareAuth(context.Background(), "")
	if host != "" || credential != nil {
		t.Fatalf("prepareAuth empty = %q, %+v; want no host or credential", host, credential)
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

	host, credential := prepareAuth(context.Background(), srv.URL)
	if host != "127.0.0.1" || credential == nil || credential.Username != "user" || credential.Password != "wrong" {
		t.Fatalf("prepareAuth = %q, %+v; want target host and environment credential", host, credential)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d; want one probe and one credential validation", got)
	}
}

func TestIndependentAuthHandlers(t *testing.T) {
	first := authHandler("first.example", &mygo.AuthResponse{Username: "first", Password: "one"})
	second := authHandler("second.example", &mygo.AuthResponse{Username: "second", Password: "two"})
	firstRequest := mygo.AuthRequest{Host: "first.example", Scheme: "basic"}
	secondRequest := mygo.AuthRequest{Host: "second.example", Scheme: "basic"}
	if got := first(secondRequest); got.Username != "" {
		t.Fatal("first host credentials were offered to a different host")
	}
	first(firstRequest)
	first(firstRequest)
	if got := first(firstRequest); got.Username != "" {
		t.Fatal("first host should stop after two attempts")
	}
	if got := second(secondRequest); got.Username != "second" {
		t.Fatal("first host attempts should not consume another host's retries")
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
	result := make(chan *mygo.AuthResponse, 1)
	go func() {
		_, cred := prepareAuth(ctx, srv.URL)
		result <- cred
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
