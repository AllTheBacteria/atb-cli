package index

import (
	"testing"

	"github.com/allthebacteria/atb-cli/internal/columns"
)

// The registry records which columns the SQLite index can answer; userColToSQL
// is what answers them. A column marked InIndex with no entry here comes back
// empty on the fast path, and an entry here that is not marked InIndex sends the
// query to the parquet files for data the index already holds.
func TestIndexColumnsMatchRegistry(t *testing.T) {
	for _, c := range columns.All() {
		sqlCol, mapped := userColToSQL[c.Name]
		switch {
		case c.InIndex && !mapped:
			t.Errorf("column %q is marked InIndex but userColToSQL has no entry for it", c.Name)
		case !c.InIndex && mapped:
			t.Errorf("column %q is not marked InIndex but userColToSQL maps it to %q", c.Name, sqlCol)
		}
	}

	for name := range userColToSQL {
		if _, ok := columns.Lookup(name); !ok {
			t.Errorf("userColToSQL maps %q but it is not in the columns registry", name)
		}
	}
}
