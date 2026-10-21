package audio

import (
	"os"
	"strings"

	"github.com/dhowden/tag"
)

// Tags is the track metadata read from an audio file.
type Tags struct {
	Title  string
	Artist string
	Album  string
	ISRC   string // the recording's code; identifies this exact recording
}

// isrcKeys are the raw tag names holding the ISRC: Vorbis comments (FLAC,
// Ogg), lowercased by the tag library, and ID3v2.3/2.4 and ID3v2.2 frames (MP3).
var isrcKeys = []string{"isrc", "TSRC", "TRC"}

// ReadTags reads the title, artist, album and ISRC from the file's tags.
// Missing or unreadable tags give empty fields; tags are optional, so this
// never fails.
func ReadTags(path string) Tags {
	file, err := os.Open(path)
	if err != nil {
		return Tags{}
	}
	defer file.Close()

	meta, err := tag.ReadFrom(file)
	if err != nil {
		return Tags{}
	}

	artist := meta.Artist()
	if strings.TrimSpace(artist) == "" {
		artist = meta.AlbumArtist()
	}

	tags := Tags{
		Title:  strings.TrimSpace(meta.Title()),
		Artist: strings.TrimSpace(artist),
		Album:  strings.TrimSpace(meta.Album()),
	}
	tags.ISRC = readISRC(meta.Raw())

	return tags
}

// readISRC finds the ISRC in raw tags: the standard fields first, then an
// ID3 user-defined (TXXX) frame named ISRC, which ffmpeg and some taggers write.
func readISRC(raw map[string]any) string {
	for _, key := range isrcKeys {
		if isrc, ok := raw[key].(string); ok && cleanTag(isrc) != "" {
			return cleanTag(isrc)
		}
	}
	for key, value := range raw {
		frame, ok := value.(*tag.Comm)
		if ok && strings.HasPrefix(key, "TXX") && strings.EqualFold(frame.Description, "ISRC") {
			return cleanTag(frame.Text)
		}
	}

	return ""
}

// cleanTag trims whitespace and the NUL terminators some ID3 frames keep.
func cleanTag(value string) string {
	return strings.Trim(value, " \t\r\n\x00")
}
