APP_NAME := nixos
BUILD_VAR_PKG := github.com/nix-community/nixos-cli/internal/build/vars

VERSION ?= $(shell git describe --tags --always)
COMMIT_HASH ?= $(shell git rev-parse HEAD)

# Configurable parameters
FLAKE ?= true
NIXPKGS_REVISION ?= 25.05

LDFLAGS := -X $(BUILD_VAR_PKG).Version=$(VERSION)
LDFLAGS += -X $(BUILD_VAR_PKG).GitRevision=$(COMMIT_HASH)
LDFLAGS += -X $(BUILD_VAR_PKG).Flake=$(FLAKE)
LDFLAGS += -X $(BUILD_VAR_PKG).NixpkgsVersion=$(NIXPKGS_REVISION)

# Disable CGO by default. This should be a static executable.
CGO_ENABLED ?= 0

all: build

.PHONY: build
build:
	@echo "building $(APP_NAME)..."
	CGO_ENABLED=$(CGO_ENABLED) go build -o ./$(APP_NAME) -ldflags="$(LDFLAGS)" .

.PHONY: clean
clean:
	@echo "cleaning up..."
	go clean
	rm -rf site/ man/

.PHONY: test
test:
	@echo "running tests..."
	CGO_ENABLED=$(CGO_ENABLED) go test ./...

.PHONY: gen-docs
gen-docs: gen-manpages gen-site

.PHONY: site
site: gen-site
	# -d is interpreted relative to the book directory.
	mdbook build ./doc -d ../site

.PHONY: gen-site
gen-site:
	go run doc/build.go site -r $(COMMIT_HASH)

.PHONY: gen-site
gen-manpages:
	go run doc/build.go man

.PHONY: serve-site
serve-site:
	mdbook serve ./doc --open
