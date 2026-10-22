package splitter

import "testing"

func TestSplitSentences(t *testing.T) {
	input := "第一句。第二句!\n第三句？  \n  。第四句"
	got := SplitSentences(input)
	want := []string{"第一句", "第二句", "第三句", "第四句"}
	if len(got) != len(want) {
		t.Fatalf("unexpected sentence count: got=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected sentence at %d: got=%q want=%q", i, got[i], want[i])
		}
	}
}
