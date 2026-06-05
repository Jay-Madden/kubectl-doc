GO ?= go
GOLANGCI_LINT_VERSION := $(shell cat .golangci-lint-version)
GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

GITHUB_EXAMPLE := docs/examples/github-dynamographdeployment.md
HTML_EXAMPLE := docs/examples/html-dynamographdeployment.html
KRO_EXAMPLE := docs/examples/kro-dynamographdeployment.yaml
EXAMPLE_CRD := internal/cli/testdata/dynamographdeployment-crd.yaml
FERN_COMPONENT_DIR := fern/components/kubectl-doc

.PHONY: gen check-generated test lint

gen:
	@mkdir -p docs/examples $(FERN_COMPONENT_DIR)
	cp internal/render/web/assets/kubectl-doc.js $(FERN_COMPONENT_DIR)/kubectl-doc-runtime.js
	$(GO) run ./hack/fernstyles --css internal/render/web/assets/kubectl-doc.css --out $(FERN_COMPONENT_DIR)/kubectl-doc-styles.ts
	$(GO) run ./cmd/kubectl-doc -f $(EXAMPLE_CRD) -o markdown-github --all-versions --descriptions=true --expand-depth=4 --columns=100 > $(GITHUB_EXAMPLE)
	$(GO) run ./cmd/kubectl-doc -f $(EXAMPLE_CRD) -o html --all-versions --descriptions=true --expand-depth=4 --columns=100 > $(HTML_EXAMPLE)
	$(GO) run ./cmd/kubectl-doc -f $(EXAMPLE_CRD) -o kro --all-versions --descriptions=true > $(KRO_EXAMPLE)
	$(GO) run ./hack/readmegen --readme README.md --example $(GITHUB_EXAMPLE)

check-generated:
	$(MAKE) gen
	git diff --exit-code -- README.md docs/examples $(FERN_COMPONENT_DIR)/kubectl-doc-runtime.js $(FERN_COMPONENT_DIR)/kubectl-doc-styles.ts

test:
	$(GO) test ./...

lint:
	$(GOLANGCI_LINT) run
	$(GOLANGCI_LINT) fmt --diff
