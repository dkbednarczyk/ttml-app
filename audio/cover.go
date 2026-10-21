package audio

import (
	"bytes"
	"image"
	_ "image/gif"  // register GIF for cover files
	_ "image/jpeg" // register JPEG for embedded and file covers
	_ "image/png"  // register PNG for embedded and file covers
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dhowden/tag"
	_ "golang.org/x/image/bmp"  // register BMP for cover files
	_ "golang.org/x/image/webp" // register WebP for embedded and file covers
)

const (
	// maxCoverAspect is how far from square a cover may be (longer side over
	// shorter). Wider or taller images are likely scans, banners or booklets.
	maxCoverAspect = 1.25
	// minCoverSide drops icons and thumbnails too small to be album art.
	minCoverSide = 64
)

var (
	// coverExts are the image types a cover file may have.
	coverExts = []string{".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp"}
	// coverNames are the usual names of a cover beside the audio, best first.
	coverNames = []string{"cover", "folder", "front", "album"}
)

// Cover is album art found for an audio file.
type Cover struct {
	Image image.Image
	From  string // "embedded", or the image file's path
}

// FindCover looks for album art for the audio file at path: the picture in
// its tags, then an image beside it named cover, folder, front, album or after
// the album (album is the tagged album name), then one with "cover" or
// "front" in its name, then the folder's only image. Images that aren't
// roughly square are skipped. It returns nil if nothing suitable turns up.
func FindCover(path, album string) *Cover {
	if img := decodeCover(bytes.NewReader(embeddedPicture(path))); img != nil {
		return &Cover{Image: img, From: "embedded"}
	}
	for _, file := range coverFiles(filepath.Dir(path), album) {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		if img := decodeCover(bytes.NewReader(data)); img != nil {
			return &Cover{Image: img, From: file}
		}
	}

	return nil
}

func embeddedPicture(path string) []byte {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	meta, err := tag.ReadFrom(file)
	if err != nil || meta.Picture() == nil {
		return nil
	}

	return meta.Picture().Data
}

// coverFiles lists the images in dir that may be the cover, best first.
func coverFiles(dir, album string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	images := map[string]string{} // lowercased name -> name
	var names []string
	for _, e := range entries {
		if !e.IsDir() && slices.Contains(coverExts, strings.ToLower(filepath.Ext(e.Name()))) {
			images[strings.ToLower(e.Name())] = e.Name()
			names = append(names, e.Name())
		}
	}

	var files []string
	add := func(name string) {
		if path := filepath.Join(dir, name); !slices.Contains(files, path) {
			files = append(files, path)
		}
	}
	stems := coverNames
	if album != "" {
		stems = append(slices.Clone(coverNames), strings.ToLower(album))
	}
	for _, stem := range stems {
		for _, ext := range coverExts {
			if name, ok := images[stem+ext]; ok {
				add(name)
			}
		}
	}
	for _, name := range names {
		if lower := strings.ToLower(name); strings.Contains(lower, "cover") || strings.Contains(lower, "front") {
			add(name)
		}
	}
	if len(names) == 1 {
		add(names[0])
	}

	return files
}

// decodeCover decodes an image if it looks like album art, else returns nil.
// The size is checked from the header before the whole image is decoded.
func decodeCover(r io.ReadSeeker) image.Image {
	config, _, err := image.DecodeConfig(r)
	if err != nil || !squareEnough(config.Width, config.Height) {
		return nil
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil
	}
	img, _, err := image.Decode(r)
	if err != nil {
		return nil
	}

	return img
}

func squareEnough(width, height int) bool {
	short, long := min(width, height), max(width, height)

	return short >= minCoverSide && float64(long) <= float64(short)*maxCoverAspect
}
