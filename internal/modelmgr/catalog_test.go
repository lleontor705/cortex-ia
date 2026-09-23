package modelmgr

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func TestCatalogDaemonTierFiltersEnabledAndDropsSecrets(t *testing.T) {
	body := `[
		{"modelID":"claude-sonnet-4-5","providerID":"anthropic","name":"Sonnet","enabled":true,
		 "capabilities":{"reasoning":true},
		 "variants":[
			{"id":"low","settings":{"note":"SUPERSECRETSETTING"}},
			{"id":"high","headers":{"authorization":"SUPERSECRETHEADER"},"body":{"prompt":"SUPERSECRETBODY"}}
		 ]},
		{"modelID":"gpt-5.2","providerID":"openai","enabled":false,"variants":[{"id":"x"}]}
	]`
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/model" {
			return nil, fmt.Errorf("unexpected path %q", request.URL.Path)
		}
		return jsonResponse(http.StatusOK, body), nil
	})
	catalog := New(t.TempDir()).Catalog(CatalogOptions{RoundTripper: transport})
	if catalog.Source != CatalogSourceDaemon || catalog.Truncated {
		t.Fatalf("catalog envelope = %+v", catalog)
	}
	want := []CatalogEntry{{Provider: "anthropic", Model: "claude-sonnet-4-5", Variants: []string{"high", "low"}}}
	if !reflect.DeepEqual(catalog.Entries, want) {
		t.Fatalf("entries = %+v, want %+v", catalog.Entries, want)
	}
	encoded, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"SUPERSECRETSETTING", "SUPERSECRETHEADER", "SUPERSECRETBODY", "settings", "headers", "body"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("catalog leaked %q: %s", secret, encoded)
		}
	}
}

func TestCatalogDaemonFailuresFallThroughToText(t *testing.T) {
	text := func(string, ...string) ([]byte, error) {
		return []byte("anthropic/claude-sonnet-4-5#high\n"), nil
	}
	cases := map[string]roundTripFunc{
		"status-503": func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusServiceUnavailable, "unavailable"), nil
		},
		"status-401": func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, "unauthorized"), nil
		},
		"unreachable": func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial tcp 127.0.0.1:4096: connect: connection refused")
		},
		"malformed": func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, "{not json"), nil
		},
	}
	want := []CatalogEntry{{Provider: "anthropic", Model: "claude-sonnet-4-5", Variants: []string{"high"}}}
	for name, transport := range cases {
		t.Run(name, func(t *testing.T) {
			catalog := New(t.TempDir()).Catalog(CatalogOptions{RoundTripper: transport, RunCommand: text})
			if catalog.Source != CatalogSourceOpencode2 || !reflect.DeepEqual(catalog.Entries, want) {
				t.Fatalf("fallthrough catalog = %+v", catalog)
			}
		})
	}
}

func TestCatalogDaemonEmptyArrayIsASuccessfulTier(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, "[]"), nil
	})
	catalog := New(t.TempDir()).Catalog(CatalogOptions{
		RoundTripper: transport,
		RunCommand:   func(string, ...string) ([]byte, error) { return []byte("openai/gpt-5.2"), nil },
	})
	if catalog.Source != CatalogSourceDaemon || len(catalog.Entries) != 0 {
		t.Fatalf("empty daemon catalog = %+v", catalog)
	}
}

func TestCatalogBothTiersUnavailableDegradesToNone(t *testing.T) {
	catalog := New(t.TempDir()).Catalog(CatalogOptions{
		RoundTripper: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("unreachable")
		}),
		RunCommand: func(string, ...string) ([]byte, error) { return []byte("no model information available"), nil },
	})
	if catalog.Source != CatalogSourceNone || len(catalog.Entries) != 0 || catalog.Truncated {
		t.Fatalf("degraded catalog = %+v", catalog)
	}
}

func TestParseModelCatalogMergesVariantsBareEntriesAndDuplicates(t *testing.T) {
	output := []byte(strings.Join([]string{
		"| anthropic/claude-sonnet-4-5#high |",
		"anthropic/claude-sonnet-4-5#low",
		"anthropic/claude-sonnet-4-5",
		"anthropic/claude-sonnet-4-5#high",
		"anthropic/claude-haiku-4-5",
		"openrouter/anthropic/claude-sonnet-4.5#xhigh",
		"not-a-reference",
	}, "\n"))
	want := []CatalogEntry{
		{Provider: "anthropic", Model: "claude-haiku-4-5", Variants: []string{}},
		{Provider: "anthropic", Model: "claude-sonnet-4-5", Variants: []string{"high", "low"}},
		{Provider: "openrouter", Model: "anthropic/claude-sonnet-4.5", Variants: []string{"xhigh"}},
	}
	if got := ParseModelCatalog(output); !reflect.DeepEqual(got, want) {
		t.Fatalf("parsed = %+v, want %+v", got, want)
	}
}

func TestCatalogTextTierCapsAtEntryLimit(t *testing.T) {
	var builder strings.Builder
	for i := 0; i < catalogEntryLimit+88; i++ {
		fmt.Fprintf(&builder, "vendor/model-%03d\n", i)
	}
	catalog := New(t.TempDir()).Catalog(CatalogOptions{
		RoundTripper: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("unreachable")
		}),
		RunCommand: func(string, ...string) ([]byte, error) { return []byte(builder.String()), nil },
	})
	if catalog.Source != CatalogSourceOpencode2 || !catalog.Truncated {
		t.Fatalf("catalog envelope = %+v", catalog)
	}
	if len(catalog.Entries) != catalogEntryLimit {
		t.Fatalf("entries = %d, want %d", len(catalog.Entries), catalogEntryLimit)
	}
	if catalog.Entries[0].Model != "model-000" || catalog.Entries[catalogEntryLimit-1].Model != "model-511" {
		t.Fatalf("cap kept the wrong window: %s .. %s", catalog.Entries[0].Model, catalog.Entries[catalogEntryLimit-1].Model)
	}
}
