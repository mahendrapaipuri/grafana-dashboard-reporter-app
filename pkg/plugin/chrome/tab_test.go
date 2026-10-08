package chrome

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Auth headers must reach Grafana's origin only. A third-party origin the page
// loads from must neither receive them nor be sent a CORS preflight because of them.
func TestNavigateAndWaitForScopesHeadersToOrigin(t *testing.T) {
	var (
		mu          sync.Mutex
		pageHeaders http.Header
		external    []*http.Request
	)

	// A third-party origin such as a public CDN: it allows simple cross-origin
	// GETs, but answers preflights without CORS headers.
	thirdParty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()

		external = append(external, r)

		mu.Unlock()

		if r.Method == http.MethodOptions {
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		fmt.Fprint(w, "ok")
	}))
	defer thirdParty.Close()

	grafana := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/d/test" {
			http.NotFound(w, r)

			return
		}

		mu.Lock()

		pageHeaders = r.Header.Clone()

		mu.Unlock()

		fmt.Fprintf(w, `<html><body><script>
			fetch(%q).then(r => r.text()).then(t => { document.title = t }).catch(() => { document.title = "failed" })
		</script></body></html>`, thirdParty.URL+"/resource")
	}))
	defer grafana.Close()

	instance, err := NewLocalBrowserInstance(t.Context(), log.NewNullLogger(), true)
	if err != nil {
		t.Skipf("Chrome not available: %v", err)
	}

	defer instance.Close(log.NewNullLogger())

	tab := instance.NewTab(log.NewNullLogger(), nil)
	tab.WithTimeout(30 * time.Second)

	defer tab.Close(log.NewNullLogger())

	headers := map[string]any{
		"Authorization": "Bearer test-token",
		"Cookie":        "grafana_session=test-session",
	}

	err = tab.NavigateAndWaitFor(grafana.URL+"/d/test", headers, "networkIdle", nil)
	require.NoError(t, err)

	var title string

	require.NoError(t, tab.Run(chromedp.Title(&title)))

	mu.Lock()
	defer mu.Unlock()

	require.NotNil(t, pageHeaders)
	assert.Equal(t, "Bearer test-token", pageHeaders.Get("Authorization"))
	assert.Contains(t, pageHeaders.Get("Cookie"), "grafana_session=test-session")

	require.NotEmpty(t, external)

	for _, r := range external {
		assert.NotEqual(t, http.MethodOptions, r.Method, "third-party origin was sent a CORS preflight")
		assert.Empty(t, r.Header.Get("Authorization"), "Authorization sent to third-party origin")
		assert.Empty(t, r.Header.Get("Cookie"), "Cookie sent to third-party origin")
	}

	assert.Equal(t, "ok", title, "cross-origin fetch failed")
}

func TestOriginPattern(t *testing.T) {
	for addr, want := range map[string]string{
		"http://127.0.0.1:3000/d/abc?x=1":       "http://127.0.0.1:3000/*",
		"https://Grafana.Example.com:443/d/abc": "https://grafana.example.com/*",
		"http://grafana.example.com:80":         "http://grafana.example.com/*",
		"https://grafana.example.com:8443/":     "https://grafana.example.com:8443/*",
		"http://[::1]:3000/d/abc":               "http://[::1]:3000/*",
	} {
		got, err := originPattern(addr)
		require.NoError(t, err)
		assert.Equal(t, want, got, addr)
	}
}
