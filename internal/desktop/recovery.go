package desktop

import (
	"fmt"
	"os"
	"strings"

	cfg "ghostfish/internal/config"
)

// recoverableNote 说明强制可操作状态的原因，和具体哪个键失败一起显示。
const recoverableNote = "窗口已恢复可见、不穿透，请重新保存可用的组合"

// noteRestoreFailure 把一条恢复失败说明追加到界面与终端共用的提示里，
// 相同说明不重复追加；旧提示保留，避免后来的失败盖掉先前的原因。
func (a *Controller) noteRestoreFailure(note string) {
	if note == "" || strings.Contains(a.restoreError, note) {
		return
	}
	if a.restoreError == "" {
		a.restoreError = note
	} else {
		a.restoreError += "；" + note
	}
	fmt.Fprintln(os.Stderr, "ghostfish:", note)
}

// noteRecoveryLost 在恢复后真的弄丢了穿透或隐藏入口时标记强制状态。
// 只有不透明度之类的快捷键没恢复不算，窗口仍然完全可恢复。
// 这里同时把原因留下：恢复失败可能发生在保存回滚里，或者之前已有别的提示，
// 用户取消设置再打开时要能看到当前丢的是哪个恢复入口。
func (a *Controller) noteRecoveryLost(registeredBefore [4]bool) {
	if a.keys == nil {
		return
	}
	var lost []string
	for _, i := range []int{cfg.ShortcutThrough, cfg.ShortcutHidden} {
		if !registeredBefore[i] || a.keys.registered[i] {
			continue
		}
		a.recoveryLost = true
		if strings.Contains(a.restoreError, cfg.ShortcutLabels[i]) {
			continue
		}
		lost = append(lost, fmt.Sprintf("%s（%s）", cfg.ShortcutLabels[i], a.keys.bindings[i].Display()))
	}
	if !a.recoveryLost {
		return
	}
	if len(lost) > 0 {
		a.noteRestoreFailure("恢复快捷键失败：" + strings.Join(lost, "；"))
	}
	a.noteRestoreFailure(recoverableNote)
}

// ensureRecoverable 只在录入或保存回滚真的没能恢复旧快捷键时，
// 把窗口拉回可见、不穿透。用户仍可通过可用的入口主动更改状态。
func (a *Controller) ensureRecoverable() {
	if !a.restoreFailed() || (!a.settingsOpen && a.trayAvailable) {
		return
	}
	if a.st.ClickThrough {
		a.st.ClickThrough = false
		a.save()
	}
	a.applyClickThrough()
	if !a.windowShown() {
		a.showWindow()
	}
}

// restoreFailed 报告穿透或隐藏入口是否在录入或保存回滚中真的丢了。
func (a *Controller) restoreFailed() bool {
	return a.recoveryLost
}

// canClickThrough 判断托盘或穿透快捷键是否提供恢复入口。
func (a *Controller) canClickThrough() bool {
	if a.trayAvailable {
		return true
	}
	return a.shortcutAvailable(cfg.ShortcutThrough)
}

// applyClickThrough 把生效的穿透状态同步到窗口，设置打开时保持窗口可操作。
func (a *Controller) applyClickThrough() {
	a.setIgnoreMouse(a.st.ClickThrough && !a.settingsOpen && a.canClickThrough())
	a.syncTrayMenu()
}

// showStartupWindow 在托盘失败或尚未确认时按已注册的恢复入口显示窗口。
func (a *Controller) showStartupWindow(registered [4]bool) {
	if !a.trayPending {
		started := a.st.ClickThrough
		var notes []string
		a.st, notes = guardRecovery(registered, a.st, a.trayAvailable)
		for _, note := range notes {
			fmt.Fprintf(os.Stderr, "ghostfish: %s\n", note)
		}
		if started != a.st.ClickThrough {
			a.save()
		}
	}
	a.applyClickThrough()
	a.showWindow()
}

// guardRecovery 按各动作自己的注册结果保护窗口：穿透快捷键不可用时不能带着穿透启动，
// 隐藏快捷键不可用时窗口必须留在可见状态，其他快捷键是否成功不能代替这两项检查。
// 返回调整后的状态和需要写到终端的说明。
func guardRecovery(registered [4]bool, st cfg.State, trayAvailable bool) (cfg.State, []string) {
	if trayAvailable {
		return st, nil
	}
	var notes []string
	if st.ClickThrough && !registered[cfg.ShortcutThrough] {
		st.ClickThrough = false
		notes = append(notes, "穿透快捷键不可用，已以不穿透状态启动")
	}
	switch {
	case !registered[cfg.ShortcutThrough] && !registered[cfg.ShortcutHidden]:
		notes = append(notes, "警告：切换穿透和隐藏的快捷键都不可用，窗口保持可见、不穿透")
	case !registered[cfg.ShortcutHidden]:
		notes = append(notes, "隐藏快捷键不可用，窗口保持可见")
	}
	return st, notes
}

// setTrayAvailable 在托盘宿主丢失时按各动作的恢复入口保护窗口。
func (a *Controller) setTrayAvailable(available bool) {
	wasPending := a.trayPending
	a.trayPending = false
	if available {
		a.trayUnknowns = 0
	}
	if a.trayAvailable == available && !wasPending {
		return
	}
	a.trayAvailable = available
	if !available {
		if a.st.ClickThrough && !a.shortcutAvailable(cfg.ShortcutThrough) {
			a.st.ClickThrough = false
			a.applyClickThrough()
			a.save()
		}
		if !a.windowShown() && !a.shortcutAvailable(cfg.ShortcutHidden) {
			a.showWindow()
		}
		a.warnTrayUnavailable("托盘宿主或图标不可用")
	} else if wasPending {
		a.applyClickThrough()
	}
	a.syncTrayMenu()
}
