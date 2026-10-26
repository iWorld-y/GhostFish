package main

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
			got, err := targetURL(tt.args)
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
		{"没有参数", nil},
		{"空参数", []string{""}},
		{"只有空格", []string{"   "}},
		{"不支持的协议", []string{"ftp://example.com"}},
		{"缺少主机名", []string{"https://"}},
		{"包含空格", []string{"exa mple.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := targetURL(tt.args); err == nil {
				t.Fatalf("targetURL(%q) = %q, want error", tt.args, got)
			}
		})
	}
}
