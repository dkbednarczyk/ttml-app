package formats

import "testing"

func TestParseText(t *testing.T) {
	const pasted = "[Verse 1]\r\n" +
		"We've seen this place before\r\n" +
		"  A room that's   void of doors  \n" +
		"\n" +
		"   \n" +
		"[Chorus]\n" +
		"I'm forced to look inside myself\n" +
		"(oh) [softly]\n" +
		"Last line, no newline"

	got := ParseText(pasted)
	want := []string{
		"We've seen this place before",
		"A room that's void of doors",
		"I'm forced to look inside myself",
		"(oh) [softly]", // brackets inside a lyric are kept
		"Last line, no newline",
	}
	if len(got) != len(want) {
		t.Fatalf("ParseText() = %d lines, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Text != want[i] || got[i].Begin != nil || got[i].End != nil {
			t.Errorf("line %d = %+v, want untimed %q", i+1, got[i], want[i])
		}
	}

	if got := ParseText(" \n\n[Intro]\n"); len(got) != 0 {
		t.Errorf("ParseText(blank and markers) = %+v, want no lines", got)
	}
}

func TestParseTextBackgroundVocals(t *testing.T) {
	tests := []struct{ line, text, background string }{
		{"I can't let this go (let it burn)", "I can't let this go", "(let it burn)"},
		{"We are the ones (oh) (oh)", "We are the ones", "(oh) (oh)"},
		{"(Hey! Hey!)", "", "(Hey! Hey!)"},
		{"Say it (say it (louder))", "Say it", "(say it (louder))"},
		{"I (don't) care", "I (don't) care", ""},
		{"Stuck in a loop (x2)", "Stuck in a loop (x2)", ""},
		{"Again (oh) (×4)", "Again (oh) (×4)", ""},
		{"Unbalanced)", "Unbalanced)", ""},
		{"Never again(again)", "Never again", "(again)"},
	}
	for _, tt := range tests {
		got := ParseText(tt.line)
		if len(got) != 1 || got[0].Text != tt.text || got[0].Background != tt.background {
			t.Errorf("ParseText(%q) = %+v, want text %q and background %q", tt.line, got, tt.text, tt.background)
		}
	}
}
