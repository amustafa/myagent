# myagent — top-level tasks
# The installer TUI is a Go module under installer/.

INSTALLER_DIR := installer

.PHONY: run list build test

# Launch the installer TUI (symlink skills/commands/etc. into global or a project).
run:
	cd $(INSTALLER_DIR) && go run .

# Print discovered components without launching the TUI.
list:
	cd $(INSTALLER_DIR) && go run . -list

# Build the installer binary at installer/myagent-install.
build:
	cd $(INSTALLER_DIR) && go build -o myagent-install .

# Run the installer's tests.
test:
	cd $(INSTALLER_DIR) && go test ./...
