package chrome

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

/*
	This file contains chromedp package related helper functions.
	Sources:
		- https://github.com/chromedp/chromedp/issues/1044
		- https://github.com/chromedp/chromedp/issues/431#issuecomment-592950397
		- https://github.com/chromedp/chromedp/issues/87
		- https://github.com/chromedp/examples/tree/master
*/

// enableLifeCycleEvents enables the chromedp life cycle events.
func enableLifeCycleEvents() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		err := page.Enable().Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to enable page: %w", err)
		}

		err = page.SetLifecycleEventsEnabled(true).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to enable lifecycle events: %w", err)
		}

		return nil
	}
}

// waitFor blocks until eventName is received.
// Examples of events you can wait for:
//
//	init, DOMContentLoaded, firstPaint,
//	firstContentfulPaint, firstImagePaint,
//	firstMeaningfulPaintCandidate,
//	load, networkAlmostIdle, firstMeaningfulPaint, networkIdle
//
// This is not super reliable, I've already found incidental cases where
// networkIdle was sent before load. It's probably smart to see how
// puppeteer implements this exactly.
func waitFor(eventName string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		ch := make(chan struct{})
		cctx, cancel := context.WithCancel(ctx)
		chromedp.ListenTarget(cctx, func(ev any) {
			switch e := ev.(type) {
			case *page.EventLifecycleEvent:
				if e.Name == eventName {
					cancel()
					close(ch)
				}
			}
		})

		select {
		case <-ch:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// setHeaders adds headers to requests made to the origin of addr only, so
// credentials are never sent to third-party origins a panel loads from.
func setHeaders(addr string, headers map[string]any) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		pattern, err := originPattern(addr)
		if err != nil {
			return err
		}

		injected := make([]*fetch.HeaderEntry, 0, len(headers))
		replaced := make(map[string]bool, len(headers))

		for name, value := range headers {
			injected = append(injected, &fetch.HeaderEntry{Name: name, Value: fmt.Sprint(value)})
			replaced[strings.ToLower(name)] = true
		}

		executor := cdp.WithExecutor(ctx, chromedp.FromContext(ctx).Target)

		chromedp.ListenTarget(ctx, func(ev any) {
			e, ok := ev.(*fetch.EventRequestPaused)
			if !ok {
				return
			}

			// Listeners must not block, so the request is continued from a goroutine.
			go func() {
				entries := slices.Clone(injected)

				for name, value := range e.Request.Headers {
					if !replaced[strings.ToLower(name)] {
						entries = append(entries, &fetch.HeaderEntry{Name: name, Value: fmt.Sprint(value)})
					}
				}

				err := fetch.ContinueRequest(e.RequestID).WithHeaders(entries).Do(executor)
				if err != nil {
					// A paused request that is never resumed would hang the page load.
					_ = fetch.FailRequest(e.RequestID, network.ErrorReasonFailed).Do(executor)
				}
			}()
		})

		return fetch.Enable().
			WithPatterns([]*fetch.RequestPattern{{URLPattern: pattern, RequestStage: fetch.RequestStageRequest}}).
			Do(ctx)
	}
}

// originPattern returns a Fetch URL pattern matching every URL on the origin of
// addr, normalised the way Chrome normalises request URLs.
func originPattern(addr string) (string, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", fmt.Errorf("failed to parse url %s: %w", addr, err)
	}

	host := strings.ToLower(u.Host)

	switch u.Scheme {
	case "http":
		host = strings.TrimSuffix(host, ":80")
	case "https":
		host = strings.TrimSuffix(host, ":443")
	}

	return u.Scheme + "://" + host + "/*", nil
}
