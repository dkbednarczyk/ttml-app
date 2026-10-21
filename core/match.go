package core

import (
	"fmt"
	"slices"
	"strings"
)

// durationTolerance is how far a lyrics file's stated length may be from the
// audio's before it counts as a different recording. Decoders disagree by a
// few hundred milliseconds on some files (MP3 padding, for one).
const durationTolerance Ms = 2000

// Mismatches lists reasons the lyrics may belong to a different recording
// than the audio, worded for the user. lyricsDur is the length the lyrics file
// states (nil if none); audioEnd and audioISRC describe the audio file.
func Mismatches(p *Project, lyricsDur *Ms, audioEnd Ms, audioISRC string) []string {
	var reasons []string

	var lastEnd Ms
	for _, line := range p.Lines {
		if line.End != nil {
			lastEnd = max(lastEnd, *line.End)
		}
	}
	if lastEnd > audioEnd {
		reasons = append(reasons, fmt.Sprintf("The lyrics run to %s, but the audio is only %s long.", lastEnd, audioEnd))
	}

	if lyricsDur != nil && (*lyricsDur-audioEnd > durationTolerance || audioEnd-*lyricsDur > durationTolerance) {
		reasons = append(reasons, fmt.Sprintf("The lyrics file is for a song %s long, but the audio is %s.", *lyricsDur, audioEnd))
	}

	if audioISRC != "" {
		lyricsISRCs := p.MetaValues("isrc")
		for i, isrc := range lyricsISRCs {
			lyricsISRCs[i] = normalizeISRC(isrc)
		}
		if len(lyricsISRCs) > 0 && !slices.Contains(lyricsISRCs, normalizeISRC(audioISRC)) {
			reasons = append(reasons, fmt.Sprintf("The lyrics file is for recording %s, but the audio is %s (ISRC).",
				strings.Join(p.MetaValues("isrc"), " or "), audioISRC))
		}
	}

	return reasons
}

// normalizeISRC drops the dashes and spaces some tags write and ignores case.
func normalizeISRC(isrc string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(isrc))
}
