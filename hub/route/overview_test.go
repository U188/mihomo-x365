package route

import (
	"encoding/json"
	"testing"

	"github.com/metacubex/http"
	"github.com/metacubex/http/httptest"
	C "github.com/metacubex/mihomo/constant"
	RC "github.com/metacubex/mihomo/rules/common"
	"github.com/metacubex/mihomo/tunnel"
	"github.com/stretchr/testify/assert"
)

type overviewProxy struct {
	C.Proxy
	name      string
	kind      C.AdapterType
	next      C.Proxy
	lastTouch bool
}

func (proxy *overviewProxy) Name() string            { return proxy.name }
func (proxy *overviewProxy) Type() C.AdapterType     { return proxy.kind }
func (proxy *overviewProxy) Adapter() C.ProxyAdapter { return proxy }
func (proxy *overviewProxy) Unwrap(metadata *C.Metadata, touch bool) C.Proxy {
	proxy.lastTouch = touch
	return proxy.next
}

func TestOverviewExitDoesNotTouchProvidersOrSelectSmartNodes(t *testing.T) {
	smart := &overviewProxy{name: "smart", kind: C.Smart}
	selector := &overviewProxy{name: "GLOBAL", kind: C.Selector, next: smart}
	direct := &overviewProxy{name: "DIRECT", kind: C.Direct}
	proxies := map[string]C.Proxy{"GLOBAL": selector, "DIRECT": direct}
	assert.Equal(t, overviewExit{Chain: []string{"GLOBAL", "smart"}, Smart: true}, overviewExitChain(tunnel.Global, nil, proxies))
	assert.False(t, selector.lastTouch)
	assert.Equal(t, overviewExit{Chain: []string{"DIRECT"}}, overviewExitChain(tunnel.Direct, nil, proxies))
	selector.next = selector
	assert.Equal(t, []string{"GLOBAL"}, overviewExitChain(tunnel.Global, nil, proxies).Chain)
}

func TestOverviewExitRuleOrderAndDefault(t *testing.T) {
	proxies := map[string]C.Proxy{
		"GLOBAL": &overviewProxy{name: "GLOBAL", kind: C.Selector},
		"DIRECT": &overviewProxy{name: "DIRECT", kind: C.Direct},
		"first":  &overviewProxy{name: "first", kind: C.Direct},
		"last":   &overviewProxy{name: "last", kind: C.Direct},
	}
	for _, test := range []struct {
		name  string
		rules []C.Rule
		want  string
	}{
		{name: "no rules", want: "DIRECT"},
		{name: "no MATCH", rules: []C.Rule{RC.NewDomain("example.com", "first")}, want: "DIRECT"},
		{name: "single MATCH", rules: []C.Rule{RC.NewMatch("first")}, want: "first"},
		{name: "first MATCH wins", rules: []C.Rule{RC.NewMatch("first"), RC.NewMatch("last")}, want: "first"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, []string{test.want}, overviewExitChain(tunnel.Rule, test.rules, proxies).Chain)
		})
	}
}

func TestOverviewAuthentication(t *testing.T) {
	handler := router(false, "overview-secret", "", Cors{})
	for _, token := range []string{"", "Bearer overview-secret"} {
		request := httptest.NewRequest(http.MethodGet, "/overview", nil)
		request.Header.Set("Authorization", token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if token == "" {
			assert.Equal(t, http.StatusUnauthorized, response.Code)
			continue
		}
		assert.Equal(t, http.StatusOK, response.Code)
		var result map[string]json.RawMessage
		assert.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		assert.Contains(t, result, "exit")
		assert.Contains(t, result, "providers")
		assert.NotContains(t, result, "proxies")
		assert.NotContains(t, result, "rules")
	}
}
