package main

import (
	"context"
	"net/url"

	"github.com/egoist/mygo"
)

// browserTab 保存一个页面及地址栏的运行期状态。
type browserTab struct {
	id               uint64
	view             *mygo.WebView
	title            string
	url              string
	input            string
	errorText        string
	editing          bool
	focused          bool
	pending          bool
	generation       uint64
	cancelNavigation context.CancelFunc
	openedPage       bool
	loadingPage      bool
	startupLoading   bool
	loadingInput     string
	openingFirstPage bool
	authURL          string
	authSourceURL    string
	authLoading      bool
	authMainFrame    bool
	authPrepared     *preparedAuth
	authPrompt       *authPrompt
}

// showsAddress 只为尚未打开网页的新标签显示地址栏。
func (t *browserTab) showsAddress() bool {
	return !t.openedPage && !t.startupLoading
}

// startPageLoad 记录网页加载和首次加载失败后可恢复的网址。
func (t *browserTab) startPageLoad(raw string, startup bool) {
	t.loadingPage = true
	t.loadingInput = raw
	t.openingFirstPage = !t.openedPage
	t.startupLoading = startup && t.openingFirstPage
	if t.openingFirstPage {
		t.input = raw
		t.editing = true
		t.errorText = ""
	}
}

// commitPageLoad 在主文档提交时结束新标签的地址输入状态。
func (t *browserTab) commitPageLoad(raw string) bool {
	parsed, err := url.Parse(raw)
	if !t.loadingPage || err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return false
	}
	changed := t.showsAddress()
	t.startupLoading = false
	t.openedPage = true
	t.editing = false
	t.focused = false
	t.errorText = ""
	t.updateURL(raw)
	return changed
}

// finishPageLoad 在页面载入完成后结束失败恢复阶段。
func (t *browserTab) finishPageLoad(raw string) bool {
	if !t.loadingPage {
		return false
	}
	changed := t.commitPageLoad(raw)
	if t.openedPage {
		t.loadingPage = false
		t.openingFirstPage = false
	}
	return changed
}

// cancelPageLoad 停止记录被新输入取代的网页加载。
func (t *browserTab) cancelPageLoad() bool {
	if !t.loadingPage {
		return false
	}
	t.loadingPage = false
	t.startupLoading = false
	return true
}

// failPageLoad 只处理当前网页视图的失败，并在首次失败时恢复输入。
func (t *browserTab) failPageLoad(source *mygo.WebView, message string) bool {
	if source != t.view || !t.loadingPage {
		return false
	}
	t.loadingPage = false
	t.startupLoading = false
	if !t.openingFirstPage {
		return false
	}
	t.openedPage = false
	t.openingFirstPage = false
	t.input = t.loadingInput
	t.editing = true
	t.errorText = message
	return true
}

// beginNavigation 使同一标签页中更早的异步导航结果失效。
func (t *browserTab) beginNavigation() uint64 {
	if t.cancelNavigation != nil {
		t.cancelNavigation()
		t.cancelNavigation = nil
	}
	t.generation++
	return t.generation
}

// updateURL 同步实际网址，同时保留尚未提交的输入。
func (t *browserTab) updateURL(raw string) bool {
	if raw == "about:blank" {
		raw = ""
	}
	changed := t.url != raw
	previousInput := t.input
	t.url = raw
	if !t.editing && !t.pending && !t.loadingPage {
		t.input = raw
	}
	return changed || t.input != previousInput
}

// setFocused 更新地址栏焦点；新标签失焦时保留输入和错误。
func (t *browserTab) setFocused(focused bool) bool {
	blurred := t.focused && !focused
	if blurred && t.openedPage {
		t.editing = false
		t.errorText = ""
		if !t.pending {
			t.input = t.url
		}
	}
	t.focused = focused
	return blurred
}

// pageWillNavigate 使页面发起的新导航取代等待中的地址栏请求。
func (t *browserTab) pageWillNavigate(userInitiated bool) {
	t.beginNavigation()
	t.pending = false
	if t.openedPage {
		t.loadingPage = false
		t.openingFirstPage = false
	}
	if userInitiated {
		t.setFocused(false)
	}
}

// observeURL 同步网页变化，并取消被同文档导航取代的请求。
func (t *browserTab) observeURL(raw string) bool {
	if raw == "about:blank" {
		raw = ""
	}
	if t.pending && raw != t.url {
		t.beginNavigation()
		t.pending = false
	}
	return t.updateURL(raw)
}

// tabSet 管理标签顺序和当前标签；方法均由主线程调用。
type tabSet struct {
	items  []*browserTab
	active *browserTab
	nextID uint64
}

// add 添加一个标签并将其设为当前标签。
func (s *tabSet) add(view *mygo.WebView) *browserTab {
	s.nextID++
	tab := &browserTab{id: s.nextID, view: view, title: "新标签页"}
	s.items = append(s.items, tab)
	s.active = tab
	return tab
}

// selectTab 选中已有标签，并返回该标签是否存在。
func (s *tabSet) selectTab(tab *browserTab) bool {
	for _, item := range s.items {
		if item == tab {
			s.active = tab
			return true
		}
	}
	return false
}

// remove 删除已有标签，返回是否删除及是否删除了最后一个标签。
func (s *tabSet) remove(tab *browserTab) (bool, bool) {
	for i, item := range s.items {
		if item != tab {
			continue
		}
		tab.beginNavigation()
		s.items = append(s.items[:i], s.items[i+1:]...)
		if len(s.items) == 0 {
			s.active = nil
			return true, true
		}
		if s.active == tab {
			if i >= len(s.items) {
				i = len(s.items) - 1
			}
			s.active = s.items[i]
		}
		return true, false
	}
	return false, false
}

// accepts 判断异步结果是否仍属于一个存在的标签页及其当前导航。
func (s *tabSet) accepts(tab *browserTab, generation uint64) bool {
	for _, item := range s.items {
		if item == tab {
			return tab.generation == generation
		}
	}
	return false
}
