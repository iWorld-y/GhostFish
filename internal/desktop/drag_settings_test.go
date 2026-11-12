package desktop

import (
	"path/filepath"
	"testing"

	cfg "ghostfish/internal/config"

	"github.com/egoist/mygo/ui"
)

func TestDragSettingsDialogSaveAndCancel(t *testing.T) {
	a := settingsTestApp(t)
	tester := ui.NewTester(a.dialog.View, 640, 900)
	if err := tester.Click("拖动修饰键 Command"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("拖动修饰键 Option"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("拖动修饰键 Shift"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("保存"); err != nil {
		t.Fatal(err)
	}
	want := cfg.DragModifiers{Option: true, Shift: true}
	if a.settingsOpen || a.st.Settings.DragModifiers != want {
		t.Fatalf("saved state=%+v, settingsOpen=%v", a.st.Settings, a.settingsOpen)
	}
	if got := cfg.LoadState(a.statePath, true); got.Settings == nil || got.Settings.DragModifiers != want {
		t.Fatalf("reloaded state=%+v", got)
	}
	if a.keys.bindings != cfg.DefaultSettings().Shortcuts {
		t.Fatalf("global shortcuts changed: %+v", a.keys.bindings)
	}
	a.dialog.Open()
	tester.Frame()
	if err := tester.Click("拖动修饰键 Option"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("取消"); err != nil {
		t.Fatal(err)
	}
	a.dialog.Open()
	if a.dialog.Draft.DragModifiers != want {
		t.Fatalf("cancel did not restore saved modifiers: %+v", a.dialog.Draft.DragModifiers)
	}
}

func TestDragSettingsDialogRejectsEmptyAndRollsBackOnSaveFailure(t *testing.T) {
	a := settingsTestApp(t)
	tester := ui.NewTester(a.dialog.View, 640, 900)
	if err := tester.Click("拖动修饰键 Command"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if a.dialog.Error() == "" || !a.settingsOpen || a.st.Settings.DragModifiers != (cfg.DragModifiers{Command: true}) {
		t.Fatalf("empty modifiers accepted or prior state changed: error=%q, settings=%+v", a.dialog.Error(), a.st.Settings)
	}
	if err := tester.Click("拖动修饰键 Control"); err != nil {
		t.Fatal(err)
	}
	a.statePath = filepath.Join(t.TempDir(), "missing", "state.json")
	if err := tester.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if a.dialog.Error() == "" || !a.settingsOpen || a.st.Settings.DragModifiers != (cfg.DragModifiers{Command: true}) || a.dialog.Draft.DragModifiers != (cfg.DragModifiers{Control: true}) {
		t.Fatalf("write failure changed state or draft: error=%q, state=%+v draft=%+v", a.dialog.Error(), a.st.Settings.DragModifiers, a.dialog.Draft.DragModifiers)
	}
}

func TestDragSettingsNarrowWindow(t *testing.T) {
	a := settingsTestApp(t)
	tester := ui.NewTester(a.dialog.View, 220, 1200)
	for _, label := range []string{"拖动修饰键 Command", "拖动修饰键 Control", "拖动修饰键 Option", "拖动修饰键 Shift"} {
		rect, found := tester.Find(label)
		if !found || rect.X < 0 || rect.X+rect.W > 220 {
			t.Fatalf("%s outside window: %+v, found=%v", label, rect, found)
		}
		if err := tester.Click(label); err != nil {
			t.Fatalf("%s must be clickable: %v", label, err)
		}
	}
	if got := a.dialog.Draft.DragModifiers; got != (cfg.DragModifiers{Control: true, Option: true, Shift: true}) {
		t.Fatalf("clicking all modifiers produced %+v", got)
	}
}
