package browser

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"ghostfish/internal/auth"

	"github.com/egoist/mygo"
)

func TestPrepareAuthEmpty(t *testing.T) {
	if got := (&Browser{}).prepareAuth(context.Background(), ""); got != nil {
		t.Fatalf("prepareAuth empty = %+v; want nil", got)
	}
}

func TestPrepareAuthForAddress(t *testing.T) {
	t.Setenv("GHOSTFISH_AUTH", "user:wrong")
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	got := (&Browser{}).prepareAuth(context.Background(), srv.URL)
	if got == nil || got.need.Host != "127.0.0.1" || got.credential == nil || got.credential.Username != "user" || got.credential.Password != "wrong" || got.verified {
		t.Fatal("prepareAuth should retain the unverified environment credential for the target")
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d; want one probe and one credential validation", got)
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
		result <- (&Browser{}).prepareAuth(ctx, srv.URL)
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
			need, ok := auth.NeedForURL(u.String(), tt.storedRealm)
			if !ok {
				t.Fatal("无法解析已保存保护空间")
			}
			store := &memoryAuthStore{entries: make(map[string][]byte)}
			if tt.legacy {
				store.entries["http-auth:"+need.Host] = []byte(`{"Username":"local-user","Password":"local-pass"}`)
			} else if err := auth.Save(store, need, &mygo.AuthResponse{Username: "local-user", Password: "local-pass"}); err != nil {
				t.Fatal(err)
			}
			a := &Browser{authHooks: &authHooks{resolve: func(_ context.Context, need *auth.Need) *mygo.AuthResponse {
				return auth.FromKeychain(store, *need)
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

// memoryAuthStore 隔离导航测试与真实系统钥匙串。
type memoryAuthStore struct{ entries map[string][]byte }

func (s *memoryAuthStore) Get(key string) ([]byte, error)     { return s.entries[key], nil }
func (s *memoryAuthStore) Set(key string, value []byte) error { s.entries[key] = value; return nil }
