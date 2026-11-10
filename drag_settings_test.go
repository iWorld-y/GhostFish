package main

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDragModifiersValidationAndExactMatch(t *testing.T) {
	command := dragModifiers{Command: true}
	if got := defaultSettings().DragModifiers; got != command {
		t.Fatalf("default drag modifiers = %+v, want %+v", got, command)
	}
	tests := []struct {
		name   string
		chosen dragModifiers
		held   dragModifiers
		match  bool
	}{
		{"command", command, command, true},
		{"single option", dragModifiers{Option: true}, dragModifiers{Option: true}, true},
		{"combination", dragModifiers{Command: true, Shift: true}, dragModifiers{Command: true, Shift: true}, true},
		{"extra modifier", command, dragModifiers{Command: true, Shift: true}, false},
		{"missing modifier", dragModifiers{Command: true, Shift: true}, command, false},
		{"empty does not match", dragModifiers{}, dragModifiers{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chosen.matches(tt.held); got != tt.match {
				t.Fatalf("%+v.matches(%+v) = %v, want %v", tt.chosen, tt.held, got, tt.match)
			}
		})
	}
	invalid := defaultSettings()
	invalid.DragModifiers = dragModifiers{}
	if validateSettings(invalid) == nil {
		t.Fatal("empty drag modifiers must be rejected")
	}
}

func TestDragSettingsDecodeKeepsOtherFields(t *testing.T) {
	want := defaultSettings()
	want.MinOpacity, want.MaxOpacity = 20, 70
	want.Shortcuts[0] = shortcutBinding{Key: "A", Control: true}
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name    string
		value   string
		missing bool
		invalid bool
	}{
		{"legacy", "", true, false},
		{"null", "null", false, true},
		{"empty", `{}`, false, true},
		{"invalid type", `{"command":"yes"}`, false, true},
		{"null field", `{"command":true,"control":null}`, false, true},
		{"unsupported key", `{"command":true,"fn":true}`, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			copyFields := make(map[string]json.RawMessage, len(fields))
			for key, value := range fields {
				copyFields[key] = value
			}
			if tt.missing {
				delete(copyFields, "drag_modifiers")
			} else {
				copyFields["drag_modifiers"] = json.RawMessage(tt.value)
			}
			data, err := json.Marshal(copyFields)
			if err != nil {
				t.Fatal(err)
			}
			got, invalid := decodeSettings(data)
			if invalid != tt.invalid || !reflect.DeepEqual(got, want) {
				t.Fatalf("decode = %+v, invalid=%v; want %+v, invalid=%v", got, invalid, want, tt.invalid)
			}
		})
	}
}

func TestDragSettingsDialogSaveAndCancel(t *testing.T) {
	setSettingsSupported(t, true)
	a := settingsTestApp(t)
	tester := ui.NewTester(a.settingsDialog, 640, 900)
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
	want := dragModifiers{Option: true, Shift: true}
	if a.settingsOpen || a.st.Settings.DragModifiers != want {
		t.Fatalf("saved state=%+v, settingsOpen=%v", a.st.Settings, a.settingsOpen)
	}
	if got := loadState(a.statePath); got.Settings == nil || got.Settings.DragModifiers != want {
		t.Fatalf("reloaded state=%+v", got)
	}
	if a.keys.bindings != defaultSettings().Shortcuts {
		t.Fatalf("global shortcuts changed: %+v", a.keys.bindings)
	}
	a.openSettings()
	tester.Frame()
	if err := tester.Click("拖动修饰键 Option"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("取消"); err != nil {
		t.Fatal(err)
	}
	a.openSettings()
	if a.settingsDraft.DragModifiers != want {
		t.Fatalf("cancel did not restore saved modifiers: %+v", a.settingsDraft.DragModifiers)
	}
}

func TestDragSettingsDialogRejectsEmptyAndRollsBackOnSaveFailure(t *testing.T) {
	a := settingsTestApp(t)
	tester := ui.NewTester(a.settingsDialog, 640, 900)
	if err := tester.Click("拖动修饰键 Command"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if a.settingsError == "" || !a.settingsOpen || a.st.Settings.DragModifiers != (dragModifiers{Command: true}) {
		t.Fatalf("empty modifiers accepted or prior state changed: error=%q, settings=%+v", a.settingsError, a.st.Settings)
	}
	if err := tester.Click("拖动修饰键 Control"); err != nil {
		t.Fatal(err)
	}
	a.statePath = filepath.Join(t.TempDir(), "missing", "state.json")
	if err := tester.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if a.settingsError == "" || !a.settingsOpen || a.st.Settings.DragModifiers != (dragModifiers{Command: true}) || a.settingsDraft.DragModifiers != (dragModifiers{Control: true}) {
		t.Fatalf("write failure changed state or draft: error=%q, state=%+v draft=%+v", a.settingsError, a.st.Settings.DragModifiers, a.settingsDraft.DragModifiers)
	}
}

func TestDragSettingsNarrowWindow(t *testing.T) {
	a := settingsTestApp(t)
	tester := ui.NewTester(a.settingsDialog, 220, 1200)
	for _, label := range []string{"拖动修饰键 Command", "拖动修饰键 Control", "拖动修饰键 Option", "拖动修饰键 Shift"} {
		rect, found := tester.Find(label)
		if !found || rect.X < 0 || rect.X+rect.W > 220 {
			t.Fatalf("%s outside window: %+v, found=%v", label, rect, found)
		}
		if err := tester.Click(label); err != nil {
			t.Fatalf("%s must be clickable: %v", label, err)
		}
	}
	if got := a.settingsDraft.DragModifiers; got != (dragModifiers{Control: true, Option: true, Shift: true}) {
		t.Fatalf("clicking all modifiers produced %+v", got)
	}
}
