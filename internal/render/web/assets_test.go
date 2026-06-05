package web

import (
	"bytes"
	"os"
	"testing"
)

func TestFernComponentRuntimeAssetsStaySynced(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		dst  string
	}{
		{
			name: "css",
			src:  "assets/kubectl-doc.css",
			dst:  "../../../fern/components/kubectl-doc/kubectl-doc.css",
		},
		{
			name: "runtime",
			src:  "assets/kubectl-doc.js",
			dst:  "../../../fern/components/kubectl-doc/kubectl-doc-runtime.js",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src, err := os.ReadFile(tc.src)
			if err != nil {
				t.Fatalf("read source asset: %v", err)
			}
			dst, err := os.ReadFile(tc.dst)
			if err != nil {
				t.Fatalf("read Fern component asset: %v", err)
			}
			if !bytes.Equal(src, dst) {
				t.Fatalf("Fern component asset %s is not synced with %s; run make gen", tc.dst, tc.src)
			}
		})
	}
}
