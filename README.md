# TTML Lyric Sync

A desktop editor for Apple-style TTML lyrics. Load a song, paste or import
its lyrics, and tap along to time each line. The result is a line-timed TTML
file in the Apple / [AMLL](https://github.com/amll-dev/amll-ttml-db) profile.

Today the editor times whole lines. Word and syllable timing are planned, and
automatic syncing after that.

Runs on Linux (native Wayland only, no X11 or XWayland) and macOS.

## Features

- Plays WAV, MP3, FLAC and Ogg Vorbis at 0.5x, 0.75x or 1x speed. Slower
  speeds also lower the pitch.
- Paste plain lyrics from anywhere. Blank lines and section headings like
  `[Chorus]` are skipped. Parentheses at the end of a line, as in
  `Let it go (let it burn)`, become a background vocal.
- Tap to sync: each tap starts the next line and ends the one before it.
- Edit any time by hand (`m:ss.mmm`), insert or delete lines, and clear the
  times from a line onward to re-tap the rest.
- Background vocals (`x-bg` spans), with their own times if they need them.
- Several singers, named or not, plus lines that everyone sings.
- A Preview tab that highlights the current line as the song plays. Click a
  line to jump to it.
- Import existing TTML files. If the lyrics' length or ISRC doesn't match the
  loaded audio, the editor warns you.
- Saving checks the lyrics first. The status icon lists anything that would
  block a save, plus warnings for things that look like mistakes. Song
  metadata is filled in from the audio file's tags.
- Undo and redo for every edit.

## Using it

1. **Load audio**, then **Paste lyrics** or **Import TTML**. You can do these
   in either order.
2. Press **Space** to play. Press **S** as each line begins. Press **E** to
   end a line early, before an instrumental break.
3. Fix any times by hand. Double-click a lyric to jump the song to it.
4. **Save** (Ctrl+S) writes the `.ttml` file.

| Key                     | Action                         |
| ----------------------- | ------------------------------ |
| Space                   | Play / pause                   |
| S                       | Start the next line            |
| E                       | End the current line           |
| `[` / `]`               | Slower / faster                |
| Ctrl+S                  | Save                           |
| Ctrl+Z                  | Undo                           |
| Ctrl+Y, Ctrl+Shift+Z    | Redo                           |

On macOS, use Cmd instead of Ctrl. Space, S, E and the brackets work only
when no text box has focus, so you can type lyrics without triggering them.

Import keeps line times, singers, background vocals and AMLL metadata. Word
timing, translations and song parts are not supported yet. If an imported
file has any of them, the editor tells you what it left out. It also won't
let Save overwrite that file, so you don't lose them.

## Building

You need Go (see `go.mod` for the version), a C compiler, and
[just](https://github.com/casey/just). On Linux you also need the ALSA,
OpenGL/EGL, Wayland and xkbcommon development libraries.

```sh
just doctor        # list anything missing
just setup         # download Go modules and the dev tools
just run           # launch the app
```

On Linux, the justfile builds with the `wayland` tag. If you run `go`
commands yourself, pass `-tags wayland`.

## Development

```sh
just fmt     # format the code
just check   # what must pass before every commit
```

```
cmd/app/   GUI entry point
core/      the project (lines, timing, metadata), the commands that edit it, and undo
formats/   TTML read/write, plain-text lyrics, validation
audio/     playback, speed, track tags, cover art
ui/        Fyne screens
example/   local sample songs (git-ignored; tests that need them skip)
```

Every edit goes through a `core` command, so everything can be undone. The
`core`, `formats` and `audio` packages never import Fyne. See
[AGENTS.md](AGENTS.md) for the full rules.
