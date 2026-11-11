package main

import (
	"context"
	"sync/atomic"
	"time"
)

// monitorTrayHost 在后台探测宿主；阻塞的探测至多保留一个，且仍报告未知状态。
func monitorTrayHost(ctx context.Context, ticks <-chan time.Time, probe func() trayHostStatus, evidence *atomic.Uint64, changed func(trayHostStatus, uint64)) {
	type observation struct {
		status   trayHostStatus
		evidence uint64
	}
	results := make(chan observation, 1)
	pending := false
	start := func() {
		pending = true
		started := evidence.Load()
		go func() { results <- observation{probe(), started} }()
	}
	start()
	for {
		select {
		case <-ctx.Done():
			return
		case result := <-results:
			pending = false
			if ctx.Err() == nil {
				changed(result.status, result.evidence)
			}
		case <-ticks:
			if ctx.Err() != nil {
				return
			}
			if pending {
				changed(trayHostUnknown, evidence.Load())
			} else {
				start()
			}
		}
	}
}
