package outbound

import "testing"

func TestPrepareX365XHTTPHeaders(t *testing.T) {
	base := map[string]string{"Host": "dldir1.qq.com"}

	prepared := prepareX365XHTTPHeaders(base, true)
	if prepared["User-Agent"] != x365UserAgent {
		t.Fatalf("User-Agent = %q, want %q", prepared["User-Agent"], x365UserAgent)
	}
	if prepared["Host"] != "dldir1.qq.com" {
		t.Fatalf("Host = %q, want dldir1.qq.com", prepared["Host"])
	}
	if len(base) != 1 {
		t.Fatalf("input map was mutated: %v", base)
	}

	custom := map[string]string{"User-Agent": "my-agent"}
	prepared = prepareX365XHTTPHeaders(custom, true)
	if prepared["User-Agent"] != "my-agent" {
		t.Fatalf("explicit User-Agent must win, got %q", prepared["User-Agent"])
	}

	prepared = prepareX365XHTTPHeaders(base, false)
	if _, ok := prepared["User-Agent"]; ok {
		t.Fatal("non-X365 nodes must not get an injected User-Agent")
	}
}
