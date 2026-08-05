package columns

import (
	"strings"
	"testing"
)

func TestValidateAcceptsKnownColumns(t *testing.T) {
	if err := Validate([]string{"sample_accession", "N50", "mlst_st", "country"}); err != nil {
		t.Errorf("Validate(known columns) = %v, want nil", err)
	}
}

func TestValidateAcceptsNoColumns(t *testing.T) {
	if err := Validate(nil); err != nil {
		t.Errorf("Validate(nil) = %v, want nil: no --columns means the default set", err)
	}
}

func TestValidateRejectsUnknownColumnWithSuggestion(t *testing.T) {
	err := Validate([]string{"sample_accession", "N5O"})
	if err == nil {
		t.Fatal("Validate([sample_accession N5O]) = nil, want an error naming N5O")
	}

	msg := err.Error()
	for _, want := range []string{`unknown column "N5O"`, "did you mean", `"N50"`, "atb columns"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not contain %q", msg, want)
		}
	}
}

func TestValidateSuggestsTheCorrectCase(t *testing.T) {
	err := Validate([]string{"contamination"})
	if err == nil {
		t.Fatal("Validate([contamination]) = nil, want an error: column names are case-sensitive")
	}

	if msg := err.Error(); !strings.Contains(msg, `"Contamination"`) {
		t.Errorf("error %q does not suggest the correctly cased %q", msg, "Contamination")
	}
}

func TestValidateDoesNotGuessWhenNothingIsClose(t *testing.T) {
	err := Validate([]string{"xyzzy"})
	if err == nil {
		t.Fatal("Validate([xyzzy]) = nil, want an error")
	}

	msg := err.Error()
	if !strings.Contains(msg, `unknown column "xyzzy"`) {
		t.Errorf("error %q does not name the unknown column", msg)
	}
	if strings.Contains(msg, "did you mean") {
		t.Errorf("error %q offers a suggestion for a name no column is close to", msg)
	}
	if !strings.Contains(msg, "atb columns") {
		t.Errorf("error %q does not point at how to list the columns", msg)
	}
}
