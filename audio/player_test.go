package audio

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/generators"
	"github.com/gopxl/beep/v2/wav"
)

func TestPlayerSeekAndPosition(t *testing.T) {
	path := writeTestWAV(t, int(sampleRate), 2, 100*time.Millisecond)
	player := openTest(t, path)
	if got := player.Duration(); got != 100*time.Millisecond {
		t.Fatalf("Duration() = %v, want %v", got, 100*time.Millisecond)
	}
	if got := player.Position(); got != 0 {
		t.Fatalf("Position() = %v, want 0", got)
	}

	if err := player.Seek(50 * time.Millisecond); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}
	if got := player.Position(); got != 50*time.Millisecond {
		t.Fatalf("Position() = %v, want %v", got, 50*time.Millisecond)
	}

	if err := player.Seek(time.Hour); err != nil {
		t.Fatalf("Seek() past the end error: %v", err)
	}
	if got := player.Position(); got != player.Duration() {
		t.Fatalf("Position() after seeking past the end = %v, want %v", got, player.Duration())
	}
}

func TestPlayerResamplesToOutputRate(t *testing.T) {
	path := writeTestWAV(t, 44100, 1, 100*time.Millisecond)
	player := openTest(t, path)
	if got := player.Duration(); got != 100*time.Millisecond {
		t.Fatalf("Duration() = %v, want %v", got, 100*time.Millisecond)
	}
	if err := player.Seek(50 * time.Millisecond); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}

	// The remaining 50 ms comes out at the output rate.
	player.ctrl.Paused = false
	frames := 0
	var samples [512][2]float64
	for {
		n, ok := player.ctrl.Stream(samples[:])
		frames += n
		if !ok {
			break
		}
	}
	if want := sampleRate.N(50 * time.Millisecond); frames < want-10 || frames > want+10 {
		t.Fatalf("streamed %d frames, want about %d", frames, want)
	}
}

func TestPlayerSpeed(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate int
		speed      float64
	}{
		{"half speed at the output rate", int(sampleRate), 0.5},
		{"three-quarter speed, resampled file", 44100, 0.75},
		{"back to normal", 44100, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := openTest(t, writeTestWAV(t, tt.sampleRate, 2, 100*time.Millisecond))
			player.SetSpeed(tt.speed)

			// The song's 100 ms take 100 ms / speed of output.
			player.ctrl.Paused = false
			frames := 0
			var samples [512][2]float64
			for {
				n, ok := player.ctrl.Stream(samples[:])
				frames += n
				if !ok {
					break
				}
			}
			want := int(float64(sampleRate.N(100*time.Millisecond)) / tt.speed)
			if frames < want-20 || frames > want+20 {
				t.Fatalf("streamed %d frames, want about %d", frames, want)
			}
			// Position stays in song time.
			if got := player.Position(); got != player.Duration() {
				t.Errorf("Position() = %v at the end, want %v", got, player.Duration())
			}
		})
	}
}

func TestPlayerSpeedChangeKeepsPlace(t *testing.T) {
	player := openTest(t, writeTestWAV(t, 44100, 2, 200*time.Millisecond))
	player.ctrl.Paused = false
	var samples [512][2]float64
	player.ctrl.Stream(samples[:]) // play a little
	before := player.Position()

	player.SetSpeed(0.5)
	if _, ok := player.ctrl.Streamer.(*beep.Resampler); !ok {
		t.Fatal("the running resampler was replaced, not retuned")
	}
	if got := player.Position(); got != before {
		t.Fatalf("changing speed moved the song from %v to %v", before, got)
	}
}

func TestPlayerDecodesSupportedFormats(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed; it is only used to create fixtures")
	}

	codecs := map[string]string{".flac": "flac", ".mp3": "libmp3lame", ".ogg": "libvorbis", ".wav": "pcm_s16le"}
	for _, ext := range SupportedExtensions() {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "song"+ext)
			cmd := exec.CommandContext(context.Background(), "ffmpeg", "-v", "error", "-f", "lavfi",
				"-i", "sine=frequency=440:duration=0.5", "-ar", "44100", "-c:a", codecs[ext], path)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Skipf("ffmpeg cannot write %s: %v: %s", ext, err, output)
			}

			player := openTest(t, path)
			if got := player.Duration(); got < 400*time.Millisecond {
				t.Fatalf("Duration() = %v, want about 500ms", got)
			}
		})
	}
}

func TestOpenRejectsUnsupportedFormat(t *testing.T) {
	_, err := Open(filepath.Join(t.TempDir(), "song.m4a"))
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("Open() error = %v, want %v", err, ErrUnsupportedFormat)
	}
}

func TestOpenRejectsMissingFile(t *testing.T) {
	_, err := Open(filepath.Join(t.TempDir(), "missing.wav"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open() error = %v, want fs.ErrNotExist", err)
	}
}

func TestOpenRejectsEmptyAudio(t *testing.T) {
	_, err := Open(writeTestWAV(t, int(sampleRate), 2, 0))
	if !errors.Is(err, errNoAudio) {
		t.Fatalf("Open() error = %v, want %v", err, errNoAudio)
	}
}

func writeTestWAV(t *testing.T, sampleRate, channels int, duration time.Duration) string {
	t.Helper()

	format := beep.Format{SampleRate: beep.SampleRate(sampleRate), NumChannels: channels, Precision: 2}
	path := filepath.Join(t.TempDir(), "test.wav")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test wav: %v", err)
	}
	if err := wav.Encode(file, generators.Silence(format.SampleRate.N(duration)), format); err != nil {
		t.Fatalf("encode test wav: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close test wav: %v", err)
	}

	return path
}

func openTest(t *testing.T, path string) *Player {
	t.Helper()

	player, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%q) error: %v", path, err)
	}
	t.Cleanup(func() {
		if err := player.Close(); err != nil {
			t.Errorf("Close() error: %v", err)
		}
	})

	return player
}
