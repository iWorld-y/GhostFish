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

func TestGuardRecovery(t *testing.T) {
	tests := []struct {
		name        string
		registered  [4]bool
		through     bool
		wantThrough bool
		wantNotes   int
	}{
		{"全部注册", [4]bool{true, true, true, true}, true, true, 0},
		{"穿透快捷键失败", [4]bool{true, true, false, true}, true, false, 1},
		{"隐藏快捷键失败", [4]bool{true, true, true, false}, true, true, 1},
		{"只剩不透明度快捷键", [4]bool{true, true, false, false}, true, false, 2},
		{"全部失败", [4]bool{}, true, false, 2},
		{"本来没开穿透", [4]bool{true, true, false, true}, false, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, notes := guardRecovery(tt.registered, state{Opacity: .85, ClickThrough: tt.through})
			if got.ClickThrough != tt.wantThrough {
				t.Fatalf("ClickThrough = %v, want %v", got.ClickThrough, tt.wantThrough)
			}
			if len(notes) != tt.wantNotes {
				t.Fatalf("notes = %v, want %d", notes, tt.wantNotes)
			}
			if got.Opacity != .85 {
				t.Fatalf("opacity changed to %v", got.Opacity)
			}
		})
	}
}
