# AGENTS.md

Desktop editor for Apple-style TTML lyrics (line, word and syllable timing).
The full design is in `Build Plan.md`; read the sections your milestone names.

## Stack

- **App:** Go (cgo on) + Fyne. One process holds the UI and all logic.
- **Audio:** decoding and playback through gopxl/beep (oto). The app opens only
  the formats beep decodes (WAV, MP3, FLAC, Ogg Vorbis); there is no ffmpeg
  conversion step.
- **Worker:** Python 3.12 in `worker/`, managed by uv. It only wraps Demucs and
  lyric-align and prints JSON lines. Keep it under ~300 lines. The manual
  editor must never need Python.

## Setup

```sh
just doctor        # what is missing
just deps-fedora   # Fyne and ALSA C libraries on Fedora/Nobara (sudo)
just setup         # Go modules, pinned dev tools, worker venv (dev tools only)
just worker-ml     # optional: torch, Demucs, faster-whisper (several GB; needed from M4)
```

Required on PATH: `go` (go.mod pins the toolchain; Go downloads it if yours is
older), `uv`, `just`, `gcc`, `git`. On Windows use a
MinGW-w64 gcc (e.g. MSYS2) for cgo, and Git Bash for `just doctor`.

On Linux the app is native Wayland only. X11 (and XWayland) is not supported
and support is not planned. The justfile always sets the `wayland` build tag on
Linux; for plain `go` commands outside just, pass `-tags wayland` yourself.

## Commands

`just check` must pass before every commit. It runs:

| Step | What it runs |
| --- | --- |
| `check-deps` | `go mod tidy -diff` (app and tools), `go mod verify`, `uv lock --check` |
| `lint-go` | golangci-lint: the standard set (errcheck, govet, ineffassign, staticcheck, unused) plus depguard, errorlint, misspell, nolintlint; gofumpt and goimports |
| `lint-py` | `ruff check` (E, F, W, I, B, UP, SIM, RUF), `ruff format --check`, `pyright` standard mode |
| `build` | `go build ./...` |
| `test` | `go test -race -shuffle=on ./...`, `pytest` |

`just fmt` formats Go and Python in place. `just vuln` runs govulncheck.
Go steps print "skipped" until the first package exists.

## Layout and the dependency rule

```
cmd/lyrsync/   headless CLI: import, validate, export, later align
cmd/<app>/     GUI entry point (name not decided yet); supports --demo <fixture>
core/          model, commands, undo; the only owner of the Project
formats/       TXT, LRC, eLRC, SRT, TTML import/export; validator
audio/         decoded buffers, playback, clock, peaks
jobs/          worker child processes, cancel, cache
ui/            Fyne screens; changes the model only through core commands
worker/        Python worker (own pyproject.toml, uv.lock)
testdata/      fixtures
```

- `ui/` may import `core/`. `core/`, `formats/`, `audio/` and `jobs/` never
  import Fyne. Nothing but `cmd/` imports `ui/`. depguard enforces both rules
  (`.golangci.yml`); do not edit those rules to get a build through.
- The UI never changes the model directly: every edit is a `core` command, so
  undo works everywhere.

## Dependencies

- No new dependency without asking. Versions are exact, everywhere.
- Approved for the app (add them in M0, at these versions unless asked):
  `fyne.io/fyne/v2 v2.8.2`, `github.com/gopxl/beep/v2 v2.1.1`,
  `github.com/dhowden/tag v0.0.0-20240417053706-3d75831295e8` (track tags; the
  module has no tagged releases), `github.com/rymdport/portal v0.4.2` (desktop
  settings such as the cursor theme; Fyne already depends on it, keep the
  versions in step), `golang.org/x/image v0.24.0` (WebP and BMP album art; also
  pulled in by Fyne, keep in step).
- Go dev tools live in `tools.mod` (separate from `go.mod` so they never move
  app dependency versions). Run them with `go tool -modfile=tools.mod <tool>`.
- Worker: dev tools in the `dev` group, Demucs and lyric-align in the `ml`
  group of `worker/pyproject.toml`. Change versions there, then `just tidy`.
- When unsure of a Fyne (or any) API, read its source in the module cache
  (`go env GOMODCACHE`) before writing code. Do not guess.

## Lint policy

- The Go and Python lint sets are kept light on purpose while the project is a
  proof of concept; add linters back as the code settles. Do not disable linters or
  loosen `.golangci.yml`, `[tool.ruff]` or `[tool.pyright]` just to make code
  pass. Fix the code.
- A suppression must name the rule and should say why:
  `//nolint:errcheck // best-effort cleanup`. nolintlint rejects a bare
  `//nolint`. In Python use
  `# noqa: RULE` and `# pyright: ignore[rule]`, never a bare `# type: ignore`.

## Runtime rules

- Playback goes through `audio.Player` (beep). Do not hand-write a device callback;
  if one is ever added, it never allocates, locks or logs.
- UI changes from other goroutines go through `fyne.Do`.
- Every file save is atomic: write a temp file in the same directory, then rename.
- Child processes (the worker) are cancelled by killing the process; a
  crash in one must never take down the editor.
