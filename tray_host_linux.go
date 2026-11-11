//go:build linux

package main

import (
	"context"
	"os"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	kdeWatcher  = "org.kde.StatusNotifierWatcher"
	freeWatcher = "org.freedesktop.StatusNotifierWatcher"
	watcherPath = dbus.ObjectPath("/StatusNotifierWatcher")
)

// watchTrayHost 在后台定期检查宿主，避免探测卡住影响窗口启动。
func watchTrayHost(evidence *atomic.Uint64, changed func(trayHostStatus, uint64)) (trayHostStatus, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		monitorTrayHost(ctx, ticker.C, trayHostPresent, evidence, changed)
	}()
	return trayHostUnknown, cancel
}

// trayHostPresent 只连接已存在的会话总线，不自行启动 D-Bus。
func trayHostPresent() trayHostStatus {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := dbus.SessionBusPrivateNoAutoStartup(dbus.WithContext(ctx))
	if err != nil {
		return trayHostUnknown
	}
	defer conn.Close()
	if err := conn.Auth(nil); err != nil {
		return trayHostUnknown
	}
	if err := conn.Hello(); err != nil {
		return trayHostUnknown
	}
	kde := readWatcher(ctx, conn, kdeWatcher)
	var free watcherInfo
	if kde.nameKnown && kde.owner != "" && !kde.known {
		free = readWatcher(ctx, conn, freeWatcher)
	}
	status := notifierHostStatus(kde, free)
	if status != trayHostAvailable {
		return status
	}
	items, ok := readRegisteredItems(ctx, conn, kdeWatcher)
	if !ok {
		if !free.nameKnown {
			free = readWatcher(ctx, conn, freeWatcher)
		}
		if free.owner != kde.owner {
			return trayHostUnknown
		}
		items, ok = readRegisteredItems(ctx, conn, freeWatcher)
	}
	if !ok {
		return trayHostUnknown
	}
	return notifierItemStatus(items, uint32(os.Getpid()), func(name string) (uint32, error) {
		var pid uint32
		call := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetConnectionUnixProcessID", 0, name)
		err := call.Store(&pid)
		return pid, err
	})
}

// readRegisteredItems 读取宿主可见的托盘项列表。
func readRegisteredItems(ctx context.Context, conn *dbus.Conn, name string) ([]string, bool) {
	var value dbus.Variant
	call := conn.Object(name, watcherPath).CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, name, "RegisteredStatusNotifierItems")
	if call.Err != nil || call.Store(&value) != nil {
		return nil, false
	}
	items, ok := value.Value().([]string)
	return items, ok
}

// readWatcher 查询指定服务的所有者和宿主属性。
func readWatcher(ctx context.Context, conn *dbus.Conn, name string) watcherInfo {
	var result watcherInfo
	call := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetNameOwner", 0, name)
	if call.Err != nil {
		switch err := call.Err.(type) {
		case dbus.Error:
			result.nameKnown = err.Name == "org.freedesktop.DBus.Error.NameHasNoOwner"
		case *dbus.Error:
			result.nameKnown = err.Name == "org.freedesktop.DBus.Error.NameHasNoOwner"
		}
		return result
	}
	if call.Store(&result.owner) != nil {
		return result
	}
	result.nameKnown = true
	var value dbus.Variant
	call = conn.Object(name, watcherPath).CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, name, "IsStatusNotifierHostRegistered")
	if call.Err != nil || call.Store(&value) != nil {
		return result
	}
	result.host, result.known = value.Value().(bool)
	return result
}
