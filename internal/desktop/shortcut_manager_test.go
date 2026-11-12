package desktop

import (
	"errors"
	"testing"

	cfg "ghostfish/internal/config"
)

func fakeShortcutManager() (*shortcutManager, map[cfg.ShortcutBinding]bool, map[cfg.ShortcutBinding]bool) {
	active := map[cfg.ShortcutBinding]bool{}
	blocked := map[cfg.ShortcutBinding]bool{}
	m := &shortcutManager{}
	m.register = func(_ int, binding cfg.ShortcutBinding, _ func()) (func(), error) {
		if active[binding] || blocked[binding] {
			return nil, errors.New("快捷键不可用")
		}
		active[binding] = true
		return func() { delete(active, binding) }, nil
	}
	return m, active, blocked
}

func TestShortcutUpdateSwapAndFailure(t *testing.T) {
	m, active, blocked := fakeShortcutManager()
	old := cfg.DefaultSettings().Shortcuts
	if failures := m.start(old); len(failures) != 0 {
		t.Fatal(failures)
	}
	swap := old
	swap[0], swap[1] = swap[1], swap[0]
	if _, err := m.update(swap, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if m.bindings != swap || len(active) != 4 {
		t.Fatalf("swap not applied: %+v", m)
	}
	conflict := swap
	conflict[0] = cfg.ShortcutBinding{Key: "X", Command: true}
	blocked[conflict[0]] = true
	if _, err := m.update(conflict, func() error { return nil }); err == nil {
		t.Fatal("expected conflict")
	}
	if m.bindings != swap || len(active) != 4 || !m.registered[0] {
		t.Fatalf("old shortcuts not restored: %+v", m)
	}
}

func TestShortcutUpdateWriteAndRestoreFailure(t *testing.T) {
	m, active, blocked := fakeShortcutManager()
	old := cfg.DefaultSettings().Shortcuts
	if failures := m.start(old); len(failures) != 0 {
		t.Fatal(failures)
	}
	next := old
	next[cfg.ShortcutThrough] = cfg.ShortcutBinding{Key: "X", Command: true}
	_, err := m.update(next, func() error {
		blocked[old[cfg.ShortcutThrough]] = true
		return errors.New("write failed")
	})
	if err == nil || m.registered[cfg.ShortcutThrough] || active[next[cfg.ShortcutThrough]] || m.bindings != old {
		t.Fatalf("rollback = %v, manager = %+v", err, m)
	}
}

// 未修改但启动时注册失败的快捷键，不应该挡住只改范围或其他键位的保存。
func TestShortcutUpdateKeepsUnchangedUnavailable(t *testing.T) {
	m, active, blocked := fakeShortcutManager()
	old := cfg.DefaultSettings().Shortcuts
	blocked[old[cfg.ShortcutThrough]] = true
	if failures := m.start(old); len(failures) != 1 {
		t.Fatalf("failures = %v", failures)
	}
	next := old
	next[0] = cfg.ShortcutBinding{Key: "A", Control: true}
	notes, err := m.update(next, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || m.registered[cfg.ShortcutThrough] || !m.registered[cfg.ShortcutDown] || m.bindings != next {
		t.Fatalf("notes = %v, manager = %+v", notes, m)
	}
	if !active[next[0]] || active[old[cfg.ShortcutThrough]] {
		t.Fatalf("active = %v", active)
	}
}

func TestShortcutSuspendAndResume(t *testing.T) {
	m, active, blocked := fakeShortcutManager()
	bindings := cfg.DefaultSettings().Shortcuts
	if failures := m.start(bindings); len(failures) != 0 {
		t.Fatal(failures)
	}
	previous := m.suspend()
	if len(active) != 0 || !m.suppressed {
		t.Fatal("shortcuts were not suspended")
	}
	blocked[bindings[cfg.ShortcutThrough]] = true
	if failures := m.resume(previous); len(failures) != 1 {
		t.Fatalf("failures = %v", failures)
	}
	if m.registered[cfg.ShortcutThrough] || !m.registered[cfg.ShortcutHidden] || m.suppressed {
		t.Fatal("incorrect restoration")
	}
}
