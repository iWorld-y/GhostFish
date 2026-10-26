package main

import "testing"

func TestResolveStartState(t *testing.T) {
	tests := []struct {
		name          string
		registered    int
		in            state
		wantThrough   bool
		wantForcedOff bool
	}{
		{"快捷键可用时保留穿透", 4, state{Opacity: 0.9, ClickThrough: true}, true, false},
		{"快捷键全挂时强制关穿透", 0, state{Opacity: 0.9, ClickThrough: true}, false, true},
		{"快捷键全挂但本来没穿透", 0, state{Opacity: 0.9}, false, false},
		{"快捷键可用且没穿透", 2, state{Opacity: 0.9}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, forced := resolveStartState(tt.registered, tt.in)
			if got.ClickThrough != tt.wantThrough || forced != tt.wantForcedOff {
				t.Fatalf("resolveStartState = (%+v, %v), want (through %v, forced %v)",
					got, forced, tt.wantThrough, tt.wantForcedOff)
			}
			if got.Opacity != tt.in.Opacity {
				t.Fatalf("opacity changed to %v", got.Opacity)
			}
		})
	}
}
