package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDragModifiersValidationAndExactMatch(t *testing.T) {
	command := DragModifiers{Command: true}
	if got := DefaultSettings().DragModifiers; got != command {
		t.Fatalf("default drag modifiers = %+v, want %+v", got, command)
	}
	tests := []struct {
		name   string
		chosen DragModifiers
		held   DragModifiers
		match  bool
	}{
		{"command", command, command, true},
		{"single option", DragModifiers{Option: true}, DragModifiers{Option: true}, true},
		{"combination", DragModifiers{Command: true, Shift: true}, DragModifiers{Command: true, Shift: true}, true},
		{"extra modifier", command, DragModifiers{Command: true, Shift: true}, false},
		{"missing modifier", DragModifiers{Command: true, Shift: true}, command, false},
		{"empty does not match", DragModifiers{}, DragModifiers{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chosen.Matches(tt.held); got != tt.match {
				t.Fatalf("%+v.matches(%+v) = %v, want %v", tt.chosen, tt.held, got, tt.match)
			}
		})
	}
	invalid := DefaultSettings()
	invalid.DragModifiers = DragModifiers{}
	if ValidateSettings(invalid) == nil {
		t.Fatal("empty drag modifiers must be rejected")
	}
}

func TestDragSettingsDecodeKeepsOtherFields(t *testing.T) {
	want := DefaultSettings()
	want.MinOpacity, want.MaxOpacity = 20, 70
	want.Shortcuts[0] = ShortcutBinding{Key: "A", Control: true}
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
