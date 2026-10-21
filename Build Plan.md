# TTML Lyric Sync App — Build Plan

Oct 19, 2026 · @Damian Bednarczyk

## Goal and scope

Build a cross-platform desktop editor that makes Apple-style TTML lyrics with line, word and syllable timing. The app makes an automatic first pass (Demucs vocal stem, then lyric-align). The user then corrects it with tap-to-sync, a seek bar and keyboard nudges.

**Targets:** Windows 10/11 and Linux (native Wayland only; X11 is not supported and not planned) first. macOS after the core works.

**In scope**

- Audio in through gopxl/beep: WAV, MP3, FLAC and Ogg Vorbis. No conversion step.
- Lyrics in from plain text, LRC, eLRC, SRT and TTML.
- Optional automatic "best guess" timing that runs in the background.
- Tap-to-sync and manual edit for lines, words and syllables.
- Export to TTML (Apple / AMLL profile), LRC, eLRC and SRT.

**Not in scope for v1**

- Delivery to Apple Music or Spotify. Only rights holders and distributors can submit lyrics.
- Pitch data (UltraStar, CDG karaoke).
- Model training or fine-tuning.
- Cloud processing. All processing runs on the local machine.

## Stack decision

**Decision: Go + Fyne for the app****.**** ****Python only inside a small worker process, because Demucs and lyric-align are Python.** Milestone M0 tests the hardest screen (playback clock + tap-sync) in Fyne before real work starts. No TypeScript, no web view.

Fyne weak points and how the plan handles them:

- **Custom drawing is limited.** The UI needs none: the seek bar is a standard slider and the lyrics are a normal list.
- **No real-time time-stretch library in Go.** Pre-render 0.5x and 0.75x copies with ffmpeg `atempo` in the background and switch buffers when speed changes. This also works in Rust, so neither stack needs real-time DSP.
- **Audio output needs cgo** (malgo, the miniaudio bindings). Fyne already needs cgo and a C compiler for its OpenGL driver, so this adds no new build step.

**Python worker rule:** keep it under about 300 lines. It only wraps Demucs and lyric-align and prints JSON lines. Type-check it with pyright (standard mode for now).

## Architecture

One Go process holds the UI and all logic; ffmpeg and the Python worker run as child processes. A crash or a cancel in a child process never takes down the editor, and the UI layer can be swapped without touching the logic.

&#91;embedded content: process and module layout · 1 app process, 2 child processes\]

Only `core` holds the project, and the UI changes it only through commands. The cache folder (decoded wav, stems, `segments.json`) sits beside the project file; the worker writes to it and `audio` reads the stems from it.

| Module | Job | Go (chosen) | Rust (fallback) |
| --- | --- | --- | --- |
| `ui` | Timeline, panels, key input | fyne.io/fyne/v2 | gpui + gpui-component |
| `core` | Model, commands, undo | Standard library | Standard library |
| `formats` | TTML, LRC, eLRC, SRT, TXT; validator | encoding/xml, encoding/json | quick-xml, serde |
| `audio` | PCM buffers, output, clock, peaks | github.com/gen2brain/malgo (miniaudio) | cpal |
| `jobs` | Run the worker; cancel; cache | os/exec, context | std::process |
| `worker/` | Wrap Demucs and lyric-align | Python 3.12 via uv: demucs, lyric-align\[asr\] | Same |

## Feature set

The MVP (milestones M0–M3) is a manual tap-sync editor that exports valid TTML, with no Python and no ML. Auto best-guess is gated to v1 (M4–M6) and starts only after the MVP ships, because the manual path is how every automatic error gets fixed. The list below takes the common features from Composer, AMLL TTML Tool and Lokal TTML Editor.

| Feature | Tier | Notes |
| --- | --- | --- |
| Open audio; seek bar with time readout | MVP | WAV, MP3, FLAC, Ogg Vorbis (what beep decodes) |
| Paste or import lyrics (TXT, LRC, eLRC, SRT, TTML) | MVP | Skip `[Verse]` markers and blank lines on import. TTML (line timing) and pasting plain text are in |
| Line tap-sync | MVP | One key marks line start, one marks line end |
| Word tap-sync | MVP | Key down = word start, key up = word end |
| Undo / redo for every edit | MVP | Command pattern in the core crate |
| Project file save, autosave, crash recovery | MVP | JSON project next to a cache folder |
| TTML export with pre-export validation | MVP | Save writes the TTML (line timing so far). Checks: untimed lines/words, end before start, starts out of order, child outside parent. Overlapping lines and lines with no text (instrumental breaks, exported as an empty `<p>`) are allowed |
| Playback speed 0.5x to 1.0x | MVP | Needed for fast rap verses. In, via beep's resampler, so the pitch moves with the speed (0.5x is an octave down); keeping the pitch would need a time-stretch (WSOLA) |
| Latency calibration and global tap offset | MVP | Tap-along test to a click track |
| Nudge word and line boundaries with keys; type exact times | v1 | Alt + arrows, 10 ms steps |
| Split word into syllables, join fragments without a space | v1 | Matches AMLL / Apple span rules |
| Edit lyric text and keep timing on matching words | v1 | Match by content first, then by position |
| Vocal stem playback toggle (mix / vocals / instrumental) | v1 | Free once Demucs has run; makes word boundaries much easier to hear |
| Auto best-guess (Demucs + lyric-align) in the background | v1 | Per-line confidence, unmatched lines stay untimed |
| Re-align after a lyric edit in seconds | v1 | Reuse cached ASR segments |
| Agents for duets, background vocals (x-bg), song parts | v1 | Apple TTML features |
| Karaoke-style live preview | v1 | Word wipe on the current line. A line-level Preview tab is in: playing lines lit, the rest grey, the view glides so the newest line (or a still-playing line before it) sits at the top |
| LRC, eLRC and SRT export | v1 |  |
| Fetch plain lyrics from LRCLIB | Later | Network call, optional |
| Romanization and translation lines | Later | TTML x-roman, x-translation |
| Snap boundaries to vocal onsets | Later | Onset detection on the vocal stem |
| Native ML backends (no Python) | Later | See risks |

## Auto best-guess pipeline

The pipeline is v1 scope (M4–M5); the MVP ships without it. It has five steps. Each step writes its result to a cache folder keyed by the SHA-256 of the source audio, so a rerun only redoes the steps whose input changed. A lyric edit reruns only step 4, which takes seconds.

1. **Decode (app).** The app decodes the source with beep and writes `<cache>/<hash>/mix.wav` with beep's `wav.Encode` for the worker.
2. **Separate (worker).** Demucs with `--two-stems=vocals -n htdemucs` writes `vocals.wav` and `no_vocals.wav`. Use the maintained build: the facebookresearch repo is archived, and the author's fork shipped v4.1.0 with lighter inference dependencies and models on Hugging Face ([source](https://aireiter.com/blog/demucs-stem-separation-models-flags-cost)). Offer `htdemucs_ft` as a "high quality" option; it gives better results but takes about 4x as long ([source](https://github.com/Ryan5453/demucs)).
3. **Transcribe (worker).** Run lyric-align on the vocal stem, not with `--separate`, because the app already separated it. Add `--dump-segments segments.json` to keep the Whisper output. This is the slow step.
4. **Align (worker).** `lyric-align --segments segments.json lyrics.txt -f json`. The JSON output holds scores and unmatched lines ([source](https://pypi.org/project/lyric-align/)).
5. **Import (app).** Matched lines get timing with origin `Auto(score)`. Unmatched lines stay untimed and get a red marker. The import never overwrites timing with origin `Manual`.

**How far to trust it.** On lyric-align's own test track, line starts from a Demucs vocal stem had a mean error of 0.50 s and matched 19 of 20 lines. From the full mix, only 8 of 20 matched, with a mean error of 1.59 s. Word timing inside a line comes from spreading the characters across the segment by proportion. So: treat auto line starts as a good draft, and treat auto word timing as a rough draft that the user tap-syncs or drags.

**Failure cases to show in the UI**

- **Slow, sustained singing:** the voice-activity filter drops it. Add a "Slow vocals" checkbox that passes `--no-vad`.
- **Repeated hooks:** when the ASR splits the repeats unevenly, a hook line can land on the wrong repetition. Flag every line whose text repeats elsewhere for review.
- **Gaps:** `--interpolate` fills unmatched lines but marks them as guessed. Offer it as an option and import those lines with origin `Interpolated`.
- **Model size:** `large-v3` is not always better than the default `medium` in lyric-align's tests. Keep `medium` as the default and make the model a setting.

**Worker protocol.** One worker process per job; cancel = kill the process. The app writes one JSON request to stdin. The worker writes JSON lines to stdout: `{"event":"progress","stage":"separate","pct":0.42}`, then `{"event":"done","result":{…}}` or `{"event":"error","message":"…"}`. The worker reads Demucs progress from its stderr and re-emits it as progress events. All stderr goes to a log file.

**Runtime setup.** Auto-sync is an optional download. On first use, the app gets `uv` (one static binary), then creates a managed environment with Python 3.12, `demucs` and `lyric-align[asr]`. The manual editor works with no Python installed.

**Hardware.** faster-whisper runs on CPU or NVIDIA CUDA only. Demucs runs on CUDA, on ROCm (Linux), or on CPU. On an AMD card such as your RX 9070 XT: Whisper runs on CPU on both OSes; Demucs can use ROCm on Nobara if the installed torch ROCm build supports RDNA4, and runs on CPU on Windows. Show the device in the job panel so slow runs are not a surprise.

## Audio engine and tap-to-sync

> The current minimal player uses gopxl/beep for decoding and output and decodes from the file as it plays, like a desktop music player. The custom clock design below is still the target for tap-to-sync; beep's streamer position is not accurate enough for taps.

Tap accuracy depends on one value: the true playback position at the moment of the key press. So the app owns its playback clock. Do not use a media player's position property; those update in coarse steps and do not include output latency.

**Engine design**

- **Decode while playing.** beep decodes from the file as it plays and resamples on the fly when the file's rate differs from the output rate, so opening a file is instant. A FLAC seek takes up to about 50 ms and holds the speaker lock; the 100 ms output buffer absorbs it. If tap-sync later needs instant seeks, decode the whole file into memory in the background (a 4-minute stereo song at 48 kHz is about 92 MB at float32).
- **Output.** malgo (Go) or cpal (Rust). The device callback copies frames from the buffer and updates an atomic frame counter plus the monotonic time of that callback.
- **Position at a key press** = last callback position + (now − last callback time) − output latency + user offset. Read "now" from the monotonic clock as the first line of the key handler.
- **Latency calibration.** Play a 120 BPM click track. The user taps 16 beats. The median error becomes the user offset, stored per output device.
- **Speed.** 0.5x, 0.75x and 1x through beep's resampler, changed live (a running resampler is retuned, not replaced, so nothing skips). The pitch moves with the speed. The player's position stays in song time, so a tap at any speed lands at the right song time. (The first plan pre-rendered pitch-kept copies with ffmpeg `atempo`; ffmpeg is no longer used.)
- **Stems.** Mix, vocals and instrumental buffers have the same length. Switching is instant at the same frame.

**Tap rules**

- Line mode: S marks the start of the next line; the open line ends at that time. The next line is the first untimed line after the last started one (pasted lyrics waiting to be tapped, shown with an accent-coloured number); with none waiting, S adds a new line at the end. E ends the open line without starting another (an instrumental break), and the next S starts a new line. Marks only count while the song plays. A new line never starts before the previous line's start (a tap that would, after seeking back, is ignored); it may start before the previous line ends.
- Lines may overlap on purpose (several singers, a held word running into the next line): an end may run past the next line's start. Starts stay in order: each line starts at or after the previous line's start.
- Typed times (click a time to edit it) use exactly `m:ss.mmm`, or `h:mm:ss.mmm` from one hour on. A time that can't be used is refused in place with the reason, and Esc or clicking away restores the old one: bad format, an end not after its start, a start out of order, or a time past the end of the song. Editing a time moves only that time.
- Lines can be inserted above or below any line with the buttons at the end of its row. An inserted line has no times until you type them; untimed lines are skipped by the start-order rule, and S still works on the last line that has a start.
- Word mode: key down = start of the current word, key up = its end, then move to the next word.
- Legato: if the next key down comes before the key up, the previous word ends at the new key down.
- A tap never changes a word with origin `Manual` that is outside the current pass. Each pass is one undo step per line.

**Default keys** (all rebindable)

| Key | Action |
| --- | --- |
| J (hold) | Word tap: press = word start, release = word end |
| Space | Play / pause |
| S | Line tap: end the open line and start the next |
| E | End the open line without starting another |
| Backspace | Undo the last tap and seek back 2 s |
| ← / → | Seek 2 s back / forward (with Shift: 0.2 s) |
| Alt + ← / → | Move the selected boundary 10 ms |
| \[ / \] | Speed down / up (0.5x, 0.75x, 1.0x) |
| V | Cycle the stem: mix, vocals, instrumental |
| Ctrl + Z / Ctrl + Shift + Z (or Ctrl + Y) | Undo / redo |
| Ctrl + S | Save |

Fyne gives separate key-down and key-up events through its `desktop.Keyable` interface, which word mode needs. Check this in M0 on both Windows and Wayland, because key-repeat behavior differs by platform.

## Data model and TTML profile

One in-memory model is the single source of truth. Every format is an import into it or an export from it, and every edit is a command object so undo works everywhere. The UI never changes the model directly.

```go
type Ms int64 // milliseconds from song start

type Origin uint8

const (
	OriginNone Origin = iota
	OriginManual       // tapped or dragged by the user
	OriginAuto         // from lyric-align; Score is set
	OriginInterpolated // gap filled by --interpolate
)

type Timing struct {
	Begin, End *Ms // nil = not set
	Origin     Origin
	Score      float32 // 0..1, Auto only
}

type Syllable struct {
	Text     string
	JoinNext bool // no space before the next fragment
	Timing
}

type Line struct {
	ID         string // stable; used for undo and itunes:key
	AgentID    string // "v1", "v2"
	Part       string // "Verse", "Chorus"; optional
	Main       []Syllable
	Background []Syllable // ttm:role="x-bg"
	Timing
}

type Project struct {
	Version int // file format version, for migrations
	Audio   AudioRef // path, SHA-256, duration
	Meta    Meta     // title, artists, album, platform IDs
	Agents  []Agent
	Lines   []Line
}

type Command interface {
	Do(p *Project)
	Undo(p *Project)
}
```

The project file is this struct as JSON (`.lyrproj`), with the cache folder beside it.

**TTML output (Apple / AMLL profile)**

```xml
<tt xmlns="http://www.w3.org/ns/ttml"
    xmlns:ttm="http://www.w3.org/ns/ttml#metadata"
    xmlns:itunes="http://music.apple.com/lyric-ttml-internal"
    itunes:timing="Word" xml:lang="en">
  <head>
    <metadata>
      <ttm:agent type="person" xml:id="v1"/>
      <ttm:agent type="person" xml:id="v2"/>
    </metadata>
  </head>
  <body dur="3:41.250">
    <div begin="0:12.340" end="0:45.100" itunes:songPart="Verse">
      <p begin="0:12.340" end="0:15.900" ttm:agent="v1" itunes:key="L1">
        <span begin="0:12.340" end="0:12.700">Hello</span> <span begin="0:12.700" end="0:13.200">world</span>
        <span ttm:role="x-bg"><span begin="0:15.100" end="0:15.900">(ooh)</span></span>
      </p>
    </div>
  </body>
</tt>
```

Export rules:

- Import TTML replaces the lines (asking first if there are unsaved changes). Each `<p>` becomes a line with its times and text; times may be clock time, Apple's `m:ss.mmm`, plain seconds or offsets (`12.5s`, `12500ms`). Word timing, background vocals, translations, several singers and song parts are left out for now and named in a dialog; Save then asks for a new file instead of overwriting the original. Without audio, the editor shows the lines but no playback or mark controls; loading audio afterwards keeps the lines, while replacing already loaded audio starts a new song with no lines.
- Save writes the TTML file: the first save asks where (suggesting `<audio name>.ttml` beside the audio), later saves overwrite it with a temp-file-then-rename write. A project with problems is not written; the problems are listed by line. `xml:lang` is fixed to `en` for now, and `itunes:key` numbers lines in order at save time.

- Line-only songs: `itunes:timing="Line"`, and each `<p>` holds plain text.
- A space between words is a text node between `<span>`s. Joined syllables have no text node between them.
- Every child `begin`/`end` is inside its parent's range. The pre-export validator checks this.
- Write times as TTML clock time, `hh:mm:ss.mmm` (strict TTML requires the hours). The editor shows and accepts the shorter `m:ss.mmm` (`h:mm:ss.mmm` past one hour). On import, also accept offset times such as `12.345s` and `12345ms`.
- Metadata is written as AMLL `amll:meta` entries, copied from files in the AMLL TTML DB: `xmlns:amll="http://www.example.com/ns/amll"` (a placeholder URL AMLL uses as is) and `<amll:meta key="…" value="…"/>` in `head/metadata`, with keys such as `musicName`, `artists`, `album`, `isrc`, platform IDs (`ncmMusicId`, `qqMusicId`, `spotifyId`, `appleMusicId`) and `ttmlAuthorGithub`; keys may repeat. Imported entries are kept as they are; the audio's tags (title, artist, album, ISRC) fill in keys the lyrics lack. AMLL files also write `<div xmlns="">`, which takes the lines out of the TTML namespace, so the reader accepts TTML elements with no namespace.
- When lyrics and audio are both open, the editor warns (without blocking) if they look like different recordings: a line ends after the audio, the TTML's `body dur` differs from the audio length by more than 2 s, or the TTML's ISRCs don't include the audio's.
- Golden tests: lyric-align's TTML output passes the AMLL reference parser and the AMLL DB checker ([source](https://pypi.org/project/lyric-align/)), so use a few of its outputs as import fixtures. Optionally run the AMLL parser on your exports in CI only (Node in CI, not in the app).

## Milestones

Eight milestones, each small enough for one to three agent sessions. M0 is a go / no-go gate for Fyne. M0–M3 make the MVP: a manual editor with no ML code. M4–M6 make v1 and add auto best-guess; start them only after the MVP ships. Tick each one when its acceptance criteria pass.

- [ ] **M0: Fyne spike (gate).** One window: open a file and play it with beep, show a seek bar that follows playback and seeks on click, log J key-down and key-up times, run a 16-tap click calibration.
  - Done when: the seek bar tracks playback and a click seeks with no audible glitch; key-down and key-up arrive as separate events on Windows and on Nobara (Wayland); the calibration prints the mean and spread of 16 taps.
  - If it fails: start M0 again in Rust + GPUI before you write M1.
- [ ] **M1: Core, formats, CLI.** Model, commands with undo, import and export for TXT, LRC, eLRC, SRT and TTML, the validator, and the `lyrsync` CLI.
  - Done when: round-trip tests pass for every format; the lyric-align TTML fixtures import; each validator rule has a failing fixture that it catches.
- [ ] **M2: Editor MVP.** Lyrics panel, line tap-sync, word tap-sync, undo and redo, project save, autosave, TTML export with validation.
  - Done when: you can word-sync the *Amazing Grace* fixture from start to end at 1.0x, and the export passes the validator.
- [ ] **M3: Timing edits.** Nudge boundaries with keys, type exact times, split and join syllables, edit lyric text with timing kept, 0.5x and 0.75x speed from pre-rendered copies, calibration screen.
  - Done when: fixing a typo keeps the timing of every other word; a speed change keeps the playhead at the same song time.
- [ ] **M4: Worker and jobs.** `uv` bootstrap, the JSON-lines worker, Demucs separation with progress and cancel, the cache, the stem toggle.
  - Done when: a second run on the same file skips separation; cancel kills the worker within 1 second; the vocals stem plays in sync with the mix.
- [ ] **M5: Auto best-guess.** lyric-align transcribe and align, import with origin and score shown by color, unmatched and repeated-line markers, fast re-align after a lyric edit, "accept" and "clear auto" per line.
  - Done when: on the fixture, matched lines land within about 0.5 s of your manual sync, and no `Manual` timing changes after a re-run.
- [ ] **M6: Apple features and polish.** Agents (duets), background vocals, song parts, karaoke preview, LRC and SRT export, AMLL metadata fields.
  - Done when: a duet fixture with background vocals round-trips through TTML with no change.
- [ ] **M7: Packaging.** Windows build with an LGPL ffmpeg, Linux AppImage or Flatpak, settings screen, crash recovery. macOS after this.
  - Done when: a clean Windows VM and a clean Nobara install can open, sync and export with no manual setup for the manual editor.

## How to vibe code it

AI agents do best on pure logic with tests, and worst on GUI APIs they half-remember. So: give the agent one milestone per session, a written contract, tests it must pass, and a way to check behavior without the UI.

**Repo contract (`AGENTS.md` at the root)**

- The package layout and the one dependency rule: `ui/` may import `core/`; `core/`, `formats/`, `audio/` and `jobs/` never import Fyne.
- The commands that must pass before a commit: `go build ./...`, `go vet ./...`, `go test ./...`, `staticcheck ./...`. For the worker: `pyright` and `pytest`.
- Pin exact versions in `go.mod`. When unsure of a Fyne API, read its source in the module cache (`go env GOMODCACHE`) before writing code. Do not guess.
- Concurrency rules: the audio callback never allocates, locks or logs. UI changes from other goroutines go through `fyne.Do`.
- File rules: every save is atomic (write a temp file, then rename).
- No new dependency without asking.

**Make it checkable without the UI**

- Build a headless CLI in M1: `lyrsync import`, `lyrsync validate`, `lyrsync export`, later `lyrsync align`. The agent can prove behavior with it, and you can script batch jobs.
- Add a `--demo <fixture>` flag that opens the UI on a fixture project, so you can check a screen in seconds.

**Tests that carry the weight**

- Format round trips: import → export → import gives the same model, for every format.
- Undo property test: apply 500 random commands, undo all, compare to the start state.
- Clock math: tap position from a fake callback time and latency.
- Validator: one fixture per rule (overlap, untimed word, child outside parent, end before start).

**Fixtures**

- The public-domain *Amazing Grace* recording and lyrics that lyric-align uses in its own example ([source](https://pypi.org/project/lyric-align/)).
- A generated 120 BPM click track for latency tests.
- Small hand-written TTML files: a duet, background vocals, joined syllables, line-only timing.

**Session prompt template**

```text
Milestone M2: line tap-sync.
Read AGENTS.md and docs/plan.md (sections: Data model, Audio engine).
Task: <one paragraph>
You may change: core/, ui/timeline/. Do not change: formats/.
Done when:
- go test ./... passes, including the new tests listed below
- <acceptance criteria copied from the milestone list>
At the end: list what you changed and anything you were unsure about.
```

## Risks and open questions

The main risk that can change the plan is the Python ML stack, so it stays optional. The rest have known fixes.

| Risk | Effect | Mitigation |
| --- | --- | --- |
| Tap latency differs per device (Bluetooth is worst) | Every tap is early or late by a fixed amount | Per-device calibration; warn when the output is Bluetooth |
| Key-up and key-repeat behave differently on Wayland and Windows | Word mode ends words at the wrong time | Test in M0; ignore repeated key-down events |
| Torch install is large and can fail | Auto-sync does not start | Optional download through `uv`; a "repair environment" button; manual editor never needs Python |
| No CUDA on AMD cards | Whisper runs on CPU; minutes per song | Accept for v1. Later: whisper.cpp with Vulkan, fed to lyric-align through `--segments`, which accepts any Whisper output with word timings |
| Demucs is in maintenance mode; lyric-align has one maintainer and is beta | Fixes come slowly or stop | Pin versions. The worker interface and the segments JSON are the contract, so either tool can be swapped |
| ffmpeg licensing and bundling | Legal or install problems on release | Run ffmpeg as a separate process; ship an LGPL build on Windows; use the system ffmpeg on Linux |
| Repeated hooks land on the wrong repetition | Wrong auto timing that looks right | Flag repeated lines for review |
| CJK text rendering in Fyne | Broken glyphs for Japanese, Chinese, Korean lyrics | A Fyne 2.7 bug broke Korean rendering on Windows ([source](https://pkg.go.dev/github.com/k-atusa/USAG-Lib-macro/code)); test CJK fixtures in M0 if you need them |

**Open questions**

- [ ] App name.
- [ ] License for the app (MIT, or GPL if a GPL dependency is added later).
- [ ] Personal tool only, or a public release? This decides code signing and ffmpeg bundling.
- [ ] Is CJK lyric support needed at launch?
