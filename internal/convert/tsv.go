package convert

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// tsvReader reads tab separated lines. Quotes have no special meaning and a
// line may be any length.
type tsvReader struct {
	r    *bufio.Reader
	line int
}

func newTSVReader(r io.Reader) *tsvReader {
	return &tsvReader{r: bufio.NewReaderSize(r, 1<<20)}
}

// next returns the fields of the next line, or io.EOF after the last line.
// The final line may omit its trailing newline.
func (t *tsvReader) next() ([]string, error) {
	s, err := t.r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if s == "" {
		return nil, io.EOF
	}
	t.line++
	return strings.Split(strings.TrimSuffix(s, "\n"), "\t"), nil
}
