package main

import (
	"fmt"

	"github.com/ebitengine/purego/objc"
)

const leftMouseDown = 1

var (
	windowDragAddMonitor    = objc.RegisterName("addLocalMonitorForEventsMatchingMask:handler:")
	windowDragRemoveMonitor = objc.RegisterName("removeMonitor:")
	windowDragEventType     = objc.RegisterName("type")
	windowDragEventWindow   = objc.RegisterName("window")
	windowDragEventFlags    = objc.RegisterName("modifierFlags")
	windowDragEventLocation = objc.RegisterName("locationInWindow")
	windowDragContentView   = objc.RegisterName("contentView")
	windowDragSuperview     = objc.RegisterName("superview")
	windowDragConvertPoint  = objc.RegisterName("convertPoint:fromView:")
	windowDragHitTest       = objc.RegisterName("hitTest:")
	windowDragDescendant    = objc.RegisterName("isDescendantOf:")
	windowDragPerform       = objc.RegisterName("performWindowDragWithEvent:")
)

// shouldStartWindowDrag 判断左键事件是否属于目标窗口并精确匹配拖动组合。
func shouldStartWindowDrag(eventWindow, targetWindow uintptr, eventType, flags uint64, modifiers dragModifiers, ignoring bool) bool {
	if ignoring || targetWindow == 0 || eventWindow != targetWindow || eventType != leftMouseDown {
		return false
	}
	held := dragModifiers{
		Command: flags&flagCommand != 0,
		Control: flags&flagControl != 0,
		Option:  flags&flagOption != 0,
		Shift:   flags&flagShift != 0,
	}
	return modifiers.matches(held)
}

// windowDragPoint 是 AppKit 的窗口坐标。
type windowDragPoint struct{ X, Y float64 }

// windowDragHitsContent 排除窗口边缘和原生标题栏按钮。
func windowDragHitsContent(window, event objc.ID) bool {
	return windowDragLocationHitsContent(window, objc.Send[windowDragPoint](event, windowDragEventLocation))
}

// windowDragLocationHitsContent 检查窗口坐标处的实际命中视图。
func windowDragLocationHitsContent(window objc.ID, location windowDragPoint) bool {
	content := window.Send(windowDragContentView)
	if content == 0 {
		return false
	}
	frame := content.Send(windowDragSuperview)
	if frame == 0 {
		return false
	}
	location = objc.Send[windowDragPoint](frame, windowDragConvertPoint, location, objc.ID(0))
	hit := objc.Send[objc.ID](frame, windowDragHitTest, location)
	return hit != 0 && (hit == content || hit.Send(windowDragDescendant, content) != 0)
}

// installWindowDrag 在主线程监听本窗口的左键按下，并将匹配的事件交给系统移动窗口。
func (a *app) installWindowDrag() error {
	windowDragEventClass := objc.ID(objc.GetClass("NSEvent"))
	if a.win == nil || windowDragEventClass == 0 {
		return fmt.Errorf("原生窗口不可用")
	}
	target := objc.ID(a.win.NativeHandle())
	if target == 0 {
		return fmt.Errorf("无法取得原生窗口")
	}
	block := objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
		if event == 0 {
			return event
		}
		modifiers := defaultSettings().DragModifiers
		if a.st.Settings != nil {
			modifiers = a.st.Settings.DragModifiers
		}
		eventWindow := event.Send(windowDragEventWindow)
		eventType := uint64(event.Send(windowDragEventType))
		flags := uint64(event.Send(windowDragEventFlags))
		if !shouldStartWindowDrag(uintptr(eventWindow), uintptr(target), eventType, flags, modifiers, a.ignoringMouse) || !windowDragHitsContent(target, event) {
			return event
		}
		target.Send(windowDragPerform, event)
		return 0
	})
	monitor := windowDragEventClass.Send(windowDragAddMonitor, uint64(1<<leftMouseDown), block)
	block.Release()
	if monitor == 0 {
		return fmt.Errorf("无法监听鼠标事件")
	}
	a.win.OnClosed(func() { windowDragEventClass.Send(windowDragRemoveMonitor, monitor) })
	return nil
}
