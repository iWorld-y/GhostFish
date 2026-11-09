package main

import (
	"context"
	"testing"

	"github.com/egoist/mygo"
)

func TestTabSetSelectionAndClosing(t *testing.T) {
	var tabs tabSet
	first := tabs.add(nil)
	second := tabs.add(nil)
	third := tabs.add(nil)
	if tabs.active != third || len(tabs.items) != 3 {
		t.Fatal("new tab should become active")
	}
	if !tabs.selectTab(first) || tabs.active != first {
		t.Fatal("selecting an existing tab should activate it")
	}
	if removed, last := tabs.remove(second); !removed || last || tabs.active != first {
		t.Fatal("closing an inactive tab should keep the active tab")
	}
	if removed, last := tabs.remove(first); !removed || last || tabs.active != third {
		t.Fatal("closing the active tab should select a surviving tab")
	}
	if removed, last := tabs.remove(third); !removed || !last || tabs.active != nil {
		t.Fatal("closing the last tab should leave no active tab")
	}
}

func TestTabNavigationGeneration(t *testing.T) {
	var tabs tabSet
	one := tabs.add(nil)
	two := tabs.add(nil)
	old := one.beginNavigation()
	current := one.beginNavigation()
	other := two.beginNavigation()
	if tabs.accepts(one, old) || !tabs.accepts(one, current) || !tabs.accepts(two, other) {
		t.Fatal("navigation results must belong to the current generation of their own tab")
	}
	tabs.remove(one)
	if tabs.accepts(one, current) || !tabs.accepts(two, other) {
		t.Fatal("a closed tab must reject pending results")
	}
}

func TestTabURLKeepsUnsubmittedInput(t *testing.T) {
	tab := &browserTab{}
	tab.updateURL("https://example.com/start")
	if tab.input != "https://example.com/start" {
		t.Fatalf("input = %q", tab.input)
	}
	tab.input = "unfinished"
	tab.editing = true
	tab.updateURL("https://example.com/redirect")
	if tab.input != "unfinished" || tab.url != "https://example.com/redirect" {
		t.Fatalf("input = %q, url = %q", tab.input, tab.url)
	}
	tab.editing = false
	tab.updateURL("about:blank")
	if tab.input != "" || tab.url != "" {
		t.Fatalf("blank page should have an empty address: %+v", tab)
	}
}

func TestAddressBlurRestoresCurrentURL(t *testing.T) {
	tab := &browserTab{url: "https://example.com/start", input: "unfinished", editing: true, focused: true, openedPage: true}
	tab.setFocused(false)
	tab.updateURL("https://example.com/next")
	if tab.editing || tab.input != "https://example.com/next" {
		t.Fatalf("blurred address must follow the page: %+v", tab)
	}
}

func TestCloseBackgroundTabPreservesActiveDraft(t *testing.T) {
	var tabs tabSet
	background := tabs.add(nil)
	active := tabs.add(nil)
	active.input, active.editing = "unfinished", true
	tabs.remove(background)
	if tabs.active != active || !active.editing || active.input != "unfinished" {
		t.Fatal("closing a background tab must not reset the active draft")
	}
}

func TestPageNavigationSupersedesPendingAddress(t *testing.T) {
	var tabs tabSet
	tab := tabs.add(nil)
	tab.url = "https://example.com/start"
	generation := tab.beginNavigation()
	tab.pending = true
	tab.pageWillNavigate(true)
	if tabs.accepts(tab, generation) || tab.pending {
		t.Fatal("a later page navigation must supersede pending address authentication")
	}
	generation = tab.beginNavigation()
	tab.pending = true
	tab.observeURL("https://example.com/start#next")
	if tabs.accepts(tab, generation) || tab.pending {
		t.Fatal("a later same-document URL change must supersede pending authentication")
	}
}

func TestRejectedAddressSupersedesPendingNavigation(t *testing.T) {
	for _, input := range []string{"", "mailto:user@example.com"} {
		t.Run(input, func(t *testing.T) {
			var a app
			tab := a.tabs.add(nil)
			tab.url = "https://example.com/current"
			tab.input = input
			generation := tab.beginNavigation()
			tab.pending = true
			a.submitAddress(tab)
			if a.tabs.accepts(tab, generation) || tab.pending || tab.url != "https://example.com/current" {
				t.Fatalf("rejected input must leave the current page and cancel older navigation: %+v", tab)
			}
		})
	}
}

func TestTabNavigationCancelsEarlierWork(t *testing.T) {
	var tabs tabSet
	tab := tabs.add(nil)
	ctx, cancel := context.WithCancel(context.Background())
	tab.cancelNavigation = cancel
	tab.beginNavigation()
	if ctx.Err() == nil {
		t.Fatal("new navigation should cancel earlier work")
	}
	ctx, cancel = context.WithCancel(context.Background())
	tab.cancelNavigation = cancel
	tabs.remove(tab)
	if ctx.Err() == nil {
		t.Fatal("closing a tab should cancel its work")
	}
}

func TestAddressVisibilityFollowsTabLifecycle(t *testing.T) {
	var tabs tabSet
	first := tabs.add(nil)
	if !first.showsAddress() {
		t.Fatal("a new empty tab must show the address bar")
	}
	first.startPageLoad("https://example.com", false)
	if !first.showsAddress() {
		t.Fatal("an empty tab must keep the address bar while loading")
	}
	first.finishPageLoad("https://example.com")
	if first.showsAddress() {
		t.Fatal("a tab that has opened a page must hide the address bar")
	}
	first.updateURL("about:blank")
	if first.showsAddress() {
		t.Fatal("an opened tab must not become new again when its URL is blank")
	}
	second := tabs.add(nil)
	if !second.showsAddress() {
		t.Fatal("another new tab must have its own address bar")
	}
	tabs.selectTab(first)
	if tabs.active.showsAddress() {
		t.Fatal("switching back to an opened tab must keep the bar hidden")
	}
	tabs.selectTab(second)
	if !tabs.active.showsAddress() {
		t.Fatal("switching back to a new tab must show the bar")
	}
}

func TestStartupURLAndFailedFirstLoad(t *testing.T) {
	var tabs tabSet
	tab := tabs.add(nil)
	tab.startPageLoad("https://example.com", true)
	if tab.showsAddress() {
		t.Fatal("a tab started with a URL should initially hide the address bar")
	}
	tab.failPageLoad(tab.view, "网络错误")
	if !tab.showsAddress() || tab.input != "https://example.com" || tab.errorText == "" {
		t.Fatalf("a failed first load should allow editing and retrying: %+v", tab)
	}
	tab.startPageLoad("https://example.com", false)
	tab.finishPageLoad("https://example.com")
	if tab.showsAddress() || tab.errorText != "" {
		t.Fatalf("a successful retry should hide the bar and clear the error: %+v", tab)
	}
}

func TestInvalidAddressRemainsEditableOnBlur(t *testing.T) {
	var a app
	tab := a.tabs.add(nil)
	tab.input = "mailto:user@example.com"
	tab.editing = true
	tab.focused = true
	a.submitAddress(tab)
	tab.setFocused(false)
	if !tab.showsAddress() || tab.input != "mailto:user@example.com" || tab.errorText == "" {
		t.Fatalf("invalid input should remain editable after losing focus: %+v", tab)
	}
}

func TestFirstCommitHidesAddressAndFailureRestoresIt(t *testing.T) {
	var tabs tabSet
	tab := tabs.add(nil)
	tab.startPageLoad("https://example.com/start", false)
	if !tab.commitPageLoad("https://example.com/start") || tab.showsAddress() {
		t.Fatal("a committed main document should hide the address bar without waiting for resources")
	}
	if !tab.failPageLoad(tab.view, "网络错误") || !tab.showsAddress() {
		t.Fatal("a failed first navigation, including a redirect failure, must restore the address bar")
	}
	if tab.input != "https://example.com/start" || tab.errorText == "" {
		t.Fatalf("failure should preserve the submitted address and error: %+v", tab)
	}
}

func TestSupersededLoadFailureDoesNotOverrideNewNavigation(t *testing.T) {
	var tabs tabSet
	oldView := &mygo.WebView{}
	tab := tabs.add(oldView)
	tab.startPageLoad("https://old.example", false)
	tab.cancelPageLoad()
	tab.view = &mygo.WebView{}
	tab.startPageLoad("https://new.example", false)
	if tab.failPageLoad(oldView, "旧请求被取消") {
		t.Fatal("failure from a superseded load must be ignored")
	}
	if !tab.loadingPage || tab.input != "https://new.example" || tab.errorText != "" {
		t.Fatalf("newer navigation was overwritten: %+v", tab)
	}
	if !tab.failPageLoad(tab.view, "新请求失败") || !tab.showsAddress() {
		t.Fatal("failure from the current redirected load should allow retrying")
	}
}

func TestLaterFailureDoesNotMakeOpenedTabNewAgain(t *testing.T) {
	var tabs tabSet
	tab := tabs.add(nil)
	tab.startPageLoad("https://example.com", false)
	tab.commitPageLoad("https://example.com")
	tab.finishPageLoad("https://example.com")
	tab.startPageLoad("https://example.com/next", false)
	tab.failPageLoad(tab.view, "网络错误")
	if tab.showsAddress() {
		t.Fatal("an already opened tab must not become a new tab after a later failure")
	}
}

func TestSameURLRetryIgnoresFailureFromOldView(t *testing.T) {
	var tabs tabSet
	oldView := &mygo.WebView{}
	tab := tabs.add(oldView)
	tab.input = "https://example.com/retry"
	tab.focused = true
	tab.startPageLoad(tab.input, false)
	tab.cancelPageLoad()
	newView := &mygo.WebView{}
	tab.view = newView
	tab.startPageLoad("https://example.com/retry", false)
	if tab.failPageLoad(oldView, "旧加载失败") {
		t.Fatal("a canceled view must not fail a newer load of the same URL")
	}
	if !tab.loadingPage || !tab.focused || tab.errorText != "" || tab.input != "https://example.com/retry" {
		t.Fatalf("the retry or its editable address was disturbed: %+v", tab)
	}
	if !tab.failPageLoad(newView, "新加载失败") || tab.errorText == "" {
		t.Fatal("a failure from the current view should remain retryable")
	}
}

func TestLaterNavigationDoesNotResetOpenedTabOnOldFailure(t *testing.T) {
	var tabs tabSet
	tab := tabs.add(nil)
	tab.startPageLoad("https://example.com/slow", false)
	tab.commitPageLoad("https://example.com/slow")
	tab.pageWillNavigate(true)
	if tab.failPageLoad(tab.view, "旧页面已取消") {
		t.Fatal("an older load failure must not restore the new-tab address bar")
	}
	if tab.showsAddress() || !tab.openedPage {
		t.Fatalf("the already opened tab must stay opened: %+v", tab)
	}
}
