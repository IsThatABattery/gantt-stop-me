package llm

import (
	"strings"
	"testing"
)

func TestInstructions(t *testing.T) {
	for _, c := range []struct{ in, out, why string }{
		{"  Be brief.\n", "Be brief.", ""},
		{"", "", ""},
		{strings.Repeat("é", MaxInstructions), strings.Repeat("é", MaxInstructions), ""},
		{strings.Repeat("é", MaxInstructions+1), strings.Repeat("é", MaxInstructions+1), "Instructions can have at most 4000 characters."},
		{"Be\x00brief.", "Be\x00brief.", "Instructions cannot contain a NUL character."},
	} {
		if out, why := Instructions(c.in); out != c.out || why != c.why {
			t.Errorf("Instructions(%q) = %q, %q, want %q, %q", c.in, out, why, c.out, c.why)
		}
	}
}
