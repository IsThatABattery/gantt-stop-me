package llm

import (
	"strings"
	"unicode/utf8"
)

// MaxInstructions is the most characters a person's own instructions to an agent can have.
const MaxInstructions = 4000

// Instructions trims s, a person's own instructions to an agent, and reports whether it has at most MaxInstructions
// characters and no NUL, which Postgres cannot store. Empty is none.
func Instructions(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, utf8.RuneCountInString(s) <= MaxInstructions && !strings.ContainsRune(s, 0)
}

// Extra is the prompt section, after an agent's own rules, that adds instructions from who, such as "the plan's
// creator", led by a blank line. It is "" for none, which leaves the prompt as it is.
func Extra(who, instructions string) string {
	if instructions == "" {
		return ""
	}
	return "\n\nExtra instructions from " + who + ". Follow them where they don't conflict with the rules above:\n\n" + instructions
}
