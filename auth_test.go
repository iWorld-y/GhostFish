package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
			got := probeAuth(srv.URL)
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
	if got := probeAuth(withCred); got != nil {
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
	if got := probeAuth(srv.URL); got == nil {
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

	ok, status, err := verifyCredential(srv.URL, &mygo.Credential{Username: "user", Password: "pass"})
	if err != nil || !ok || status != http.StatusOK {
		t.Fatalf("verifyCredential(right) = (%v, %d, %v), want (true, 200, nil)", ok, status, err)
	}
	ok, status, err = verifyCredential(srv.URL, &mygo.Credential{Username: "user", Password: "wrong"})
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
