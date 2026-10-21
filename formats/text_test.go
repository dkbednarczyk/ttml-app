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
