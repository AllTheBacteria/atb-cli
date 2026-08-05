package match_test

import (
	"testing"

	"github.com/allthebacteria/atb-cli/internal/match"
)

func TestLike(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		pattern string
		want    bool
	}{
		{"exact match", "Escherichia coli", "Escherichia coli", true},
		{"exact mismatch", "Escherichia coli", "Salmonella enterica", false},
		{"case insensitive", "Escherichia coli", "escherichia COLI", true},
		{"prefix wildcard", "Streptococcus pneumoniae", "Streptococcus%", true},
		{"prefix wildcard mismatch", "Escherichia coli", "Streptococcus%", false},
		{"suffix wildcard", "Escherichia coli", "%coli", true},
		{"suffix wildcard mismatch", "Escherichia coli", "%enterica", false},
		{"contains wildcard", "Enterococcus faecium", "%faecium%", true},
		{"interior wildcard", "Enterococcus_B faecium", "Enterococcus%faecium", true},
		{"interior wildcard mismatch", "Enterococcus_B faecalis", "Enterococcus%faecium", false},
		{"multiple wildcards", "Streptococcus_A pyogenes", "Strep%_A%genes", true},
		{"underscore is literal", "Campylobacter_D jejuni", "Campylobacter_D jej%", true},
		{"underscore does not match other characters", "Campylobacter-D jejuni", "Campylobacter_D jej%", false},
		{"underscore does not match empty", "CampylobacterD jejuni", "Campylobacter_D jej%", false},
		{"wildcard matches empty run", "Campylobacter_D jejuni", "Campylobacter_D jejuni%", true},
		{"lone wildcard matches everything", "Escherichia coli", "%", true},
		{"empty pattern matches only empty value", "Escherichia coli", "", false},
		{"empty pattern matches empty value", "", "", true},
		{"pattern longer than value", "coli", "Escherichia coli", false},
		{"consecutive wildcards", "Escherichia coli", "Esch%%coli", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := match.Like(tt.value, tt.pattern); got != tt.want {
				t.Errorf("Like(%q, %q) = %v, want %v", tt.value, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestToSQLLike(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{"lowercased", "Salmonella%", "salmonella%"},
		{"percent stays a wildcard", "%coli", "%coli"},
		{"underscore is escaped", "Campylobacter_D jej%", "campylobacter\\_d jej%"},
		{"multiple underscores", "a_b_c", "a\\_b\\_c"},
		{"no wildcards", "Escherichia coli", "escherichia coli"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := match.ToSQLLike(tt.pattern); got != tt.want {
				t.Errorf("ToSQLLike(%q) = %q, want %q", tt.pattern, got, tt.want)
			}
		})
	}
}
