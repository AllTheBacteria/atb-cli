package suggest

import (
	"sort"
	"strings"
)

// levenshtein computes edit distance between two strings using two-row DP.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)

	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = min3(del, ins, sub)
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

type scored struct {
	name  string
	score float64
}

// plausible reports whether a candidate is close enough to the input to be
// worth suggesting. Both arguments are already lowercased. A substring match
// either way always qualifies: the user may have typed a prefix, or the stored
// GTDB name may carry an extra clade suffix (e.g. "pseudomonas_e fluorescens"
// against "pseudomonas fluorescens"). Otherwise the edit distance must be
// within a quarter of the longer name's rune length. Legitimate typos and GTDB
// renames measure well under that bound, while an out-of-dataset name (a
// different genus sharing only an epithet) sits far beyond it, so it is dropped
// rather than surfaced as a misleading match.
func plausible(input, cand string, dist int) bool {
	if strings.Contains(cand, input) || strings.Contains(input, cand) {
		return true
	}
	maxLen := len([]rune(input))
	if l := len([]rune(cand)); l > maxLen {
		maxLen = l
	}
	return dist*4 <= maxLen
}

// Suggest returns up to n species names closest to the input, limited to
// plausible near-spellings (see plausible). It returns nothing when no
// candidate is close, so callers can distinguish "did you mean X?" from "no
// such name exists here". Levenshtein distance is computed case-insensitively;
// substring matches get their distance halved as a relevance boost.
func Suggest(input string, species []string, n int) []string {
	lower := strings.ToLower(input)

	results := make([]scored, 0, len(species))
	for _, s := range species {
		cand := strings.ToLower(s)
		dist := levenshtein(lower, cand)
		if !plausible(lower, cand, dist) {
			continue
		}
		score := float64(dist)
		if strings.Contains(cand, lower) {
			score /= 2
		}
		results = append(results, scored{name: s, score: score})
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].score < results[j].score
	})

	if n > len(results) {
		n = len(results)
	}

	out := make([]string, n)
	for i := range out {
		out[i] = results[i].name
	}
	return out
}
