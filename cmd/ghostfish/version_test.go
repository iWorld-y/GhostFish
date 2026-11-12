package main

import "testing"

func TestDisplayVersion(t *testing.T) {
	for _, tt := range []struct{ build, packaged, want string }{
		{"dev", "", "dev"},
		{"dev", "1.2.3", "v1.2.3"},
		{"dev", "v1.2.3", "v1.2.3"},
		{"v2.0.0", "1.2.3", "v2.0.0"},
	} {
		if got := displayVersion(tt.build, tt.packaged); got != tt.want {
			t.Fatalf("displayVersion(%q, %q) = %q, want %q", tt.build, tt.packaged, got, tt.want)
		}
	}
}
