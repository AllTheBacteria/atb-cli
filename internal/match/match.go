// Package match implements SQL LIKE-style pattern matching for the Go
// filtering paths, so they return the same rows as the SQLite index paths.
package match

import "strings"

// Like reports whether value matches pattern. % matches any sequence of
// characters, every other character including _ is literal, and matching is
// case-insensitive.
func Like(value, pattern string) bool {
	return like(strings.ToLower(value), strings.ToLower(pattern))
}

// ToSQLLike converts a pattern into the SQL LIKE equivalent of Like: lowercased
// for case-insensitive comparison against a lowercased column, % left as the
// wildcard, and _ escaped so it matches itself. The resulting pattern requires
// ESCAPE '\' on the LIKE clause.
func ToSQLLike(pattern string) string {
	var b strings.Builder
	for _, ch := range strings.ToLower(pattern) {
		if ch == '_' {
			b.WriteString("\\_")
			continue
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// like walks value and pattern together, remembering the most recent % so a
// failed match can resume from one character later instead of giving up.
func like(value, pattern string) bool {
	var vi, pi int
	star, resume := -1, 0

	for vi < len(value) {
		switch {
		case pi < len(pattern) && pattern[pi] == '%':
			star, resume = pi, vi
			pi++
		case pi < len(pattern) && pattern[pi] == value[vi]:
			pi++
			vi++
		case star >= 0:
			pi = star + 1
			resume++
			vi = resume
		default:
			return false
		}
	}

	for pi < len(pattern) && pattern[pi] == '%' {
		pi++
	}
	return pi == len(pattern)
}
