package columns

import (
	"fmt"
	"strings"

	"github.com/allthebacteria/atb-cli/internal/suggest"
)

// maxSuggestions caps how many near-spellings an error offers.
const maxSuggestions = 3

// Validate returns an error for the first name that is not a column. The error
// names the closest columns when any are close and always points at the command
// that lists them, so a typo stops the query instead of producing a blank
// column.
func Validate(names []string) error {
	for _, name := range names {
		if _, ok := Lookup(name); ok {
			continue
		}
		hints := suggest.Identifiers(name, Names(), maxSuggestions)
		if len(hints) == 0 {
			return fmt.Errorf("unknown column %q. Run 'atb columns' to list all", name)
		}
		return fmt.Errorf("unknown column %q - did you mean %s? Run 'atb columns' to list all",
			name, quotedList(hints))
	}
	return nil
}

// quotedList renders names as `"a"`, `"a" or "b"`, or `"a", "b" or "c"`.
func quotedList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = fmt.Sprintf("%q", n)
	}
	if len(quoted) == 1 {
		return quoted[0]
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + " or " + quoted[len(quoted)-1]
}
