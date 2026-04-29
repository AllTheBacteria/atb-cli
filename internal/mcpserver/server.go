package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/allthebacteria/atb-cli/internal/amr"
	idx "github.com/allthebacteria/atb-cli/internal/index"
)

// NewServer creates and configures an MCP server with all ATB tools.
func NewServer(dataDir string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "atb",
		Version: "1.0",
	}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "atb_query",
		Description: "Search bacterial genomes in the AllTheBacteria database",
	}, makeQueryHandler(dataDir))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "atb_amr",
		Description: "Query AMR (antimicrobial resistance) genes for a bacterial species",
	}, makeAMRHandler(dataDir))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "atb_info",
		Description: "Get all available metadata for a specific sample by accession ID",
	}, makeInfoHandler(dataDir))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "atb_stats",
		Description: "Get summary statistics for the AllTheBacteria database",
	}, makeStatsHandler(dataDir))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "atb_species_list",
		Description: "List available species in the database sorted by sample count",
	}, makeSpeciesListHandler(dataDir))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "atb_mlst",
		Description: "Query MLST (Multi-Locus Sequence Typing) data for bacterial genomes",
	}, makeMLSTHandler(dataDir))

	return s
}

// ServeStdio starts the MCP server using stdio transport (for Claude Code, Cursor, Codex CLI).
func ServeStdio(ctx context.Context, dataDir string) error {
	return NewServer(dataDir).Run(ctx, &mcp.StdioTransport{})
}

// ServeHTTP starts the MCP server using HTTP/SSE transport (for ChatGPT, OpenAI API, remote clients).
func ServeHTTP(ctx context.Context, dataDir string, addr string) error {
	handler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
		return NewServer(dataDir)
	}, nil)

	mux := http.NewServeMux()
	mux.Handle("/", corsMiddleware(handler))

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	fmt.Fprintf(os.Stderr, "MCP server listening on http://%s\n", addr)
	fmt.Fprintf(os.Stderr, "SSE endpoint: http://%s/sse\n", addr)

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background()) //nolint:errcheck
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// textResult wraps a JSON-serialisable value as a text tool result.
func textResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshalling result: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

// splitMCPCSV trims and splits a comma-separated MCP input value, dropping empty entries.
func splitMCPCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func errorResult(msg string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}, nil, nil
}

// --- atb_query ---

type queryInput struct {
	Species          string  `json:"species,omitempty"          jsonschema:"Species name (case-insensitive)"`
	Genus            string  `json:"genus,omitempty"            jsonschema:"Genus name"`
	HQOnly           bool    `json:"hq_only,omitempty"          jsonschema:"Only high-quality genomes (CheckM2 PASS)"`
	MinCompleteness  float64 `json:"min_completeness,omitempty" jsonschema:"Minimum CheckM2 completeness (0-100)"`
	MaxContamination float64 `json:"max_contamination,omitempty" jsonschema:"Maximum CheckM2 contamination"`
	MinN50           int64   `json:"min_n50,omitempty"          jsonschema:"Minimum assembly N50"`
	Dataset          string  `json:"dataset,omitempty"          jsonschema:"Dataset name filter"`
	Limit            int     `json:"limit,omitempty"            jsonschema:"Maximum results to return (default 20)"`
}

func makeQueryHandler(dataDir string) mcp.ToolHandlerFor[queryInput, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in queryInput) (*mcp.CallToolResult, any, error) {
		db, err := idx.Open(dataDir)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to open index: %v", err))
		}
		defer db.Close()

		limit := in.Limit
		if limit <= 0 {
			limit = 20
		}

		rows, err := db.Query(idx.QueryParams{
			Species:          in.Species,
			Genus:            in.Genus,
			HQOnly:           in.HQOnly,
			MinCompleteness:  in.MinCompleteness,
			MaxContamination: in.MaxContamination,
			MinN50:           in.MinN50,
			Dataset:          in.Dataset,
			Limit:            limit,
		})
		if err != nil {
			return errorResult(fmt.Sprintf("query failed: %v", err))
		}
		if rows == nil {
			rows = []map[string]string{}
		}
		return textResult(rows)
	}
}

// --- atb_amr ---

type amrInput struct {
	Species     string `json:"species,omitempty"        jsonschema:"Full species name(s) for an exact match, e.g. 'Escherichia coli'. Comma-separated for multiple. Optional when gene, drug_class, or genus is given."`
	Genus       string `json:"genus,omitempty"          jsonschema:"Genus name(s) for a broader match, e.g. 'Escherichia'. Comma-separated for multiple."`
	DrugClass   string `json:"drug_class,omitempty"     jsonschema:"Filter by drug class (case-insensitive substring)"`
	Gene        string `json:"gene,omitempty"           jsonschema:"Filter by gene symbol (supports % wildcards)"`
	ElementType string `json:"element_type,omitempty"   jsonschema:"Type: amr, stress, virulence, or all (default: amr)"`
	HQOnly      bool   `json:"hq_only,omitempty"        jsonschema:"Only include HQ samples"`
	Limit       int    `json:"limit,omitempty"          jsonschema:"Maximum results (default 50)"`
}

func makeAMRHandler(dataDir string) mcp.ToolHandlerFor[amrInput, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in amrInput) (*mcp.CallToolResult, any, error) {
		speciesList := splitMCPCSV(in.Species)
		explicitGenera := splitMCPCSV(in.Genus)

		generaSet := make(map[string]struct{}, len(speciesList)+len(explicitGenera))
		for _, sp := range speciesList {
			parts := strings.Fields(sp)
			if len(parts) == 0 {
				continue
			}
			generaSet[parts[0]] = struct{}{}
		}
		for _, g := range explicitGenera {
			generaSet[g] = struct{}{}
		}
		genera := make([]string, 0, len(generaSet))
		for g := range generaSet {
			genera = append(genera, g)
		}

		if len(genera) == 0 && in.Gene == "" && in.DrugClass == "" {
			return errorResult("species, genus, gene, or drug_class is required")
		}

		elementType := in.ElementType
		if elementType == "" {
			elementType = "AMR"
		}

		// Check if amrfinderplus.parquet exists before querying.
		amrPath := filepath.Join(dataDir, amr.AMRFileName)
		if _, err := os.Stat(amrPath); err != nil {
			return errorResult(fmt.Sprintf("AMR data not found. Run: atb fetch to download %s", amr.AMRFileName))
		}

		limit := in.Limit
		if limit <= 0 {
			limit = 50
		}

		filters := amr.Filters{
			Class:       in.DrugClass,
			GenePattern: in.Gene,
			ElementType: elementType,
			Genera:      genera,
			Species:     speciesList,
			Limit:       limit,
		}

		// If hq_only, get the HQ sample set from the index.
		if in.HQOnly {
			db, err := idx.Open(dataDir)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to open index for HQ filter: %v", err))
			}

			// idx.QueryParams takes one species/genus at a time, so we union
			// across the list. If only gene/drug_class were given, run once
			// with no taxon filter.
			type idxCall struct {
				species string
				genus   string
			}
			var calls []idxCall
			for _, sp := range speciesList {
				calls = append(calls, idxCall{species: sp})
			}
			for _, g := range explicitGenera {
				calls = append(calls, idxCall{genus: g})
			}
			if len(calls) == 0 {
				calls = []idxCall{{}}
			}

			hqSet := make(map[string]struct{})
			for _, c := range calls {
				rows, qErr := db.Query(idx.QueryParams{
					Species: c.species,
					Genus:   c.genus,
					HQOnly:  true,
					Limit:   0,
				})
				if qErr != nil {
					db.Close()
					return errorResult(fmt.Sprintf("HQ filter query failed: %v", qErr))
				}
				for _, r := range rows {
					if acc := r["sample_accession"]; acc != "" {
						hqSet[acc] = struct{}{}
					}
				}
			}
			db.Close()
			filters.Samples = hqSet
		}

		results, err := amr.Query(dataDir, filters)
		if err != nil {
			return errorResult(fmt.Sprintf("AMR query failed: %v", err))
		}

		if results == nil {
			results = []amr.Result{}
		}
		return textResult(results)
	}
}

// --- atb_info ---

type infoInput struct {
	SampleID string `json:"sample_id" jsonschema:"Sample accession ID (required)"`
}

func makeInfoHandler(dataDir string) mcp.ToolHandlerFor[infoInput, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in infoInput) (*mcp.CallToolResult, any, error) {
		if in.SampleID == "" {
			return errorResult("sample_id is required")
		}

		db, err := idx.Open(dataDir)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to open index: %v", err))
		}
		defer db.Close()

		row, err := db.InfoRow(in.SampleID)
		if err != nil {
			return errorResult(fmt.Sprintf("sample not found: %v", err))
		}
		return textResult(row)
	}
}

// --- atb_stats ---

type statsInput struct {
	Species string `json:"species,omitempty" jsonschema:"Filter stats by species"`
	HQOnly  bool   `json:"hq_only,omitempty" jsonschema:"Only count HQ genomes"`
}

func makeStatsHandler(dataDir string) mcp.ToolHandlerFor[statsInput, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in statsInput) (*mcp.CallToolResult, any, error) {
		db, err := idx.Open(dataDir)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to open index: %v", err))
		}
		defer db.Close()

		stats, err := db.QueryStats(in.Species, in.HQOnly)
		if err != nil {
			return errorResult(fmt.Sprintf("stats query failed: %v", err))
		}
		return textResult(stats)
	}
}

// --- atb_species_list ---

type speciesListInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"Maximum species to return (default 50)"`
}

func makeSpeciesListHandler(dataDir string) mcp.ToolHandlerFor[speciesListInput, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in speciesListInput) (*mcp.CallToolResult, any, error) {
		db, err := idx.Open(dataDir)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to open index: %v", err))
		}
		defer db.Close()

		limit := in.Limit
		if limit <= 0 {
			limit = 50
		}

		list, err := db.SpeciesList(limit)
		if err != nil {
			return errorResult(fmt.Sprintf("species list query failed: %v", err))
		}
		if list == nil {
			list = []idx.SpeciesCount{}
		}
		return textResult(list)
	}
}

// --- atb_mlst ---

type mlstInput struct {
	Species      string `json:"species,omitempty"       jsonschema:"Species name filter (case-insensitive)"`
	SequenceType string `json:"sequence_type,omitempty" jsonschema:"Sequence type (ST) number to filter by"`
	Scheme       string `json:"scheme,omitempty"        jsonschema:"MLST scheme name filter"`
	MLSTStatus   string `json:"mlst_status,omitempty"   jsonschema:"MLST status filter: PERFECT, NOVEL, OK, MIXED, BAD, NONE, MISSING"`
	HQOnly       bool   `json:"hq_only,omitempty"       jsonschema:"Only include high-quality genomes"`
	Limit        int    `json:"limit,omitempty"         jsonschema:"Maximum results to return (default 50)"`
}

func makeMLSTHandler(dataDir string) mcp.ToolHandlerFor[mlstInput, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in mlstInput) (*mcp.CallToolResult, any, error) {
		db, err := idx.Open(dataDir)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to open index: %v", err))
		}
		defer db.Close()

		limit := in.Limit
		if limit <= 0 {
			limit = 50
		}

		rows, err := db.Query(idx.QueryParams{
			Species:      in.Species,
			HQOnly:       in.HQOnly,
			SequenceType: in.SequenceType,
			Scheme:       in.Scheme,
			MLSTStatus:   in.MLSTStatus,
			Columns: []string{
				"sample_accession",
				"sylph_species",
				"mlst_scheme",
				"mlst_st",
				"mlst_status",
				"mlst_score",
				"mlst_alleles",
			},
			Limit:    limit,
			MLSTOnly: in.MLSTStatus == "",
		})
		if err != nil {
			return errorResult(fmt.Sprintf("MLST query failed: %v", err))
		}
		if rows == nil {
			rows = []map[string]string{}
		}
		return textResult(rows)
	}
}
