package desktop

import "strings"

// trayHostStatus 区分可用、明确不可用与无法查询的托盘宿主。
type trayHostStatus uint8

const (
	trayHostUnknown trayHostStatus = iota
	trayHostUnavailable
	trayHostAvailable
)

// watcherInfo 记录名称所有者及宿主属性是否查询成功。
type watcherInfo struct {
	owner     string
	nameKnown bool
	host      bool
	known     bool
}

// notifierHostStatus 优先检查 Ayatana 实际使用的 KDE 接口。
// 只有两个名称由同一服务拥有时，才用 freedesktop 别名补充缺失属性。
func notifierHostStatus(kde, freedesktop watcherInfo) trayHostStatus {
	if !kde.nameKnown {
		return trayHostUnknown
	}
	if kde.owner == "" {
		return trayHostUnavailable
	}
	if kde.known {
		if kde.host {
			return trayHostAvailable
		}
		return trayHostUnavailable
	}
	if freedesktop.nameKnown && freedesktop.owner == kde.owner && freedesktop.known {
		if freedesktop.host {
			return trayHostAvailable
		}
		return trayHostUnavailable
	}
	return trayHostUnknown
}

// notifierItemStatus 确认托盘项属于本进程；无法确认时不视为可用。
func notifierItemStatus(items []string, pid uint32, processID func(string) (uint32, error)) trayHostStatus {
	for _, item := range items {
		name, _, _ := strings.Cut(item, "/")
		if name == "" {
			continue
		}
		ownerPID, err := processID(name)
		if err == nil && ownerPID == pid {
			return trayHostAvailable
		}
	}
	return trayHostUnknown
}
