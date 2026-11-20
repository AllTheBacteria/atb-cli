package convert

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

// TestReadAllRoundTrip writes each release table and reads it back with the
// row types the atb CLI uses. The second row holds nulls in every column
// except sample_accession and in_release_1.
func TestReadAllRoundTrip(t *testing.T) {
	t.Run("assembly", func(t *testing.T) { roundTrip[pq.AssemblyRow](t, assembly) })
	t.Run("assembly_stats", func(t *testing.T) { roundTrip[pq.AssemblyStatsRow](t, assemblyStats) })
	t.Run("checkm2", func(t *testing.T) { roundTrip[pq.CheckM2Row](t, checkM2) })
	t.Run("sylph", func(t *testing.T) { roundTrip[pq.SylphRow](t, sylph) })
	t.Run("run", func(t *testing.T) { roundTrip[pq.RunRow](t, run) })
}

func roundTrip[T any](t *testing.T, table Table) {
	full := make(map[string]string)
	nulls := make(map[string]string)
	for i, c := range table.Columns {
		full[c.Name] = sampleValue(c, i, 1)
		switch {
		case c.Name == "sample_accession" || c.Want != "":
			nulls[c.Name] = sampleValue(c, i, 2)
		case c.Kind == String:
			nulls[c.Name] = ""
		default:
			nulls[c.Name] = "NA"
		}
	}
	lines := []string{header(table), tsvLine(table, full), tsvLine(table, nulls)}
	out, _, err := convertLines(t, table, lines, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := pq.ReadAll[T](out)
	if err != nil {
		t.Fatal(err)
	}
	want := []T{expectedRow[T](t, full), expectedRow[T](t, nulls)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadAll:\n got %+v\nwant %+v", got, want)
	}
	var zero T
	if reflect.DeepEqual(want[0], zero) {
		t.Fatal("expected row is empty")
	}
}

func tsvLine(table Table, values map[string]string) string {
	fields := make([]string, len(table.Columns))
	for i, c := range table.Columns {
		fields[i] = values[c.Name]
	}
	return strings.Join(fields, "\t")
}

// expectedRow fills a T from the TSV values by parquet tag name. Empty and NA
// numeric values and empty strings become zero values.
func expectedRow[T any](t *testing.T, values map[string]string) T {
	t.Helper()
	var row T
	v := reflect.ValueOf(&row).Elem()
	for i := range v.NumField() {
		field := v.Type().Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("parquet"), ",")
		raw, ok := values[name]
		if !ok {
			t.Fatalf("%s.%s reads column %s, which the table does not write", v.Type().Name(), field.Name, name)
		}
		if raw == "" || (raw == "NA" && field.Type.Kind() != reflect.String) {
			continue
		}
		switch field.Type.Kind() {
		case reflect.String:
			v.Field(i).SetString(raw)
		case reflect.Int64:
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			v.Field(i).SetInt(n)
		case reflect.Float64:
			f, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				t.Fatal(err)
			}
			v.Field(i).SetFloat(f)
		default:
			t.Fatalf("%s.%s has unsupported kind %s", v.Type().Name(), field.Name, field.Type.Kind())
		}
	}
	return row
}
