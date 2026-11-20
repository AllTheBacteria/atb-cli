package convert

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestTSVReader(t *testing.T) {
	long := strings.Repeat("x", 3<<20)
	tests := []struct {
		name  string
		input string
		want  [][]string
	}{
		{"quotes kept", "a\"b\t\"c\"\t\"open\tclose\"\n", [][]string{{`a"b`, `"c"`, `"open`, `close"`}}},
		{"quoted tab splits", "\"a\tb\"\n", [][]string{{`"a`, `b"`}}},
		{"long field", "a\t" + long + "\tb\n", [][]string{{"a", long, "b"}}},
		{"no final newline", "a\tb\nc\td", [][]string{{"a", "b"}, {"c", "d"}}},
		{"empty fields", "\t\t\n", [][]string{{"", "", ""}}},
		{"blank line", "a\n\nb\n", [][]string{{"a"}, {""}, {"b"}}},
		{"carriage return kept", "a\tb\r\n", [][]string{{"a", "b\r"}}},
		{"empty input", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newTSVReader(strings.NewReader(tc.input))
			var got [][]string
			for {
				fields, err := r.next()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, fields)
				if r.line != len(got) {
					t.Fatalf("line = %d after %d lines", r.line, len(got))
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %d lines, want %d: %.80q", len(got), len(tc.want), got)
			}
		})
	}
}

func TestTSVReaderReturnsReadErrors(t *testing.T) {
	r := newTSVReader(io.MultiReader(strings.NewReader("a\tb\n"), errReader{}))
	if _, err := r.next(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.next(); !errors.Is(err, errBroken) {
		t.Fatalf("err = %v, want %v", err, errBroken)
	}
}

var errBroken = errors.New("broken")

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errBroken }
