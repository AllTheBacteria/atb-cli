// Package match implements SQL LIKE-style pattern matching for the Go
// filtering paths, so they return the same rows as the SQLite index paths.
package match

import (
	"regexp"
	"strings"
)

// gtdbSuffix matches the GTDB alphabetic suffix appended to a taxon word, e.g.
// the "_A" in "Enterococcus_A" or "jejunii_A". GTDB splits NCBI genera and
// species and marks the pieces with an uppercase-letter suffix.
var gtdbSuffix = regexp.MustCompile(`_[A-Z]+$`)

// CanonicalSpecies removes the GTDB alphabetic suffix from each whitespace word
// of a species name and collapses runs of whitespace to a single space, so a
// GTDB-suffixed name and the NCBI name a user types reduce to the same string.
// "Enterococcus_A faecium" and "Enterococcus faecium" both yield
// "Enterococcus faecium".
func CanonicalSpecies(species string) string {
	fields := strings.Fields(species)
	for i, word := range fields {
		fields[i] = gtdbSuffix.ReplaceAllString(word, "")
	}
	return strings.Join(fields, " ")
}

// SpeciesMatches reports whether a user-supplied species or genus query matches
// a stored GTDB name, case-insensitively. The GTDB suffix is stripped from the
// stored value only: an NCBI-style query ("Enterococcus faecium") matches every
// GTDB clade ("Enterococcus_A faecium", "Enterococcus_B faecium"), while a query
// that carries an explicit suffix ("Enterococcus_A faecium") still selects only
// that clade.
func SpeciesMatches(query, stored string) bool {
	if strings.EqualFold(query, stored) {
		return true
	}
	return strings.EqualFold(query, CanonicalSpecies(stored))
}

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
