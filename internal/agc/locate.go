package agc

// LocateStatus classifies how an accession resolved against the map and index.
type LocateStatus int

const (
	// LocateFound: in the map and its batch is in the index.
	LocateFound LocateStatus = iota
	// LocateUnresolved: not in the accession->batch map.
	LocateUnresolved
	// LocateNotYetAvailable: maps to a batch that is not (yet) in the index -
	// the collection node holding it is still being published.
	LocateNotYetAvailable
)

// LocateResult is one accession's resolution for `atb agc locate`.
type LocateResult struct {
	Accession string
	Batch     string // archive stem; "" when Status is LocateUnresolved
	Species   string // index species (Project); "" unless Status is LocateFound
	Node      string // OSF node id (ProjectID); "" unless Status is LocateFound
	URL       string // OSF download URL; "" unless Status is LocateFound
	Status    LocateStatus
}

// Locate resolves each accession against the accession->batch map m and the
// batch index refs (byName, from RefsFromIndex), preserving input order. It
// performs no I/O: callers fetch m and byName first.
func Locate(accessions []string, m ArchiveMap, byName map[string]ArchiveRef) []LocateResult {
	out := make([]LocateResult, 0, len(accessions))
	for _, acc := range accessions {
		archive, ok := m[acc]
		if !ok {
			out = append(out, LocateResult{Accession: acc, Status: LocateUnresolved})
			continue
		}
		ref, ok := byName[archive]
		if !ok {
			out = append(out, LocateResult{Accession: acc, Batch: archive, Status: LocateNotYetAvailable})
			continue
		}
		out = append(out, LocateResult{
			Accession: acc,
			Batch:     archive,
			Species:   ref.Species,
			Node:      ref.Node,
			URL:       ref.URL,
			Status:    LocateFound,
		})
	}
	return out
}
