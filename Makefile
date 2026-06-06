GO ?= go
NPM ?= npm
NPM_CACHE ?= $(CURDIR)/.cache/npm
GOLANGCI_LINT_VERSION := $(shell cat .golangci-lint-version)
GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

GITHUB_EXAMPLE := docs/examples/github-dynamographdeployment.md
HTML_EXAMPLE := docs/examples/html-dynamographdeployment.html
KRO_EXAMPLE := docs/examples/kro-dynamographdeployment.yaml
EXAMPLE_CRD := internal/cli/testdata/dynamographdeployment-crd.yaml
LIGHT_EXAMPLE_CRD := internal/cli/testdata/dynamographdeployment-light-crd.yaml
README_EXAMPLE := docs/examples/readme-dynamographdeployment.md
REACT_COMPONENT_DIR := react/kubectl-doc
FERN_DEV_DIR := fern/dev
FERN_DEV_SCHEMA_DIR := $(FERN_DEV_DIR)/public/schemas

.PHONY: gen gen-fern-dev-fixtures check-generated test lint fern-dev check-fern-dev

gen:
	@mkdir -p docs/examples $(REACT_COMPONENT_DIR)
	cp internal/render/web/assets/kubectl-doc.js $(REACT_COMPONENT_DIR)/kubectl-doc-runtime.js
	$(GO) run ./hack/fernstyles --css internal/render/web/assets/kubectl-doc.css --out $(REACT_COMPONENT_DIR)/kubectl-doc-styles.ts
	$(GO) run ./cmd/kubectl-doc -f $(EXAMPLE_CRD) -o markdown-github --all-versions --descriptions=true --expand-depth=4 --columns=100 > $(GITHUB_EXAMPLE)
	$(GO) run ./cmd/kubectl-doc -f $(EXAMPLE_CRD) -o html --all-versions --descriptions=true --expand-depth=4 --columns=100 > $(HTML_EXAMPLE)
	$(GO) run ./cmd/kubectl-doc -f $(EXAMPLE_CRD) -o kro --all-versions --descriptions=true > $(KRO_EXAMPLE)
	$(GO) run ./cmd/kubectl-doc -f $(LIGHT_EXAMPLE_CRD) -o markdown-github --all-versions --descriptions=true --expand-depth=4 --columns=100 > $(README_EXAMPLE)
	$(GO) run ./hack/readmegen --readme README.md --example $(README_EXAMPLE)

gen-fern-dev-fixtures:
	@mkdir -p $(FERN_DEV_SCHEMA_DIR)
	$(GO) run ./hack/ferndev --crd $(EXAMPLE_CRD) --out $(FERN_DEV_SCHEMA_DIR)

check-generated:
	$(MAKE) gen
	cmp -s internal/render/web/assets/kubectl-doc.js $(REACT_COMPONENT_DIR)/kubectl-doc-runtime.js
	git diff --exit-code -- README.md docs/examples $(REACT_COMPONENT_DIR)/kubectl-doc-styles.ts

test:
	$(GO) test ./...

lint:
	$(GOLANGCI_LINT) run
	$(GOLANGCI_LINT) fmt --diff

fern-dev: gen gen-fern-dev-fixtures
	$(NPM) --prefix $(FERN_DEV_DIR) install --cache $(NPM_CACHE)
	$(NPM) --prefix $(FERN_DEV_DIR) run dev

check-fern-dev: gen gen-fern-dev-fixtures
	$(NPM) --prefix $(FERN_DEV_DIR) ci --cache $(NPM_CACHE)
	$(NPM) --prefix $(FERN_DEV_DIR) run build
	$(NPM) --prefix $(FERN_DEV_DIR) exec playwright install chromium
	$(NPM) --prefix $(FERN_DEV_DIR) run test
