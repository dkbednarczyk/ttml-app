// Package audio provides playback through the gopxl/beep audio library.
package audio

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

// sampleRate is the output rate. Files at other rates are resampled while they play.
const sampleRate beep.SampleRate = 48000

var (
	errNoAudio = errors.New("file contains no audio")
	// ErrUnsupportedFormat is returned by Open for a file type beep cannot decode.
	ErrUnsupportedFormat = errors.New("unsupported audio format")
)

type decodeFunc func(io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error)

func decoders() map[string]decodeFunc {
	return map[string]decodeFunc{
		".flac": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) { return flac.Decode(r) },
		".mp3":  mp3.Decode,
		".ogg":  vorbis.Decode,
		".wav":  func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) { return wav.Decode(r) },
	}
}

// SupportedExtensions lists the lower-case file extensions Open can decode.
func SupportedExtensions() []string {
	exts := make([]string, 0, len(decoders()))
	for ext := range decoders() {
		exts = append(exts, ext)
	}
	slices.Sort(exts)

	return exts
}

// Init opens the default output device. Call it once before playing anything.
// The 100 ms buffer covers a FLAC seek (up to ~50 ms), which holds the speaker lock.
func Init() error {
	if err := speaker.Init(sampleRate, sampleRate.N(time.Second/10)); err != nil {
		return fmt.Errorf("init audio output: %w", err)
	}

	return nil
}

// Close stops the output device.
func Close() {
	speaker.Close()
}

// Player plays one audio file, decoding it from disk as it plays. It starts
// paused at the beginning. Close it when done to release the file.
type Player struct {
	decoder beep.StreamSeekCloser
	rate    beep.SampleRate // the file's sample rate
	speed   float64         // playback speed; 1 is normal
	ctrl    *beep.Ctrl
}

// Open opens an audio file for playback.
func Open(path string) (*Player, error) {
	ext := strings.ToLower(filepath.Ext(path))
	decode, ok := decoders()[ext]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, ext)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open audio: %w", err)
	}
	decoder, format, err := decode(file)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("decode audio: %w", err), file.Close())
	}
	if format.SampleRate <= 0 {
		err = fmt.Errorf("%w: sample rate %d", ErrUnsupportedFormat, format.SampleRate)
	} else if decoder.Len() == 0 {
		err = errNoAudio
	}
	if err != nil {
		return nil, errors.Join(err, decoder.Close())
	}

	p := &Player{decoder: decoder, rate: format.SampleRate, speed: 1, ctrl: &beep.Ctrl{Paused: true}}
	p.ctrl.Streamer = p.output()

	return p, nil
}

// output is the decoder, resampled on the fly when the file's rate differs
// from the device's or the speed isn't 1.
func (p *Player) output() beep.Streamer {
	if ratio := p.ratio(); ratio != 1 {
		return beep.ResampleRatio(4, ratio, p.decoder)
	}

	return p.decoder
}

// ratio is how many file frames go into each output frame.
func (p *Player) ratio() float64 {
	return float64(p.rate) / float64(sampleRate) * p.speed
}

// SetSpeed sets the playback speed, 1 being normal. The pitch moves with it,
// as on a tape: 0.5 plays an octave lower. Position and Duration stay in song
// time, so marks taken at any speed land at the right place.
func (p *Player) SetSpeed(speed float64) {
	speaker.Lock()
	defer speaker.Unlock()

	p.speed = speed
	// Retune a running resampler rather than replacing it, which would drop
	// the audio it has already read ahead and skip a moment of the song.
	if r, ok := p.ctrl.Streamer.(*beep.Resampler); ok {
		r.SetRatio(p.ratio())
	} else {
		p.ctrl.Streamer = p.output()
	}
}

// Play starts playback at the current position, replacing anything else playing.
func (p *Player) Play() {
	speaker.Lock()
	p.ctrl.Paused = false
	speaker.Unlock()

	// The speaker drops a streamer once it drains, so add it again on every Play.
	speaker.Clear()
	speaker.Play(p.ctrl)
}

// Pause stops playback, keeping the position.
func (p *Player) Pause() {
	speaker.Lock()
	p.ctrl.Paused = true
	speaker.Unlock()
}

// Seek moves the playback position, clamped to the audio.
func (p *Player) Seek(position time.Duration) error {
	speaker.Lock()
	defer speaker.Unlock()

	if err := p.decoder.Seek(min(max(p.rate.N(position), 0), p.decoder.Len())); err != nil {
		return fmt.Errorf("seek audio: %w", err)
	}
	// A fresh resampler drops samples it buffered from before the seek.
	p.ctrl.Streamer = p.output()

	return nil
}

// Position returns the playback position.
func (p *Player) Position() time.Duration {
	speaker.Lock()
	defer speaker.Unlock()

	return p.rate.D(p.decoder.Position())
}

// Duration returns the length of the audio.
func (p *Player) Duration() time.Duration {
	return p.rate.D(p.decoder.Len())
}

// Close stops playback and releases the file.
func (p *Player) Close() error {
	speaker.Lock()
	p.ctrl.Streamer = nil // the speaker drops a Ctrl with no streamer
	speaker.Unlock()

	if err := p.decoder.Close(); err != nil {
		return fmt.Errorf("close audio: %w", err)
	}

	return nil
}
