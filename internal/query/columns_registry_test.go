package query

import (
	"slices"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/columns"
)

// Plan decides which parquet files to read from the requested column names, and
// the columns registry is the list of names a user may request. They are two
// lists of the same thing, so a column added to one and not the other either
// cannot be requested or is requested without its file being read.
func TestPlanReadsTheSourceOfEveryRegisteredColumn(t *testing.T) {
	for _, c := range columns.All() {
		t.Run(c.Name, func(t *testing.T) {
			tables := Plan(Filters{}, []string{c.Name}).Tables
			if !slices.Contains(tables, string(c.Source)) {
				t.Errorf("Plan for %q reads %v, which does not include its source %q",
					c.Name, tables, c.Source)
			}
		})
	}
}

func TestEveryRoutedColumnIsRegistered(t *testing.T) {
	routed := map[columns.Source]map[string]bool{
		columns.CheckM2:       checkm2Fields,
		columns.AssemblyStats: assemblyStatsFields,
		columns.Sylph:         sylphFields,
		columns.MLST:          mlstFields,
		columns.ENA:           enaFields,
	}

	for source, fields := range routed {
		for name := range fields {
			c, ok := columns.Lookup(name)
			if !ok {
				t.Errorf("Plan routes %q to %q but it is not in the columns registry", name, source)
				continue
			}
			if c.Source != source {
				t.Errorf("Plan routes %q to %q but the registry records it under %q", name, source, c.Source)
			}
		}
	}
}

// assembly.parquet is read for every query, so listing one of its columns in a
// field map would add a join that is never needed.
func TestAssemblyColumnsAreNotRouted(t *testing.T) {
	fieldMaps := []map[string]bool{
		checkm2Fields, assemblyStatsFields, sylphFields, mlstFields, enaFields,
	}

	for _, c := range columns.All() {
		if c.Source != columns.Assembly {
			continue
		}
		for _, fields := range fieldMaps {
			if fields[c.Name] {
				t.Errorf("assembly column %q appears in a planner field map", c.Name)
			}
		}
	}
}
