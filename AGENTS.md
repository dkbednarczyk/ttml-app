# AGENTS.md

Desktop editor for Apple-style TTML lyrics. Today it syncs whole lines; word
and syllable timing are a later goal, and automatic sync (ML) after that.

## Stack

- Go (cgo on) + Fyne. One process holds the UI and all logic.
- Audio: gopxl/beep (oto output). It plays what beep decodes: WAV, MP3, FLAC
  and Ogg Vorbis, decoded from disk as it plays. Speed (0.5x, 0.75x, 1x) goes
  through beep's resampler, so the pitch moves with it.
- Track tags with dhowden/tag, album art with golang.org/x/image, desktop
  settings (cursor theme) with rymdport/portal.
- Targets: Linux, native Wayland only (no X11 or XWayland), and macOS. The
  justfile sets the `wayland` build tag on Linux; for plain `go` commands
  there, pass `-tags wayland` yourself.

## Setup and commands

```sh
just doctor        # what is missing
just deps-fedora   # Fyne and ALSA C libraries on Fedora/Nobara (sudo)
just setup         # Go modules and the pinned dev tools
just run           # launch the app
```

`just check` must pass before every commit: `go mod tidy -diff` and
`go mod verify`, golangci-lint (with gofumpt and goimports), `go build ./...`
and `go test -race -shuffle=on ./...`. `just fmt` formats the code. Dev tools
live in `tools.mod`, separate from `go.mod`; run them with
`go tool -modfile=tools.mod <tool>`.

When unsure of a Fyne (or any) API, read its source in the module cache
(`go env GOMODCACHE`) rather than guessing.

## Layout

```
cmd/app/   GUI entry point
core/      the Project (lines, timing, metadata) and the commands that edit it; undo
formats/   TTML read/write, plain-text lyrics, validator and warnings, atomic file writes
audio/     playback, speed, track tags, cover art
ui/        Fyne screens
example/   local sample songs (git-ignored; tests that need them skip)
```

## Rules

- The UI never changes the model directly: every edit is a `core` command, so
  undo works everywhere.
- `core/`, `formats/` and `audio/` never import Fyne, and nothing but `cmd/`
  imports `ui/`. depguard enforces this (`.golangci.yml`).
- UI changes from other goroutines go through `fyne.Do`.
