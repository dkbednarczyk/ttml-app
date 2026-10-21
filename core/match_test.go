package core

import (
	"strings"
	"testing"
)

func TestMismatches(t *testing.T) {
	at := func(v Ms) *Ms { return &v }
	lines := func(lastEnd Ms) []Line {
		return []Line{
			{Timing: Timing{Begin: at(1000), End: at(2000)}},
			{Timing: Timing{Begin: at(2000), End: at(lastEnd)}},
			{Text: "untimed"},
		}
	}
	isrc := []MetaEntry{{"isrc", "HKE571108401"}, {"isrc", "CNZ171500183"}}

	tests := []struct {
		name      string
		project   Project
		lyricsDur *Ms
		audioEnd  Ms
		audioISRC string
		want      []string // substrings, one per reason
	}{
		{"all match", Project{Meta: isrc, Lines: lines(244_800)}, at(249_000), 249_895, "HK-E57-11-08401", nil},
		{"no signals at all", Project{Lines: lines(244_800)}, nil, 249_895, "", nil},
		{"lyrics run past the audio", Project{Lines: lines(312_000)}, nil, 249_895, "", []string{"run to 5:12.000"}},
		{"stated length differs", Project{Lines: lines(100_000)}, at(400_000), 249_895, "", []string{"song 6:40.000 long"}},
		{"length within tolerance", Project{Lines: lines(100_000)}, at(251_500), 249_895, "", nil},
		{
			"different recording",
			Project{Meta: isrc, Lines: lines(100_000)},
			nil, 249_895, "USRC17607839",
			[]string{"HKE571108401 or CNZ171500183, but the audio is USRC17607839"},
		},
		{"audio has no ISRC", Project{Meta: isrc, Lines: lines(100_000)}, nil, 249_895, "", nil},
	}
	for _, tt := range tests {
		got := Mismatches(&tt.project, tt.lyricsDur, tt.audioEnd, tt.audioISRC)
		if len(got) != len(tt.want) {
			t.Errorf("%s: Mismatches() = %q, want %d reasons", tt.name, got, len(tt.want))

			continue
		}
		for i, want := range tt.want {
			if !strings.Contains(got[i], want) {
				t.Errorf("%s: reason %q lacks %q", tt.name, got[i], want)
			}
		}
	}
}

func TestFillMeta(t *testing.T) {
	meta := []MetaEntry{{"musicName", "Imported title"}, {"ncmMusicId", "1"}}
	defaults := []MetaEntry{{"musicName", "Tag title"}, {"artists", "Tag artist"}, {"isrc", ""}}

	got := FillMeta(meta, defaults)
	want := []MetaEntry{{"musicName", "Imported title"}, {"ncmMusicId", "1"}, {"artists", "Tag artist"}}
	if len(got) != len(want) {
		t.Fatalf("FillMeta() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %v, want %v", i, got[i], want[i])
		}
	}
	if len(meta) != 2 {
		t.Error("FillMeta changed its input")
	}
}
