package kube

import (
	_ "embed"

	apischema "k8s.io/apimachinery/pkg/runtime/schema"
)

//go:embed testdata/openapi_v3_0_0/v1.json
var nativeCoreV1OpenAPI []byte

//go:embed testdata/openapi_v3_0_0/apiextensions.k8s.io/v1.json
var nativeAPIExtensionsV1OpenAPI []byte

//go:embed testdata/openapi_v3_0_0/apps/v1.json
var nativeAppsV1OpenAPI []byte

//go:embed testdata/openapi_v3_0_0/batch/v1.json
var nativeBatchV1OpenAPI []byte

//go:embed testdata/openapi_v3_0_0/batch/v1beta1.json
var nativeBatchV1Beta1OpenAPI []byte

func NativeOpenAPIV3Document(group, version string) ([]byte, bool) {
	doc, ok := nativeOpenAPIV3Documents[apischema.GroupVersion{Group: group, Version: version}]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), doc...), true
}

var nativeOpenAPIV3Documents = map[apischema.GroupVersion][]byte{
	{Version: "v1"}: nativeCoreV1OpenAPI,
	{Group: "apiextensions.k8s.io", Version: "v1"}: nativeAPIExtensionsV1OpenAPI,
	{Group: "apps", Version: "v1"}:                 nativeAppsV1OpenAPI,
	{Group: "batch", Version: "v1"}:                nativeBatchV1OpenAPI,
	{Group: "batch", Version: "v1beta1"}:           nativeBatchV1Beta1OpenAPI,
}
