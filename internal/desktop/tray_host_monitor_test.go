package desktop

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	cfg "ghostfish/internal/config"
)

// 探测卡住不影响调用者，且连续轮询不会积累阻塞探测。
func TestMonitorBlockedProbeDoesNotBlockOrMultiply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ticks := make(chan time.Time)
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	status := make(chan trayHostStatus, 2)
	done := make(chan struct{})
	var evidence atomic.Uint64
	go func() {
		monitorTrayHost(ctx, ticks, func() trayHostStatus {
			started <- struct{}{}
			<-release
			return trayHostAvailable
		}, &evidence, func(got trayHostStatus, _ uint64) { status <- got })
		close(done)
	}()
	<-started
	ticks <- time.Time{}
	if got := <-status; got != trayHostUnknown {
		t.Fatalf("阻塞探测状态 = %v", got)
	}
	ticks <- time.Time{}
	if got := <-status; got != trayHostUnknown {
		t.Fatalf("持续阻塞状态 = %v", got)
	}
	select {
	case <-started:
		t.Fatal("同一时间不应启动多个阻塞探测")
	default:
	}
	cancel()
	<-done
	close(release)
	select {
	case <-status:
		t.Fatal("停止后不应再通知界面")
	default:
	}
}

// 首次探测期间用户改变了配置，晚到的结果只能使用当前状态。
func TestPendingTrayProbeUsesCurrentState(t *testing.T) {
	a := &Controller{
		st:          cfg.State{Opacity: .85, ClickThrough: true},
		keys:        &shortcutManager{registered: [4]bool{true, true, true, true}},
		trayPending: true, statePath: filepath.Join(t.TempDir(), "state.json"),
	}
	a.showWindow()
	a.toggleClickThrough()
	a.setTrayHostStatus(trayHostAvailable)
	if a.trayPending || !a.trayAvailable || a.st.ClickThrough || a.ignoringMouse {
		t.Fatal("晚到的探测结果不应恢复旧穿透状态")
	}
}

// 菜单点击后的证据优先于旧探测，新探测仍能报告宿主消失。
func TestTrayMenuActionSupersedesInFlightProbe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time)
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	type observation struct {
		status   trayHostStatus
		evidence uint64
	}
	observations := make(chan observation, 2)
	probeCount := 0
	a := &Controller{st: cfg.State{Opacity: .85}, keys: &shortcutManager{}, trayPending: true,
		statePath: filepath.Join(t.TempDir(), "state.json"), trayWarning: func(string) {}}
	a.showWindow()
	go monitorTrayHost(ctx, ticks, func() trayHostStatus {
		probeCount++
		started <- struct{}{}
		if probeCount == 1 {
			<-release
		}
		return trayHostUnavailable
	}, &a.trayEvidence, func(status trayHostStatus, evidence uint64) {
		observations <- observation{status, evidence}
	})
	<-started
	a.recordTrayAction()
	a.toggleClickThrough()
	close(release)
	old := <-observations
	a.setTrayHostObservation(old.status, old.evidence)
	if !a.trayAvailable || !a.st.ClickThrough || !a.ignoringMouse {
		t.Fatal("旧探测不能撤销用户刚才的菜单操作")
	}
	ticks <- time.Time{}
	<-started
	current := <-observations
	a.setTrayHostObservation(current.status, current.evidence)
	if a.trayAvailable || a.st.ClickThrough || a.ignoringMouse {
		t.Fatal("新探测确认宿主消失后应保护窗口")
	}
}
