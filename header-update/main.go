// Command header-update is a deterministic Sparsi/dagor workflow that rebuilds
// every site page's <nav> and <footer> from the shared _header.html and
// _footer.html partials — a build step exposed as a single MCP tool.
//
// The DAG: discover the *.html pages (excluding _-prefixed partials), fan out
// one map lane per discovered page, and in each lane replace the content
// between the HEADER:START/END and FOOTER:START/END markers with the partials,
// applying the correct per-page active-nav state. A summarize node reduces the
// per-page outcomes into one report.
//
// Dual-mode: a one-shot CLI by default, or a stdio MCP server with -mcp that
// exposes the workflow as the rebuild_site tool. Every op is deterministic —
// there are no AI nodes.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/panjf2000/ants/v2"
	"github.com/akennis/dagor"
	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/graph"
	"github.com/akennis/dagor/operator"
	builtin "github.com/akennis/dagor/operator/builtin"
	"github.com/akennis/dagor/reporter"
)

// ─── Constants ─────────────────────────────────────────────────────────────

const (
	headerStart = "<!-- HEADER:START -->"
	headerEnd   = "<!-- HEADER:END -->"
	footerStart = "<!-- FOOTER:START -->"
	footerEnd   = "<!-- FOOTER:END -->"

	headerPartial = "_header.html"
	footerPartial = "_footer.html"
)

// ─── Context keys ──────────────────────────────────────────────────────────

type ctxKey string

const siteDirKey ctxKey = "site_dir"

// ─── Wire types ────────────────────────────────────────────────────────────

// PageJob is one map-lane element: a discovered page plus the shared header and
// footer templates. The templates are carried in every element because a dagor
// map sub-graph receives only its per-element wire, never a broadcast input.
type PageJob struct {
	Path   string
	Header string
	Footer string
}

// PageResult is the per-page outcome collected back from the map.
type PageResult struct {
	Path   string `json:"path"`
	Status string `json:"status"` // updated | unchanged | skipped
	Detail string `json:"detail,omitempty"`
}

// Summary is the reduced result of one workflow run.
type Summary struct {
	Total     int
	Updated   int
	Unchanged int
	Skipped   int
	Pages     []PageResult
}

// ─── ReadPartialsOp ────────────────────────────────────────────────────────

// ReadPartialsOp reads the _header.html and _footer.html partials from the
// site directory.
type ReadPartialsOp struct {
	Dir    *string `dag:"input"`
	Header string  `dag:"output"`
	Footer string  `dag:"output"`
}

func (op *ReadPartialsOp) Setup(_ *config.Params) error { return nil }
func (op *ReadPartialsOp) Reset() error                 { return nil }

func (op *ReadPartialsOp) Run(ctx context.Context) error {
	dir := resolveDir(op.Dir)
	header, err := os.ReadFile(filepath.Join(dir, headerPartial))
	if err != nil {
		return fmt.Errorf("ReadPartialsOp: read %s: %w", headerPartial, err)
	}
	footer, err := os.ReadFile(filepath.Join(dir, footerPartial))
	if err != nil {
		return fmt.Errorf("ReadPartialsOp: read %s: %w", footerPartial, err)
	}
	op.Header = strings.TrimSpace(string(header))
	op.Footer = strings.TrimSpace(string(footer))
	slog.DebugContext(ctx, "ReadPartialsOp.done", "run_id", dagor.RunID(ctx),
		"header_bytes", len(op.Header), "footer_bytes", len(op.Footer))
	return nil
}

func (op *ReadPartialsOp) InputFields() map[string]any {
	return map[string]any{"Dir": &op.Dir}
}

func (op *ReadPartialsOp) OutputFields() map[string]any {
	return map[string]any{"Header": &op.Header, "Footer": &op.Footer}
}

func (op *ReadPartialsOp) SetInputField(field string, value any) error {
	if field != "Dir" {
		return fmt.Errorf("ReadPartialsOp: unknown field %q", field)
	}
	v, ok := value.(*string)
	if !ok {
		return fmt.Errorf("ReadPartialsOp: Dir: expected *string, got %T", value)
	}
	op.Dir = v
	return nil
}

func (op *ReadPartialsOp) ResetFields() { op.Dir = nil; op.Header = ""; op.Footer = "" }

// ─── DiscoverPagesOp ───────────────────────────────────────────────────────

// DiscoverPagesOp finds every *.html page in the site directory (excluding
// _-prefixed partials) and emits one PageJob per page, threading the header and
// footer templates into each so the downstream map lanes are self-contained.
type DiscoverPagesOp struct {
	Dir    *string   `dag:"input"`
	Header *string   `dag:"input"`
	Footer *string   `dag:"input"`
	Jobs   []PageJob `dag:"output"`
}

func (op *DiscoverPagesOp) Setup(_ *config.Params) error { return nil }
func (op *DiscoverPagesOp) Reset() error                 { return nil }

func (op *DiscoverPagesOp) Run(ctx context.Context) error {
	dir := resolveDir(op.Dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("DiscoverPagesOp: read dir %s: %w", dir, err)
	}
	var header, footer string
	if op.Header != nil {
		header = *op.Header
	}
	if op.Footer != nil {
		footer = *op.Footer
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".html") || strings.HasPrefix(name, "_") {
			continue
		}
		op.Jobs = append(op.Jobs, PageJob{
			Path:   filepath.Join(dir, name),
			Header: header,
			Footer: footer,
		})
	}
	sort.Slice(op.Jobs, func(i, j int) bool { return op.Jobs[i].Path < op.Jobs[j].Path })
	slog.DebugContext(ctx, "DiscoverPagesOp.done", "run_id", dagor.RunID(ctx), "pages", len(op.Jobs))
	return nil
}

func (op *DiscoverPagesOp) InputFields() map[string]any {
	return map[string]any{"Dir": &op.Dir, "Header": &op.Header, "Footer": &op.Footer}
}

func (op *DiscoverPagesOp) OutputFields() map[string]any {
	return map[string]any{"Jobs": &op.Jobs}
}

func (op *DiscoverPagesOp) SetInputField(field string, value any) error {
	v, ok := value.(*string)
	if !ok {
		return fmt.Errorf("DiscoverPagesOp: %s: expected *string, got %T", field, value)
	}
	switch field {
	case "Dir":
		op.Dir = v
	case "Header":
		op.Header = v
	case "Footer":
		op.Footer = v
	default:
		return fmt.Errorf("DiscoverPagesOp: unknown field %q", field)
	}
	return nil
}

func (op *DiscoverPagesOp) ResetFields() {
	op.Dir = nil
	op.Header = nil
	op.Footer = nil
	op.Jobs = nil
}

// ─── RebuildPageOp ─────────────────────────────────────────────────────────

// RebuildPageOp is the map-lane operator. For one page it injects the header
// (with the page's active-nav state applied) and the footer between their
// markers, writes the file back only when the content actually changed, and
// reports the outcome.
type RebuildPageOp struct {
	Job    *PageJob   `dag:"input"`
	Result PageResult `dag:"output"`
}

func (op *RebuildPageOp) Setup(_ *config.Params) error { return nil }
func (op *RebuildPageOp) Reset() error                 { return nil }

func (op *RebuildPageOp) Run(ctx context.Context) error {
	if op.Job == nil {
		return fmt.Errorf("RebuildPageOp: nil job")
	}
	job := *op.Job
	data, err := os.ReadFile(job.Path)
	if err != nil {
		return fmt.Errorf("RebuildPageOp: read %s: %w", job.Path, err)
	}
	content := string(data)

	header := renderActive(job.Header, activeHref(job.Path))
	updated, headerOK := replaceBetween(content, headerStart, headerEnd, header)
	updated, footerOK := replaceBetween(updated, footerStart, footerEnd, job.Footer)

	switch {
	case !headerOK && !footerOK:
		op.Result = PageResult{Path: job.Path, Status: "skipped", Detail: "no HEADER/FOOTER markers"}
		return nil
	case updated == content:
		op.Result = PageResult{Path: job.Path, Status: "unchanged", Detail: missingNote(headerOK, footerOK)}
		return nil
	}
	if err := os.WriteFile(job.Path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("RebuildPageOp: write %s: %w", job.Path, err)
	}
	op.Result = PageResult{Path: job.Path, Status: "updated", Detail: missingNote(headerOK, footerOK)}
	slog.DebugContext(ctx, "RebuildPageOp.updated", "run_id", dagor.RunID(ctx), "path", job.Path)
	return nil
}

func (op *RebuildPageOp) InputFields() map[string]any {
	return map[string]any{"Job": &op.Job}
}

func (op *RebuildPageOp) OutputFields() map[string]any {
	return map[string]any{"Result": &op.Result}
}

func (op *RebuildPageOp) SetInputField(field string, value any) error {
	if field != "Job" {
		return fmt.Errorf("RebuildPageOp: unknown field %q", field)
	}
	v, ok := value.(*PageJob)
	if !ok {
		return fmt.Errorf("RebuildPageOp: Job: expected *PageJob, got %T", value)
	}
	op.Job = v
	return nil
}

func (op *RebuildPageOp) ResetFields() { op.Job = nil; op.Result = PageResult{} }

// ─── SummarizeOp ───────────────────────────────────────────────────────────

// SummarizeOp reduces the collected per-page results into one Summary.
type SummarizeOp struct {
	Results *[]any  `dag:"input"`
	Report  Summary `dag:"output"`
}

func (op *SummarizeOp) Setup(_ *config.Params) error { return nil }
func (op *SummarizeOp) Reset() error                 { return nil }

func (op *SummarizeOp) Run(ctx context.Context) error {
	if op.Results == nil {
		return nil
	}
	for _, raw := range *op.Results {
		pr, ok := raw.(PageResult)
		if !ok {
			return fmt.Errorf("SummarizeOp: unexpected result element %T", raw)
		}
		op.Report.Pages = append(op.Report.Pages, pr)
		op.Report.Total++
		switch pr.Status {
		case "updated":
			op.Report.Updated++
		case "unchanged":
			op.Report.Unchanged++
		case "skipped":
			op.Report.Skipped++
		}
	}
	slog.DebugContext(ctx, "SummarizeOp.done", "run_id", dagor.RunID(ctx),
		"total", op.Report.Total, "updated", op.Report.Updated,
		"unchanged", op.Report.Unchanged, "skipped", op.Report.Skipped)
	return nil
}

func (op *SummarizeOp) InputFields() map[string]any {
	return map[string]any{"Results": &op.Results}
}

func (op *SummarizeOp) OutputFields() map[string]any {
	return map[string]any{"Report": &op.Report}
}

func (op *SummarizeOp) SetInputField(field string, value any) error {
	if field != "Results" {
		return fmt.Errorf("SummarizeOp: unknown field %q", field)
	}
	v, ok := value.(*[]any)
	if !ok {
		return fmt.Errorf("SummarizeOp: Results: expected *[]any, got %T", value)
	}
	op.Results = v
	return nil
}

func (op *SummarizeOp) ResetFields() { op.Results = nil; op.Report = Summary{} }

// ─── Pure helpers ──────────────────────────────────────────────────────────

// resolveDir returns the site directory, defaulting to "." when unset.
func resolveDir(p *string) string {
	if p == nil || strings.TrimSpace(*p) == "" {
		return "."
	}
	return *p
}

// replaceBetween swaps the text between (but not including) start and end with
// replacement, keeping the markers. It reports whether both markers were found.
func replaceBetween(content, start, end, replacement string) (string, bool) {
	s := strings.Index(content, start)
	if s < 0 {
		return content, false
	}
	rest := strings.Index(content[s:], end)
	if rest < 0 {
		return content, false
	}
	e := s + rest
	return content[:s+len(start)] + "\n" + replacement + "\n" + content[e:], true
}

// activeHref returns the nav href that should carry class="active" for the
// given page, or "" when the page is not a top-level nav destination. Example
// pages map to Examples; the Concepts deep-dive pages map to Concepts.
func activeHref(path string) string {
	base := filepath.Base(path)
	switch {
	case strings.HasPrefix(base, "example-"):
		return "examples.html"
	case base == "index_alt.html":
		return "index.html"
	}
	switch base {
	case "dag.html", "mcp.html", "rag.html", "repair.html", "ai-routing.html":
		return "concepts.html"
	}
	switch base {
	case "index.html", "concepts.html", "examples.html", "developers.html",
		"library.html", "operations.html", "leaders.html", "releases.html":
		return base
	}
	return ""
}

// renderActive marks the matching nav link in the header template with
// class="active". The _header.html partial is stored with no active state.
func renderActive(header, active string) string {
	if active == "" {
		return header
	}
	plain := `<li><a href="` + active + `">`
	marked := `<li><a href="` + active + `" class="active">`
	return strings.Replace(header, plain, marked, 1)
}

// missingNote describes a partially-marked page for the per-page report.
func missingNote(headerOK, footerOK bool) string {
	switch {
	case !headerOK:
		return "no HEADER markers"
	case !footerOK:
		return "no FOOTER markers"
	default:
		return ""
	}
}

// ─── Registration ──────────────────────────────────────────────────────────

func init() {
	if err := operator.RegisterOpFactory("SiteDirOp", builtin.ContextValFactory[string](siteDirKey)); err != nil {
		log.Fatalf("register SiteDirOp: %v", err)
	}
	for _, reg := range []func() error{
		operator.RegisterOp[ReadPartialsOp],
		operator.RegisterOp[DiscoverPagesOp],
		operator.RegisterOp[RebuildPageOp],
		operator.RegisterOp[SummarizeOp],
	} {
		if err := reg(); err != nil {
			log.Fatalf("register custom op: %v", err)
		}
	}
}

// ─── Graph ─────────────────────────────────────────────────────────────────

// buildGraph wires the workflow:
//
//	site_dir ─► read_partials ─► discover ─► map_pages ─► summarize
//
// discover emits one PageJob per discovered page; map_pages fans out a
// RebuildPageOp lane over that array and collects the per-page results.
func buildGraph() (*graph.Graph, error) {
	b := graph.NewBuilder("header_update")

	b.Vertex("site_dir_input").Op("SiteDirOp").
		Output("Result", "site_dir")

	b.Vertex("read_partials").Op("ReadPartialsOp").
		Input("Dir", "site_dir").
		Output("Header", "header_tpl").
		Output("Footer", "footer_tpl")

	b.Vertex("discover").Op("DiscoverPagesOp").
		Input("Dir", "site_dir").
		Input("Header", "header_tpl").
		Input("Footer", "footer_tpl").
		Output("Jobs", "jobs")

	b.Vertex("map_pages").
		Input("Items", "jobs").
		MapOver("job").
		SubVertex("rebuild").
		Op("RebuildPageOp").
		Input("Job", "job").
		Output("Result", "lane_result").
		CollectInto("lane_result", "results")

	b.Vertex("summarize").Op("SummarizeOp").
		Input("Results", "results").
		Output("Report", "report")

	return b.Build()
}

// ─── Workflow boundary ─────────────────────────────────────────────────────

// UserInput is the external boundary of the workflow. In CLI mode it is filled
// from -site_dir; in MCP mode the SDK derives the tool input schema from this
// struct and deserializes + validates every tools/call request into it.
type UserInput struct {
	SiteDir string `json:"site_dir,omitempty" jsonschema:"path to the website directory holding _header.html, _footer.html and the *.html pages; optional, defaults to the current directory"`
}

// Result is the workflow's structured output: the MCP tool's typed Out and the
// CLI's stdout JSON.
type Result struct {
	SiteDir   string       `json:"site_dir"`
	Total     int          `json:"total"`
	Updated   int          `json:"updated"`
	Unchanged int          `json:"unchanged"`
	Skipped   int          `json:"skipped"`
	Pages     []PageResult `json:"pages"`
}

// runWorkflow is the single execution path shared by CLI and MCP modes. It
// builds a fresh graph + engine per call so concurrent MCP tool calls never
// share mutable operator state; the ants.Pool is safe to share.
func runWorkflow(ctx context.Context, pool *ants.Pool, in UserInput) (Result, error) {
	dir := strings.TrimSpace(in.SiteDir)
	if dir == "" {
		dir = "."
	}
	g, err := buildGraph()
	if err != nil {
		return Result{}, fmt.Errorf("build graph: %w", err)
	}
	eng, err := dagor.NewEngine(g, pool, dagor.WithReporter(reporter.New(slog.Default())))
	if err != nil {
		return Result{}, fmt.Errorf("create engine: %w", err)
	}

	ctx = context.WithValue(ctx, siteDirKey, dir)
	if err := eng.Run(ctx); err != nil {
		return Result{}, fmt.Errorf("run graph: %w", err)
	}

	res := Result{SiteDir: dir}
	if raw, ok := eng.GetOutput("report"); ok {
		if p, ok := raw.(*Summary); ok && p != nil {
			res.Total = p.Total
			res.Updated = p.Updated
			res.Unchanged = p.Unchanged
			res.Skipped = p.Skipped
			res.Pages = p.Pages
		}
	}
	return res, nil
}

// ─── MCP server mode ───────────────────────────────────────────────────────

// runMCPServer exposes the workflow as one MCP tool over stdin/stdout.
func runMCPServer(pool *ants.Pool) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "header-update",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "rebuild_site",
		Description: "Rebuild every site page's <nav> and <footer> from the shared " +
			"_header.html and _footer.html partials. For each *.html page in the site " +
			"directory it replaces the content between the HEADER:START/END and " +
			"FOOTER:START/END markers, applying the correct active-nav state per page, " +
			"and returns a per-page report (updated / unchanged / skipped).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in UserInput) (*mcp.CallToolResult, Result, error) {
		res, err := runWorkflow(ctx, pool, in)
		if err != nil {
			return nil, Result{}, err
		}
		return nil, res, nil
	})

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("mcp server: %v", err)
	}
}

// ─── Entrypoint ────────────────────────────────────────────────────────────

func main() {
	mcpMode := flag.Bool("mcp", false, "run as a stdio MCP server instead of a one-shot CLI")
	siteDir := flag.String("site_dir", ".", "website directory containing the partials and pages (CLI mode)")
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	pool, err := ants.NewPool(10)
	if err != nil {
		log.Fatalf("create pool: %v", err)
	}
	defer pool.Release()

	if *mcpMode {
		runMCPServer(pool)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	res, err := runWorkflow(ctx, pool, UserInput{SiteDir: *siteDir})
	if err != nil {
		log.Fatalf("workflow: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(res); err != nil {
		log.Fatalf("encode output: %v", err)
	}
}
