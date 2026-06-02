GO ?= go
GOLANGCI_LINT_VERSION := $(shell cat .golangci-lint-version)
GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

GITHUB_EXAMPLE := docs/examples/github-crontab.md
EXAMPLE_CRD := internal/cli/testdata/crontab-crd.yaml

.PHONY: gen check-generated test lint

gen:
	@mkdir -p docs/examples
	$(GO) run ./cmd/kubectl-doc -f $(EXAMPLE_CRD) -o markdown-github --all-versions --descriptions=true --columns=100 > $(GITHUB_EXAMPLE)
	$(GO) run ./hack/readmegen --readme README.md --example $(GITHUB_EXAMPLE)

check-generated:
	$(MAKE) gen
	git diff --exit-code -- README.md $(GITHUB_EXAMPLE)

test:
	$(GO) test ./...

lint:
	$(GOLANGCI_LINT) run
	$(GOLANGCI_LINT) fmt --diff
