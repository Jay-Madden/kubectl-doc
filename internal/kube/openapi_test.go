package kube

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestOpenAPIClientFetchesGroupVersionDocument(t *testing.T) {
	transport := &fakeRoundTripper{
		responses: map[string]string{
			"https://cluster.example/openapi/v3": `{
				"paths": {
					"api/v1": {"serverRelativeURL": "/openapi/v3/api/v1?hash=core"},
					"apis/apps/v1": {"serverRelativeURL": "/openapi/v3/apis/apps/v1?hash=apps"}
				}
			}`,
			"https://cluster.example/openapi/v3/apis/apps/v1?hash=apps": `{"openapi":"3.0.0","info":{"title":"apps"}}`,
		},
	}
	baseURL, err := url.Parse("https://cluster.example")
	if err != nil {
		t.Fatal(err)
	}
	client := NewOpenAPIClient(baseURL, &http.Client{Transport: transport})

	data, err := client.GroupVersionDocument(context.Background(), "apps", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"openapi":"3.0.0","info":{"title":"apps"}}` {
		t.Fatalf("unexpected document: %s", string(data))
	}

	expected := []string{"https://cluster.example/openapi/v3", "https://cluster.example/openapi/v3/apis/apps/v1?hash=apps"}
	if strings.Join(transport.requested, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("unexpected requests\nwant: %v\ngot:  %v", expected, transport.requested)
	}
}

func TestOpenAPIClientUsesBaseURLPathPrefix(t *testing.T) {
	transport := &fakeRoundTripper{
		responses: map[string]string{
			"https://cluster.example/proxy/openapi/v3": `{
				"paths": {
					"api/v1": {"serverRelativeURL": "/openapi/v3/api/v1?hash=core"}
				}
			}`,
			"https://cluster.example/proxy/openapi/v3/api/v1?hash=core": `{"openapi":"3.0.0","info":{"title":"core"}}`,
		},
	}
	baseURL, err := url.Parse("https://cluster.example/proxy")
	if err != nil {
		t.Fatal(err)
	}
	client := NewOpenAPIClient(baseURL, &http.Client{Transport: transport})

	if _, err := client.GroupVersionDocument(context.Background(), "", "v1"); err != nil {
		t.Fatal(err)
	}

	expected := []string{"https://cluster.example/proxy/openapi/v3", "https://cluster.example/proxy/openapi/v3/api/v1?hash=core"}
	if strings.Join(transport.requested, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("unexpected requests\nwant: %v\ngot:  %v", expected, transport.requested)
	}
}

func TestOpenAPIClientReportsMissingGroupVersion(t *testing.T) {
	transport := &fakeRoundTripper{
		responses: map[string]string{
			"https://cluster.example/openapi/v3": `{"paths":{}}`,
		},
	}
	baseURL, err := url.Parse("https://cluster.example")
	if err != nil {
		t.Fatal(err)
	}
	client := NewOpenAPIClient(baseURL, &http.Client{Transport: transport})

	_, err = client.GroupVersionDocument(context.Background(), "apps", "v1")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "OpenAPI v3 document for apps/v1 not found" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAPIClientRejectsAbsoluteServerRelativeURLs(t *testing.T) {
	baseURL, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	client := NewOpenAPIClient(baseURL, nil)

	_, err = client.urlFor("https://evil.example/openapi/v3/apis/apps/v1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must be server-relative") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAPIClientReportsHTTPStatus(t *testing.T) {
	transport := &fakeRoundTripper{
		statuses: map[string]int{
			"https://cluster.example/openapi/v3": http.StatusNotFound,
		},
		responses: map[string]string{
			"https://cluster.example/openapi/v3": "missing",
		},
	}
	baseURL, err := url.Parse("https://cluster.example")
	if err != nil {
		t.Fatal(err)
	}
	client := NewOpenAPIClient(baseURL, &http.Client{Transport: transport})

	_, err = client.GroupVersions(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 404: missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGroupVersionFromOpenAPIPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		group   string
		version string
		ok      bool
	}{
		{name: "core", path: "api/v1", version: "v1", ok: true},
		{name: "core leading slash", path: "/api/v1", version: "v1", ok: true},
		{name: "group", path: "apis/apps/v1", group: "apps", version: "v1", ok: true},
		{name: "invalid", path: "openapi/v3", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gv, ok := groupVersionFromOpenAPIPath(test.path)
			if ok != test.ok {
				t.Fatalf("expected ok=%t, got %t", test.ok, ok)
			}
			if gv.Group != test.group || gv.Version != test.version {
				t.Fatalf("expected %s/%s, got %#v", test.group, test.version, gv)
			}
		})
	}
}

type fakeRoundTripper struct {
	requested []string
	statuses  map[string]int
	responses map[string]string
}

func (rt *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	requestURL := req.URL.String()
	rt.requested = append(rt.requested, requestURL)
	body, ok := rt.responses[requestURL]
	if !ok {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewBufferString("not found")),
			Header:     http.Header{},
			Request:    req,
		}, nil
	}

	status := rt.statuses[requestURL]
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     http.Header{},
		Request:    req,
	}, nil
}
