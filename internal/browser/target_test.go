package browser

import "testing"

func TestTargetURL(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"完整网址", []string{"https://example.com/a?b=1"}, "https://example.com/a?b=1"},
		{"裸域名补 https", []string{"example.com"}, "https://example.com"},
		{"裸域名带路径", []string{"example.com/docs"}, "https://example.com/docs"},
		{"http 保留", []string{"http://example.com"}, "http://example.com"},
		{"多余参数只取第一个", []string{"example.com", "https://other.test"}, "https://example.com"},
		{"首尾空格", []string{"  example.com  "}, "https://example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TargetURL(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("targetURL(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestTargetURLErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"空参数", []string{""}},
		{"只有空格", []string{"   "}},
		{"不支持的协议", []string{"ftp://example.com"}},
		{"缺少主机名", []string{"https://"}},
		{"包含空格", []string{"exa mple.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := TargetURL(tt.args); err == nil {
				t.Fatalf("targetURL(%q) = %q, want error", tt.args, got)
			}
		})
	}
}

func TestTargetURLOmitted(t *testing.T) {
	got, err := TargetURL(nil)
	if err != nil || got != "" {
		t.Fatalf("targetURL(nil) = %q, %v; want empty target without error", got, err)
	}
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"example.com/docs", "https://example.com/docs"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"https://example.com", "https://example.com"},
		{"example.com/login?next=https://other.example/path", "https://example.com/login?next=https://other.example/path"},
		{"example.com/path/https://other.example", "https://example.com/path/https://other.example"},
	}
	for _, tt := range tests {
		got, err := parseURL(tt.input)
		if err != nil || got != tt.want {
			t.Errorf("parseURL(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
		}
	}
	for _, input := range []string{"", " ", "ftp://example.com", "https://"} {
		if got, err := parseURL(input); err == nil {
			t.Errorf("parseURL(%q) = %q; want error", input, got)
		}
	}
}

func TestParseURLSchemesAndPorts(t *testing.T) {
	for _, input := range []string{"mailto:user@example.com", "javascript:alert(1)", "data:text/plain,hello", "tel:123"} {
		if got, err := parseURL(input); err == nil {
			t.Errorf("parseURL(%q) = %q; want unsupported scheme error", input, got)
		}
	}
	for _, tt := range []struct{ input, want string }{
		{"localhost:8080", "https://localhost:8080"},
		{"example.com:8443/path", "https://example.com:8443/path"},
	} {
		got, err := parseURL(tt.input)
		if err != nil || got != tt.want {
			t.Errorf("parseURL(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
		}
	}
}
