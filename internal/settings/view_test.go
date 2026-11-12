package settings

import (
	"testing"

	cfg "ghostfish/internal/config"

	"github.com/egoist/mygo/ui"
)

func TestShortcutFromInput(t *testing.T) {
	binding, ok := shortcutFromInput(ui.InputEvent{Key: ui.KeyT, PhysicalKey: ui.KeyT, Mods: ui.Super | ui.Shift})
	if !ok || binding != (cfg.ShortcutBinding{Key: "T", Command: true, Shift: true}) {
		t.Fatalf("binding=%+v, ok=%v", binding, ok)
	}
	if _, ok := shortcutFromInput(ui.InputEvent{Key: ui.KeyT, Mods: ui.Shift}); ok {
		t.Fatal("modifier required")
	}
	if _, ok := shortcutFromInput(ui.InputEvent{Key: ui.KeyEscape, Mods: ui.Super}); ok {
		t.Fatal("unsupported key accepted")
	}
}
