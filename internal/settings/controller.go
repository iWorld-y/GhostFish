// Package settings 管理设置草稿、快捷键录入和设置界面。
package settings

import (
	"fmt"
	"strconv"
	"strings"

	cfg "ghostfish/internal/config"
)

// Host 提供设置界面需要的桌面操作；窗口状态由桌面组件持有。
type Host interface {
	CurrentSettings() cfg.Settings
	SetSettingsOpen(bool)
	SuspendShortcuts() [4]bool
	ResumeShortcuts([4]bool)
	ShortcutAvailable(int) bool
	RestoreError() string
	ApplySettings(cfg.Settings) error
}

// Controller 持有可丢弃的编辑草稿；Draft、Min、Max 是界面输入，保存成功后才生效。
type Controller struct {
	host                   Host
	Draft                  cfg.Settings
	Min, Max               string
	open, modalOpen, focus bool
	errorText              string
	recording              int
	recordingFocus         bool
	suspendedKeys          [4]bool
}

// New 连接桌面操作，不打开设置界面。
func New(host Host) *Controller { return &Controller{host: host, recording: -1} }

// Open 从生效配置重新建立草稿。
func (a *Controller) Open() {
	a.Draft = a.host.CurrentSettings()
	a.Min, a.Max = strconv.Itoa(a.Draft.MinOpacity), strconv.Itoa(a.Draft.MaxOpacity)
	a.errorText = ""
	a.focus = true
	a.recording = -1
	a.open, a.modalOpen = true, true
	a.host.SetSettingsOpen(true)
}

// BeginRecording 释放原快捷键并开始录入指定动作。
func (a *Controller) BeginRecording(index int) {
	if a.recording >= 0 {
		a.EndRecording()
	}
	a.errorText = ""
	a.recording, a.recordingFocus = index, true
	a.suspendedKeys = a.host.SuspendShortcuts()
}

// EndRecording 结束录入并恢复原快捷键。
func (a *Controller) EndRecording() {
	if a.recording < 0 {
		return
	}
	a.recording = -1
	a.host.ResumeShortcuts(a.suspendedKeys)
}

// Close 丢弃草稿，并恢复窗口操作状态。
func (a *Controller) Close() {
	if a.recording >= 0 {
		a.EndRecording()
	}
	a.open, a.modalOpen = false, false
	a.host.SetSettingsOpen(false)
}

// Save 校验草稿并交给桌面组件应用，失败时保留编辑内容。
func (a *Controller) Save() error {
	if a.recording >= 0 {
		return fmt.Errorf("请先完成或取消快捷键录入")
	}
	low, lowErr := strconv.Atoi(strings.TrimSpace(a.Min))
	high, highErr := strconv.Atoi(strings.TrimSpace(a.Max))
	if lowErr != nil || highErr != nil {
		return fmt.Errorf("请输入 0%% 至 100%% 的整数，且下限不能大于上限")
	}
	draft := a.Draft
	draft.MinOpacity, draft.MaxOpacity = low, high
	if err := a.host.ApplySettings(draft); err != nil {
		return err
	}
	a.Close()
	return nil
}

// Recording 返回正在录入的动作，-1 表示没有录入。
func (a *Controller) Recording() int { return a.recording }

// Error 返回最近一次界面操作错误。
func (a *Controller) Error() string { return a.errorText }
