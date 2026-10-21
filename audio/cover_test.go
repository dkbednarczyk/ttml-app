package audio

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/image/bmp"
)

// writeImage writes a solid w×h PNG; the colour tells the files apart.
func writeImage(t *testing.T, path string, w, h int, c color.Color) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFindCoverEmbedded(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed; it is only used to create fixtures")
	}
	dir := t.TempDir()
	art := filepath.Join(t.TempDir(), "art.png") // not beside the song
	writeImage(t, art, 300, 300, color.RGBA{R: 200, A: 255})
	song := filepath.Join(dir, "song.flac")
	cmd := exec.CommandContext(context.Background(), "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=duration=0.2",
		"-i", art, "-map", "0:a", "-map", "1:v", "-c:a", "flac", "-c:v", "png", "-disposition:v", "attached_pic", song)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot embed art: %v: %s", err, output)
	}
	writeImage(t, filepath.Join(dir, "cover.png"), 300, 300, color.RGBA{B: 200, A: 255}) // embedded art wins

	cover := FindCover(song, "")
	if cover == nil || cover.From != "embedded" {
		t.Fatalf("FindCover() = %+v, want the embedded picture", cover)
	}
	if r, _, _, _ := cover.Image.At(10, 10).RGBA(); r>>8 != 200 {
		t.Error("returned image is not the embedded one")
	}
}

func TestFindCoverFiles(t *testing.T) {
	type file struct {
		name string
		w, h int
	}
	tests := []struct {
		name  string
		files []file
		album string
		want  string // "" for no cover
	}{
		{"cover over folder", []file{{"folder.jpg", 200, 200}, {"cover.png", 200, 200}}, "", "cover.png"},
		{"any case", []file{{"Cover.PNG", 200, 200}}, "", "Cover.PNG"},
		{"not square is skipped", []file{{"cover.png", 1000, 300}, {"folder.png", 500, 500}}, "", "folder.png"},
		{"nearly square is fine", []file{{"front.png", 600, 500}}, "", "front.png"},
		{"named after the album", []file{{"Amarionette.png", 300, 300}, {"booklet.png", 300, 300}}, "Amarionette", "Amarionette.png"},
		{"cover in the name", []file{{"Amarionette - Cover (Front).png", 300, 300}, {"back.png", 300, 300}}, "", "Amarionette - Cover (Front).png"},
		{"the only image", []file{{"scan01.png", 300, 300}}, "", "scan01.png"},
		{"two unrelated images", []file{{"scan01.png", 300, 300}, {"scan02.png", 300, 300}}, "", ""},
		{"too small", []file{{"cover.png", 32, 32}}, "", ""},
		{"no images", nil, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			song := writeTestWAV(t, int(sampleRate), 2, 10*time.Millisecond)
			if err := os.Rename(song, filepath.Join(dir, "song.wav")); err != nil {
				t.Fatal(err)
			}
			for _, f := range tt.files {
				writeImage(t, filepath.Join(dir, f.name), f.w, f.h, color.Gray{Y: 128})
			}

			cover := FindCover(filepath.Join(dir, "song.wav"), tt.album)
			switch {
			case tt.want == "" && cover != nil:
				t.Fatalf("FindCover() = %s, want none", cover.From)
			case tt.want != "" && (cover == nil || cover.From != filepath.Join(dir, tt.want)):
				t.Fatalf("FindCover() = %+v, want %s", cover, tt.want)
			}
		})
	}
}

func TestFindCoverWebPAndBMP(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed; it is only used to create WebP fixtures")
	}
	writeWebP := func(t *testing.T, path string, w, h int) {
		t.Helper()
		size := strconv.Itoa(w) + "x" + strconv.Itoa(h)
		cmd := exec.CommandContext(context.Background(), "ffmpeg", "-v", "error", "-f", "lavfi",
			"-i", "color=c=red:s="+size, "-frames:v", "1", path)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("ffmpeg cannot write WebP: %v: %s", err, output)
		}
	}
	writeBMP := func(t *testing.T, path string, w, h int) {
		t.Helper()
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := bmp.Encode(file, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	song := func(t *testing.T, dir string) string {
		t.Helper()
		path := filepath.Join(dir, "song.wav")
		if err := os.Rename(writeTestWAV(t, int(sampleRate), 2, 10*time.Millisecond), path); err != nil {
			t.Fatal(err)
		}

		return path
	}

	t.Run("webp", func(t *testing.T) {
		dir := t.TempDir()
		writeWebP(t, filepath.Join(dir, "cover.webp"), 300, 300)
		if cover := FindCover(song(t, dir), ""); cover == nil || cover.From != filepath.Join(dir, "cover.webp") {
			t.Fatalf("FindCover() = %+v, want cover.webp", cover)
		}
	})
	t.Run("bmp", func(t *testing.T) {
		dir := t.TempDir()
		writeBMP(t, filepath.Join(dir, "Folder.BMP"), 300, 300)
		if cover := FindCover(song(t, dir), ""); cover == nil || cover.From != filepath.Join(dir, "Folder.BMP") {
			t.Fatalf("FindCover() = %+v, want Folder.BMP", cover)
		}
	})
	t.Run("non-square webp skipped for a square bmp", func(t *testing.T) {
		dir := t.TempDir()
		writeWebP(t, filepath.Join(dir, "cover.webp"), 900, 300)
		writeBMP(t, filepath.Join(dir, "folder.bmp"), 300, 300)
		if cover := FindCover(song(t, dir), ""); cover == nil || cover.From != filepath.Join(dir, "folder.bmp") {
			t.Fatalf("FindCover() = %+v, want folder.bmp", cover)
		}
	})
}
