# Sparsi site

The static site for Sparsi, plus the build tooling that maintains it.

Each `*.html` page carries its shared `<nav>` and `<footer>` between marker
comments (`<!-- HEADER:START -->` … `<!-- HEADER:END -->` and the matching
`FOOTER` pair). The canonical markup lives once in `_header.html` and
`_footer.html`; the `header-update` tool stamps those partials into every page.

## header-update

`header-update/` is a deterministic Sparsi/dagor workflow that rebuilds every
page's header and footer from the shared partials. It discovers all `*.html`
pages (skipping the `_`-prefixed partials), replaces the content between the
`HEADER`/`FOOTER` markers on each, applies the correct active-nav state per
page, and reports what it touched (`updated` / `unchanged` / `skipped`). It runs
in two modes: a one-shot CLI, or a stdio MCP server exposing the workflow as the
`rebuild_site` tool.

### Build

```bash
cd header-update
go build -o header-update .
```

Requires Go 1.25+ (see `header-update/go.mod`).

### Run as a CLI

From the repository root, point it at the site directory (defaults to the
current directory):

```bash
# rebuild the pages in the repo root
./header-update/header-update -site_dir .
```

It writes updated pages in place and prints a JSON report to stdout:

```json
{
  "site_dir": ".",
  "total": 20,
  "updated": 3,
  "unchanged": 17,
  "skipped": 0,
  "pages": [ ... ]
}
```

Edit `_header.html` or `_footer.html`, rerun the command, and every page picks
up the change. Pages without the marker comments are reported as `skipped` and
left untouched.

### Run as an MCP server

Start it on stdin/stdout with `-mcp`; it exposes a single `rebuild_site` tool
that takes an optional `site_dir` argument:

```bash
./header-update/header-update -mcp
```

To register it with an MCP client (e.g. Claude Code), point the client at the
built binary with the `-mcp` flag, for example:

```json
{
  "mcpServers": {
    "header-update": {
      "command": "/absolute/path/to/header-update/header-update",
      "args": ["-mcp"]
    }
  }
}
```
