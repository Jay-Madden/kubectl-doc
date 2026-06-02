package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	apischema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type OpenAPIClient struct {
	baseURL *url.URL
	client  *http.Client
}

type OpenAPIV3GroupVersion struct {
	Group             string
	Version           string
	ServerRelativeURL string
}

type openAPIV3Index struct {
	Paths map[string]openAPIV3Path `json:"paths"`
}

type openAPIV3Path struct {
	ServerRelativeURL string `json:"serverRelativeURL"`
}

func LoadOpenAPIClient() (*OpenAPIClient, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	return NewOpenAPIClientForConfig(restConfig)
}

func NewOpenAPIClientForConfig(restConfig *rest.Config) (*OpenAPIClient, error) {
	baseURL, _, err := rest.DefaultServerUrlFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("build API server URL: %w", err)
	}
	httpClient, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("build API server HTTP client: %w", err)
	}
	return NewOpenAPIClient(baseURL, httpClient), nil
}

func NewOpenAPIClient(baseURL *url.URL, httpClient *http.Client) *OpenAPIClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &OpenAPIClient{baseURL: baseURL, client: httpClient}
}

func (c *OpenAPIClient) GroupVersionDocument(ctx context.Context, group, version string) ([]byte, error) {
	paths, err := c.GroupVersions(ctx)
	if err != nil {
		return nil, err
	}

	serverRelativeURL := ""
	for _, candidate := range paths {
		if candidate.Group == group && candidate.Version == version {
			serverRelativeURL = candidate.ServerRelativeURL
			break
		}
	}
	if serverRelativeURL == "" {
		gv := apischema.GroupVersion{Group: group, Version: version}
		return nil, fmt.Errorf("OpenAPI v3 document for %s not found", gv.String())
	}
	return c.get(ctx, serverRelativeURL)
}

func (c *OpenAPIClient) GroupVersions(ctx context.Context) ([]OpenAPIV3GroupVersion, error) {
	data, err := c.get(ctx, "/openapi/v3")
	if err != nil {
		return nil, err
	}

	var index openAPIV3Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("decode OpenAPI v3 index: %w", err)
	}

	groupVersions := make([]OpenAPIV3GroupVersion, 0, len(index.Paths))
	for key, value := range index.Paths {
		gv, ok := groupVersionFromOpenAPIPath(key)
		if !ok || value.ServerRelativeURL == "" {
			continue
		}
		groupVersions = append(groupVersions, OpenAPIV3GroupVersion{
			Group:             gv.Group,
			Version:           gv.Version,
			ServerRelativeURL: value.ServerRelativeURL,
		})
	}
	return groupVersions, nil
}

func (c *OpenAPIClient) get(ctx context.Context, serverRelativeURL string) ([]byte, error) {
	requestURL, err := c.urlFor(serverRelativeURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", serverRelativeURL, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", serverRelativeURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get %s: status %d: %s", serverRelativeURL, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func (c *OpenAPIClient) urlFor(serverRelativeURL string) (*url.URL, error) {
	if c.baseURL == nil {
		return nil, fmt.Errorf("base URL is not configured")
	}
	relative, err := url.Parse(serverRelativeURL)
	if err != nil {
		return nil, fmt.Errorf("parse server-relative URL %q: %w", serverRelativeURL, err)
	}
	if relative.IsAbs() {
		return nil, fmt.Errorf("OpenAPI URL %q must be server-relative", serverRelativeURL)
	}

	out := *c.baseURL
	basePath := strings.TrimRight(out.Path, "/")
	relativePath := strings.TrimLeft(relative.Path, "/")
	out.Path = path.Join(basePath, relativePath)
	if strings.HasPrefix(relative.Path, "/") && basePath == "" {
		out.Path = "/" + strings.TrimLeft(out.Path, "/")
	}
	out.RawQuery = relative.RawQuery
	return &out, nil
}

func groupVersionFromOpenAPIPath(key string) (apischema.GroupVersion, bool) {
	trimmed := strings.Trim(strings.TrimSpace(key), "/")
	parts := strings.Split(trimmed, "/")
	switch {
	case len(parts) == 2 && parts[0] == "api":
		return apischema.GroupVersion{Version: parts[1]}, true
	case len(parts) == 3 && parts[0] == "apis":
		return apischema.GroupVersion{Group: parts[1], Version: parts[2]}, true
	default:
		return apischema.GroupVersion{}, false
	}
}
