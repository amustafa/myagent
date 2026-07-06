# myagent — top-level tasks
# The installer TUI is a Go module under installer/.

INSTALLER_DIR := installer
REPO          := $(abspath .)
BINDIR        ?= $(HOME)/.local/bin

.PHONY: run list status install build test ci

# Launch the installer TUI (symlink skills/commands/etc. into global or a project).
run:
	cd $(INSTALLER_DIR) && go run .

# Print discovered components without launching the TUI.
list:
	cd $(INSTALLER_DIR) && go run . -list

# Print a read-only report of install state across environments.
status:
	cd $(INSTALLER_DIR) && go run . -status

# Install a `myagent` binary onto your PATH. It bakes in this repo as the
# default source, so you can run `myagent` from anywhere to install FROM here.
install:
	@mkdir -p $(BINDIR)
	cd $(INSTALLER_DIR) && go build -ldflags "-X main.defaultSource=$(REPO)" -o $(BINDIR)/myagent .
	@echo "Installed myagent -> $(BINDIR)/myagent (source: $(REPO))"
	@echo "Ensure $(BINDIR) is on your PATH, then run: myagent"

# Build the installer binary at installer/myagent-install (dev artifact).
build:
	cd $(INSTALLER_DIR) && go build -o myagent-install .

# Run the installer's tests.
test:
	cd $(INSTALLER_DIR) && go test ./...

# What CI runs: formatting check, build, vet, tests.
ci:
	cd $(INSTALLER_DIR) && \
	  test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }
	cd $(INSTALLER_DIR) && go build ./... && go vet ./... && go test ./...
