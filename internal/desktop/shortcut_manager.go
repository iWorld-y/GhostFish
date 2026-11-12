package desktop

import (
	"fmt"
	"strings"

	cfg "ghostfish/internal/config"
)

// shortcutManager 跟踪每个动作的注册结果，并在更新失败时恢复旧键位。
type shortcutManager struct {
	bindings   [4]cfg.ShortcutBinding
	registered [4]bool
	stops      [4]func()
	actions    [4]func()
	register   func(int, cfg.ShortcutBinding, func()) (func(), error)
	suppressed bool
}

func (m *shortcutManager) bind(i int, binding cfg.ShortcutBinding) error {
	stop, err := m.register(i, binding, func() {
		if !m.suppressed && m.actions[i] != nil {
			m.actions[i]()
		}
	})
	if err != nil {
		return err
	}
	m.stops[i], m.registered[i] = stop, true
	return nil
}

func (m *shortcutManager) unbind(i int) {
	if m.registered[i] {
		m.stops[i]()
		m.stops[i] = nil
		m.registered[i] = false
	}
}

// start 逐项注册快捷键，返回无法注册的动作及原因。
func (m *shortcutManager) start(bindings [4]cfg.ShortcutBinding) []string {
	m.bindings = bindings
	var failures []string
	for i, binding := range bindings {
		if err := m.bind(i, binding); err != nil {
			failures = append(failures, fmt.Sprintf("%s（%s）：%v", cfg.ShortcutLabels[i], binding.Display(), err))
		}
	}
	return failures
}

// suspend 录入期间暂时释放已有组合，避免系统拦截当前快捷键。
func (m *shortcutManager) suspend() [4]bool {
	previous := m.registered
	m.suppressed = true
	for i := range previous {
		m.unbind(i)
	}
	return previous
}

// resume 只恢复此前注册成功的组合，返回实际无法恢复的动作。
func (m *shortcutManager) resume(previous [4]bool) []string {
	var failures []string
	for i, wasRegistered := range previous {
		if wasRegistered {
			if err := m.bind(i, m.bindings[i]); err != nil {
				failures = append(failures, fmt.Sprintf("%s（%s）：%v", cfg.ShortcutLabels[i], m.bindings[i].Display(), err))
			}
		}
	}
	m.suppressed = false
	return failures
}

// update 在全部变更键位注册成功且持久化成功后提交配置；失败时尽量恢复旧注册。
// 未修改但当前仍不可用的键位会再试一次注册，成功就恢复使用，失败不妨碍保存，
// 只作为说明返回，因为它们的注册状态本来就没变。
func (m *shortcutManager) update(bindings [4]cfg.ShortcutBinding, persist func() error) ([]string, error) {
	old := m.bindings
	wasRegistered := m.registered
	var changed, added [4]bool
	m.suppressed = true
	defer func() { m.suppressed = false }()
	for i := range bindings {
		changed[i] = old[i] != bindings[i]
		if changed[i] {
			m.unbind(i)
		}
	}
	var failure error
	var notes []string
	for i, binding := range bindings {
		if m.registered[i] {
			continue
		}
		if err := m.bind(i, binding); err != nil {
			if changed[i] {
				failure = fmt.Errorf("%s（%s）注册失败：%w", cfg.ShortcutLabels[i], binding.Display(), err)
				break
			}
			notes = append(notes, fmt.Sprintf("%s（%s）", cfg.ShortcutLabels[i], binding.Display()))
			continue
		}
		added[i] = true
	}
	if failure == nil {
		failure = persist()
	}
	if failure == nil {
		m.bindings = bindings
		return notes, nil
	}
	for i := range added {
		if added[i] {
			m.unbind(i)
		}
	}
	var failedRestore []string
	for i := range old {
		if wasRegistered[i] && !m.registered[i] {
			if err := m.bind(i, old[i]); err != nil {
				failedRestore = append(failedRestore, fmt.Sprintf("%s（%s）：%v", cfg.ShortcutLabels[i], old[i].Display(), err))
			}
		}
	}
	if len(failedRestore) > 0 {
		return nil, fmt.Errorf("%v；恢复原快捷键失败：%s", failure, strings.Join(failedRestore, "；"))
	}
	return nil, failure
}
