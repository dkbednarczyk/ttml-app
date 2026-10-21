# Build script for the TTML lyric sync app. Run `just` to list recipes.
# `just check` is the gate every commit must pass (see AGENTS.md).

[windows]
set shell := ["pwsh.exe", "-NoLogo", "-NoProfile", "-Command"]

export CGO_ENABLED := "1"

tags := if os() == "linux" { "wayland" } else { "" }
tool := "go tool -modfile=tools.mod"
py := "uv run --directory worker --locked"
has_py_tests := path_exists(justfile_directory() / "worker" / "tests")

[private]
default:
    @just --list --unsorted

# Show missing tools and Fyne system libraries
doctor:
    #!/usr/bin/env bash
    set -uo pipefail
    missing=0
    for cmd in go uv gcc git; do
        command -v "$cmd" >/dev/null || { echo "missing: $cmd"; missing=1; }
    done
    if [[ "$(uname -s)" == Linux ]]; then
        command -v pkg-config >/dev/null || { echo "missing: pkg-config"; missing=1; }
        for lib in alsa gl egl wayland-client wayland-cursor wayland-egl wayland-protocols xkbcommon; do
            pkg-config --exists "$lib" || { echo "missing: $lib"; missing=1; }
        done
    fi
    exit $missing

# Install Fyne's C build dependencies on Fedora/Nobara (uses sudo)
[linux]
deps-fedora:
    sudo dnf install -y \
        alsa-lib-devel gcc pkgconf-pkg-config mesa-libGL-devel mesa-libEGL-devel \
        wayland-devel wayland-protocols-devel libxkbcommon-devel

# Download Go modules, build the pinned dev tools, and create the worker venv
setup:
    go mod download
    go mod download -modfile=tools.mod
    {{ tool }} golangci-lint version
    {{ tool }} govulncheck -version
    uv sync --directory worker --locked

# Add the heavy ML stack to the worker venv
worker-ml:
    uv sync --directory worker --locked --group ml

# Format all Go and Python code in place
fmt:
    {{ tool }} golangci-lint fmt
    {{ py }} ruff format
    {{ py }} ruff check --fix

# Lint everything
lint:
    {{ tool }} golangci-lint run --build-tags '{{ tags }}' ./...
    {{ py }} ruff check
    {{ py }} ruff format --check
    {{ py }} pyright

# Run all tests
test:
    go test -race -shuffle=on -tags '{{ tags }}' ./...
    {{ if has_py_tests == "true" { py + " pytest" } else { "echo 'skipped: no worker tests yet'" } }}

# Compile every Go package
build:
    go build -tags '{{ tags }}' ./...

# Launch the graphical app
run:
    go run -tags '{{ tags }}' ./cmd/app

# Scan Go dependencies for known vulnerabilities
vuln:
    {{ tool }} govulncheck -tags '{{ tags }}' ./...

# Fail if go.mod, tools.mod or uv.lock are out of date
check-deps:
    go mod tidy -diff
    go mod tidy -diff -modfile=tools.mod
    go mod verify
    uv lock --directory worker --check

# Pre-commit gate: deps, lint, build, test
check: check-deps lint build test

# Tidy Go modules and refresh the worker lockfile
tidy:
    go mod tidy
    go mod tidy -modfile=tools.mod
    uv lock --directory worker
