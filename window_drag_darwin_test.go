package main

import "testing"

func TestShouldStartWindowDrag(t *testing.T) {
	const target uintptr = 123
	command := dragModifiers{Command: true}
	tests := []struct {
		name       string
		window     uintptr
		target     uintptr
		eventType  uint64
		flags      uint64
		modifiers  dragModifiers
		ignore     bool
		shouldDrag bool
	}{
		{"matching command", target, target, 1, flagCommand, command, false, true},
		{"matching combination", target, target, 1, flagCommand | flagShift, dragModifiers{Command: true, Shift: true}, false, true},
		{"extra modifier", target, target, 1, flagCommand | flagShift, command, false, false},
		{"missing modifier", target, target, 1, flagCommand, dragModifiers{Command: true, Shift: true}, false, false},
		{"caps lock ignored", target, target, 1, flagCommand | 1<<16, command, false, true},
		{"other window", 456, target, 1, flagCommand, command, false, false},
		{"invalid target", 0, 0, 1, flagCommand, command, false, false},
		{"right mouse", target, target, 3, flagCommand, command, false, false},
		{"middle mouse", target, target, 25, flagCommand, command, false, false},
		{"ignored mouse", target, target, 1, flagCommand, command, true, false},
		{"empty configuration", target, target, 1, 0, dragModifiers{}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldStartWindowDrag(tt.window, tt.target, tt.eventType, tt.flags, tt.modifiers, tt.ignore); got != tt.shouldDrag {
				t.Fatalf("shouldStartWindowDrag = %v, want %v", got, tt.shouldDrag)
			}
		})
	}
}
