package desktop

import (
	"testing"

	cfg "ghostfish/internal/config"
)

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
			got, notes := guardRecovery(tt.registered, cfg.State{Opacity: .85, ClickThrough: tt.through}, false)
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
