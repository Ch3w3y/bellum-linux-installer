# Makefile for Bellum Linux Installer Go
#
# Targets:
#   build   - build both binaries for the host arch
#   release - reproducible release archive + MANIFEST + SHA256SUMS
#   check   - the CI gate (gofmt/vet/test, pinned modules)
#   verify-release - recompute checksums of a release dir and compare
#
# Reproducibility rules:
# - No `go mod tidy` as a build side effect; go.mod/go.sum are pinned by hand
#   and `go mod tidy -diff` verifies them in the check target.
# - Fixed build flags (-trimpath, deterministic ldflags), fixed gzip and tar
#   metadata (mtime/owner/mode), fixed locale/sort, so the archive and its
#   checksums are byte-for-byte identical for the same VERSION + source tree.
# - The installer itself is never executed by these targets.

SHELL := /bin/bash
.SUFFIXES:
.DELETE_ON_ERROR:

.PHONY: all build check release verify-release clean help

VERSION ?= 2.3.0
# VERSION may be overridden on the command line, e.g.
#   make release VERSION=2.1.0-rc1
ifeq ($(VERSION),)
$(error VERSION must not be empty)
endif
VERSION := $(strip $(VERSION))

# Go configuration
GO := go
GOOS ?= linux
GOARCH ?= amd64
CGO_ENABLED := 0
export GOOS GOARCH CGO_ENABLED

# Reproducible-build switches: stable tool behavior regardless of host env.
export LC_ALL := C
export SOURCE_DATE_EPOCH ?= 946684800

# Directories
INSTALLER_DIR := bellum-installer
UNINSTALLER_DIR := bellum-uninstaller
PACKAGES_DIR := packages

# Output files
INSTALLER_BIN := installer
UNINSTALLER_BIN := uninstaller
RELEASE_TARBALL := bellum-installer-linux-$(GOARCH)-$(VERSION).tar.gz
RELEASE_STEM := bellum-installer-linux-$(GOARCH)-$(VERSION)
RELEASE_DIR := dist/$(RELEASE_STEM)
DIST_DIR := dist

# Deterministic ldflags: no paths, no timestamps. Version stamping is via
# -X ldflags on config vars when present; empty by default to stay stable.
GO_LDFLAGS := -s -w -X bellum-installer/pkg/config.InstallerVersion=$(VERSION)

GO_BUILD_FLAGS := -buildvcs=false -trimpath -mod=readonly -ldflags "$(GO_LDFLAGS)"

# Default target
all: build

# The CI gate: exactly what .github/workflows/ci.yml runs.
check:
	@echo "[CHECK] gofmt..."
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Files not gofmt-formatted:"; echo "$$unformatted"; \
		gofmt -d $$unformatted; exit 1; \
	fi
	@echo "[CHECK] go vet..."
	$(GO) vet ./...
	@echo "[CHECK] go test..."
	$(GO) test ./...
	@echo "[CHECK] module pinning (no tidy drift)..."
	$(GO) mod verify
	$(GO) mod tidy -diff
	@echo "[CHECK] Done. All checks passed."

# Build both binaries for GOOS/GOARCH (default linux/amd64)
build: $(INSTALLER_BIN) $(UNINSTALLER_BIN)
	@echo "[BUILD] Done! Both binaries built successfully."

# Build bellum-installer
$(INSTALLER_BIN):
	@echo "[BUILD] Building installer..."
	$(GO) build $(GO_BUILD_FLAGS) -o $(INSTALLER_BIN) ./$(INSTALLER_DIR)

# Build bellum-uninstaller
$(UNINSTALLER_BIN):
	@echo "[BUILD] Building uninstaller..."
	$(GO) build $(GO_BUILD_FLAGS) -o $(UNINSTALLER_BIN) ./$(UNINSTALLER_DIR)

# Reproducible release: binaries + versioned MANIFEST + SHA256SUMS + tarball.
#
# Steps:
#  1. Build with -trimpath / -mod=readonly (no tidy, no side effects).
#  2. Stage binaries + packages under dist/$(RELEASE_STEM)/.
#  3. Write MANIFEST (version, files, sizes, sha256) and SHA256SUMS.
#  4. Pack the tarball with fixed metadata so reruns are byte-identical.
release:
	@test -z "$$(git status --porcelain --untracked-files=all)" || { echo '[RELEASE] Refusing dirty tree'; exit 1; }
	@$(MAKE) -B $(INSTALLER_BIN) $(UNINSTALLER_BIN)
	@echo "[RELEASE] Staging $(RELEASE_STEM)..."
	@rm -rf $(RELEASE_DIR)
	@mkdir -p $(RELEASE_DIR)/$(PACKAGES_DIR)
	@cp $(INSTALLER_BIN) $(RELEASE_DIR)/installer
	@cp $(UNINSTALLER_BIN) $(RELEASE_DIR)/uninstaller
	@git ls-files -z -- $(PACKAGES_DIR) | while IFS= read -r -d '' file; do \
		mkdir -p "$(RELEASE_DIR)/$$(dirname "$$file")"; cp "$$file" "$(RELEASE_DIR)/$$file"; \
	done
	@echo "[RELEASE] Writing MANIFEST and checksums..."
	@MANIFEST="$(RELEASE_DIR)/MANIFEST.md"; \
	{ \
		printf '# Bellum Linux Installer release manifest\n\n'; \
		printf 'version: %s\n' '$(VERSION)'; \
		printf 'created: %s\n' "$$(date -u -d @$(SOURCE_DATE_EPOCH) '+%Y-%m-%dT%H:%M:%SZ')"; \
		printf 'goos: %s\n' '$(GOOS)'; \
		printf 'goarch: %s\n' '$(GOARCH)'; \
		printf 'go: %s\n' "$$($(GO) env GOVERSION 2>/dev/null || echo unknown)"; \
		printf 'commit: %s\n' "$$(git rev-parse HEAD 2>/dev/null || echo unknown)"; \
		printf '\n'; \
		printf 'files:\n'; \
		cd $(RELEASE_DIR); \
		find . -type f ! -name MANIFEST.md ! -name SHA256SUMS -print0 \
			| LC_ALL=C sort -z \
			| while IFS= read -r -d '' f; do \
				printf '  - %s sha256=%s size=%s\n' "$${f#./}" \
					"$$(sha256sum "$$f" | awk '{print $$1}')" \
					"$$(wc -c <"$$f" | tr -d ' ')"; \
			done; \
		printf '\n'; \
		printf 'notes: |\n'; \
		printf '  Packages are bundled as distributed by upstream projects.\n'; \
		printf '  Provenance and pinned-source verification: see docs/releasing.md.\n'; \
	} > "$$MANIFEST"
	@cd $(RELEASE_DIR) && find . -type f ! -name SHA256SUMS -print0 | LC_ALL=C sort -z \
		| xargs -0 sha256sum > SHA256SUMS
	@echo "[RELEASE] Packing $(RELEASE_TARBALL) (reproducible)..."
	@tar --create --gzip --file "$(DIST_DIR)/$(RELEASE_TARBALL)" \
		--owner=0 --group=0 --numeric-owner \
		--mtime="@$(SOURCE_DATE_EPOCH)" --sort=name \
		--mode='u+rwX,go+rX,go-w' \
		-C $(DIST_DIR) $(RELEASE_STEM)
	@echo "[RELEASE] Done! Release built in $(DIST_DIR)/:"
	@ls -l $(DIST_DIR)/$(RELEASE_TARBALL) $(RELEASE_DIR)/MANIFEST.md $(RELEASE_DIR)/SHA256SUMS
	@printf '%s\n' '$(VERSION)' > $(DIST_DIR)/.last-version
	@rm -f $(INSTALLER_BIN) $(UNINSTALLER_BIN)

# Verify the last successful release unless VERSION was explicitly supplied.
verify-release:
	@version='$(VERSION)'; \
	if [ '$(origin VERSION)' = 'file' ] && [ -f $(DIST_DIR)/.last-version ]; then \
		version=$$(cat $(DIST_DIR)/.last-version); \
	fi; \
	release_dir="$(DIST_DIR)/bellum-installer-linux-$(GOARCH)-$$version"; \
	echo "[VERIFY] Checking checksums in $$release_dir..."; \
	(cd "$$release_dir" && sha256sum --check --strict SHA256SUMS) || exit 1; \
	tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
	tar -xzf "$(DIST_DIR)/bellum-installer-linux-$(GOARCH)-$$version.tar.gz" -C "$$tmp"; \
	diff -r "$$release_dir" "$$tmp/bellum-installer-linux-$(GOARCH)-$$version"

# Clean build artifacts
clean:
	@echo "[CLEAN] Removing build artifacts..."
	rm -f $(INSTALLER_BIN) $(UNINSTALLER_BIN) $(RELEASE_TARBALL)
	rm -rf $(DIST_DIR)
	@echo "[CLEAN] Done!"

# Help target
help:
	@echo "Bellum Linux Installer Go - Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build          - Build both installer and uninstaller binaries"
	@echo "  check          - Run the CI gate (gofmt, vet, test, module pinning)"
	@echo "  release        - Reproducible release archive + MANIFEST + SHA256SUMS"
	@echo "                   (override VERSION, e.g. make release VERSION=2.1.0)"
	@echo "  verify-release - sha256 --check the staged SHA256SUMS"
	@echo "  clean          - Remove all build artifacts"
	@echo "  help           - Show this help message"
