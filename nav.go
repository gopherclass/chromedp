package chromedp

import (
	"context"
	"errors"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
)

// NavigateAction are actions that manipulate the navigation of the browser.
type NavigateAction Action

// Navigate is an action that navigates the current frame.
func Navigate(urlstr string) NavigateAction {
	return ActionFunc(func(ctx context.Context) error {
		expect, release := expectLifecycleLoaded(ctx)
		defer release()
		_, _, _, err = page.Navigate(urlstr).Do(ctx)
		if err != nil {
			return err
		}
		return expect()
	})
}

// NavigationEntries is an action that retrieves the page's navigation history
// entries.
func NavigationEntries(currentIndex *int64, entries *[]*page.NavigationEntry) NavigateAction {
	if currentIndex == nil || entries == nil {
		panic("currentIndex and entries cannot be nil")
	}

	return ActionFunc(func(ctx context.Context) error {
		var err error
		*currentIndex, *entries, err = page.GetNavigationHistory().Do(ctx)
		return err
	})
}

// NavigateToHistoryEntry is an action to navigate to the specified navigation
// entry.
func NavigateToHistoryEntry(entryID int64) NavigateAction {
	return ActionFunc(func(ctx context.Context) error {
		expect, release := expectLifecycleLoaded(ctx)
		defer release()
		if err := page.NavigateToHistoryEntry(entryID).Do(ctx); err != nil {
			return err
		}
		return expect()
	})
}

// NavigateBack is an action that navigates the current frame backwards in its
// history.
func NavigateBack() NavigateAction {
	return ActionFunc(func(ctx context.Context) error {
		cur, entries, err := page.GetNavigationHistory().Do(ctx)
		if err != nil {
			return err
		}

		if cur <= 0 || cur > int64(len(entries)-1) {
			return errors.New("invalid navigation entry")
		}
		expect, release := expectLifecycleLoaded(ctx)
		defer release()
		entryID := entries[cur-1].ID
		if err := page.NavigateToHistoryEntry(entryID).Do(ctx); err != nil {
			return err
		}
		return expect()
	})
}

// NavigateForward is an action that navigates the current frame forwards in
// its history.
func NavigateForward() NavigateAction {
	return ActionFunc(func(ctx context.Context) error {
		cur, entries, err := page.GetNavigationHistory().Do(ctx)
		if err != nil {
			return err
		}

		if cur < 0 || cur >= int64(len(entries)-1) {
			return errors.New("invalid navigation entry")
		}
		expect, release := expectLifecycleLoaded(ctx)
		defer release()
		entryID := entries[cur+1].ID
		if err := page.NavigateToHistoryEntry(entryID).Do(ctx); err != nil {
			return err
		}
		return expect()
	})
}

// Reload is an action that reloads the current page.
func Reload() NavigateAction {
	return ActionFunc(func(ctx context.Context) error {
		expect, release := expectLifecycleLoaded(ctx)
		defer release()
		if err := page.Reload().Do(ctx); err != nil {
			return err
		}
		return expect()
	})
}

// Stop is an action that stops all navigation and pending resource retrieval.
func Stop() NavigateAction {
	return page.StopLoading()
}

// CaptureScreenshot is an action that captures/takes a screenshot of the
// current browser viewport.
//
// See the Screenshot action to take a screenshot of a specific element.
//
// See the 'screenshot' example in the https://github.com/chromedp/examples
// project for an example of taking a screenshot of the entire page.
func CaptureScreenshot(res *[]byte) Action {
	if res == nil {
		panic("res cannot be nil")
	}

	return ActionFunc(func(ctx context.Context) error {
		var err error
		*res, err = page.CaptureScreenshot().Do(ctx)
		return err
	})
}

// Location is an action that retrieves the document location.
func Location(urlstr *string) Action {
	if urlstr == nil {
		panic("urlstr cannot be nil")
	}
	return EvaluateAsDevTools(`document.location.toString()`, urlstr)
}

// Title is an action that retrieves the document title.
func Title(title *string) Action {
	if title == nil {
		panic("title cannot be nil")
	}
	return EvaluateAsDevTools(`document.title`, title)
}

type isExpectedEvent func(i interface{}) bool

type expectFunc func() error

func expectEvent(r context.Context, is isExpectedEvent) (expectFunc, context.CancelFunc) {
	ok := make(chan struct{})
	listener, release := context.WithCancel(r)
	listen := func(i interface{}) {
		if is(i) {
			close(ok)
			release()
		}
	}
	ListenTarget(listener, listen)
	expect := func() error {
		select {
		case <-ok:
			return nil
		case <-r.Done():
			return r.Err()
		}
	}
	return expect, release
}

func expectLifecycleEvent(r context.Context, name string) (expectFunc, context.CancelFunc) {
	is := func(i interface{}) bool {
		e, recv := i.(*page.EventLifecycleEvent)
		if !recv {
			return false
		}
		return e.Name == name && e.FrameID == navigatedFrameID(r)

	}
	return expectEvent(r, is)
}

func expectLifecycleLoaded(r context.Context) (expectFunc, context.CancelFunc) {
	return expectLifecycleEvent(r, "load")
}

func navigatedFrameID(ctx context.Context) cdp.FrameID {
	target, ok := cdp.ExecutorFromContext(ctx).(*Target)
	if !ok {
		return ""
	}
	target.curMu.RLock()
	if target.cur == nil {
		return ""
	}
	id := target.cur.ID
	target.curMu.RUnlock()
	return id
}
