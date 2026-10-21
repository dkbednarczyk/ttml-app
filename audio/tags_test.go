package audio

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dhowden/tag"
)

func TestReadTags(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed; it is only used to create fixtures")
	}

	codecs := map[string]string{".flac": "flac", ".mp3": "libmp3lame", ".ogg": "libvorbis"}
	for ext, codec := range codecs {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "song"+ext)
			cmd := exec.CommandContext(context.Background(), "ffmpeg", "-v", "error", "-f", "lavfi",
				"-i", "sine=frequency=440:duration=0.2", "-c:a", codec,
				"-metadata", "title=Shine On", "-metadata", "artist=Pink Floyd", "-metadata", "album=Wish You Were Here",
				"-metadata", "ISRC=GBN9Y1100088", path)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Skipf("ffmpeg cannot write %s: %v: %s", ext, err, output)
			}

			want := Tags{Title: "Shine On", Artist: "Pink Floyd", Album: "Wish You Were Here", ISRC: "GBN9Y1100088"}
			if got := ReadTags(path); got != want {
				t.Fatalf("ReadTags() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestReadTagsWithoutTags(t *testing.T) {
	if got := ReadTags(writeTestWAV(t, int(sampleRate), 2, 10*time.Millisecond)); got != (Tags{}) {
		t.Fatalf("ReadTags() = %+v for an untagged file, want empty", got)
	}
	if got := ReadTags(filepath.Join(t.TempDir(), "missing.flac")); got != (Tags{}) {
		t.Fatalf("ReadTags() = %+v for a missing file, want empty", got)
	}
}

func TestReadISRC(t *testing.T) {
	tests := []struct {
		raw  map[string]any
		want string
	}{
		{map[string]any{"isrc": "GBN9Y1100088"}, "GBN9Y1100088"},     // Vorbis comment
		{map[string]any{"TSRC": "GBN9Y1100088\x00"}, "GBN9Y1100088"}, // ID3v2.3/2.4
		{map[string]any{"TRC": " GBN9Y1100088 "}, "GBN9Y1100088"},    // ID3v2.2
		{map[string]any{"TXXX_1": &tag.Comm{Description: "isrc", Text: "GBN9Y1100088"}}, "GBN9Y1100088"},
		{map[string]any{"TXXX": &tag.Comm{Description: "MusicBrainz Album Id", Text: "x"}}, ""},
		{map[string]any{}, ""},
	}
	for _, tt := range tests {
		if got := readISRC(tt.raw); got != tt.want {
			t.Errorf("readISRC(%v) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}
