# Ops Documentation: Complete Runnable Examples

You are updating the sparsi.ai website. Each HTML file in
`/mnt/c/Users/albert.kennis/projects/sparsi.ai/ops/` has a "Graph Config Example"
section showing a partial DAG workflow builder snippet. Your job is to replace every such section
with a "Complete Runnable Example" — a full, standalone Go program that compiles and runs.

`ai-bool.html` is already done and serves as the reference. Do not touch it.

---

## Step 1 — Discover files

Run:
```
ls /mnt/c/Users/albert.kennis/projects/sparsi.ai/ops/*.html
```

That produces the full list. Skip `ai-bool.html`. The other 68 files each need one sub-agent.

## Step 2 — Spawn sub-agents

For the first 3 of the remaining HTML files, spawn one sub-agent using the sub-agent prompt template below,
substituting the actual file path for `<FILE>`. Spawn them in parallel batches of 10. Wait
for each batch to complete and confirm success before starting the next.

---

## Sub-agent prompt template

```
You are updating one ops documentation page for the sparsi.ai website.

**Your file:** <FILE>

**Key paths:**
- Website ops dir:  /mnt/c/Users/albert.kennis/projects/sparsi.ai/ops/
- Library source:   /mnt/c/Users/albert.kennis/projects/sparsi-go/library/
- Example programs: /mnt/c/Users/albert.kennis/projects/sparsi-go/examples/
- Reference (done): /mnt/c/Users/albert.kennis/projects/sparsi.ai/ops/ai-bool.html
- dagor engine:     /mnt/c/Users/albert.kennis/projects/dagor/

---

### Your task

Replace the "Graph Config Example" section in your file with a "Complete Runnable Example"
— a full standalone Go program that compiles, runs, and demonstrates the op with realistic
hard-coded inputs. Follow these steps exactly.

---

### Step 1 — Read and understand the HTML file

Read <FILE>. Extract:
- The op name (it appears in the h1 heading and in the existing code snippet)
- Its inputs, outputs, and params (from the tables)
- A realistic use case to demonstrate

Also note whether the closing HTML around the code block is formatted (multi-line) or
minified (single line). You will need to match it precisely when replacing.

---

### Step 2 — Find the library source

Search in `/mnt/c/Users/albert.kennis/projects/sparsi-go/library/` for the Go file that
defines this op. The file names map roughly to the op category:
- `ai_ops.go` or `ai_compute_op.go` — AI ops
- `math_ops.go` — math ops
- `string_ops.go` — string ops
- `bool_ops.go` — bool ops
- `predicate_ops.go` and `routing_ops.go` — predicate/if ops
- `select_ops.go` — select/switch ops
- `slice_ops.go` — slice ops
- `time_ops.go` — time ops
- `io_ops.go` — IO ops (FileReadOp, EnvOp, HttpGetOp)
- `json_ops.go` — JSON ops
- `mode_select_op.go` — ModeSelectOp

Read the op struct to confirm:
- The exact registered name (from `operator.RegisterOp[XxxOp]()` in `init()`)
- The exact Go types for each input and output field (these determine the pointer type
  you need when calling `eng.GetOutput`)

---

### Step 3 — Write a demo program

Write the program to `/tmp/<op-slug>-demo/main.go` where `<op-slug>` is derived from the
HTML filename (e.g. `math-add.html` → `/tmp/math-add-demo/`).

**Patterns to follow** (study `/mnt/c/Users/albert.kennis/projects/sparsi.ai/ops/ai-bool.html`
for the complete reference):

#### Injecting inputs

Use `library.RegisterConst` + a matching ConstOp vertex to feed any Go value into the graph:

```go
library.RegisterConst("my_input", "hello world")

graph.NewBuilder("demo").
    Vertex("src").Op("my_input").
    Output("Result", "my_wire").
    ...
```

`RegisterConst` works for any type: `string`, `float64`, `int`, `bool`, `[]string`, etc.

#### Boilerplate

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/akennis/sparsi-go/library"
	_ "github.com/akennis/dagor/operator/builtin"

	"github.com/panjf2000/ants/v2"
	"github.com/akennis/dagor"
	"github.com/akennis/dagor/graph"
)

func main() {
	library.RegisterConst("input_val", <your value>)

	g, err := graph.NewBuilder("demo").
		Vertex("src").Op("input_val").Output("Result", "input_wire").

		Vertex("op").Op("<OpName>").
		Params(map[string]string{"key": "value"}).
		Input("<Field>", "input_wire").
		Output("<Field>", "output_wire").

		Build()
	if err != nil {
		log.Fatalf("build: %v", err)
	}

	pool, err := ants.NewPool(4)
	if err != nil {
		log.Fatalf("pool: %v", err)
	}
	defer pool.Release()

	eng, err := dagor.NewEngine(g, pool)
	if err != nil {
		log.Fatalf("engine: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := eng.Run(ctx); err != nil {
		log.Fatalf("run: %v", err)
	}

	raw, ok := eng.GetOutput("output_wire")
	if !ok {
		log.Fatal("output_wire not found")
	}
	result := *raw.(*<GoType>)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(map[string]any{"result": result})
}
```

#### Output pointer types

`eng.GetOutput` returns the pointer stored in `op.OutputFields()`. The type is always
a pointer to the op's field type:
- `bool` field → assert `*bool`
- `string` field → assert `*string`
- `float64` field → assert `*float64`
- `int` field → assert `*int`
- `[]string` field → assert `*[]string`
- `[]float64` field → assert `*[]float64`
- `map[string]string` field → assert `*map[string]string`

#### Params types

- String params: `map[string]string{"key": "value"}`
- Integer params: `map[string]int{"n": 4}`
- Mixed: look at how the op's `Setup` method calls `params.GetString` / `params.GetInt`

#### Conditional routing (when the example needs a predicate gate)

```go
import "github.com/akennis/dagor/predicate"

predicate.Register("pred_name", func(inputs map[string]any) bool {
    v, ok := inputs["wire_name"].(*bool)   // or *string, *float64, etc.
    return ok && v != nil && <condition>
})

// In the builder:
Vertex("gated").Op("SomeOp").
    Condition("pred_name").
    ConditionInput("wire_name").  // wire whose value the predicate reads
    ...
```

#### Special cases

**IO ops:**
- `FileReadOp` — inject the path string with `RegisterConst`; create a temp file in `main()`
  with `os.WriteFile` before building the graph.
- `EnvReadOp` (or `EnvOp`) — call `os.Setenv("VAR", "value")` before building the graph.
- `HttpGetOp` — use a stable public URL (e.g. `https://httpbin.org/get`).

**ModeSelectOp** — uses the Anthropic API; categories param is comma-separated string.

**Slice ops** — inject `[]string` or `[]float64` via `RegisterConst`.

**Math-pack (PackMathOperandsOp)** — packs two float64 inputs into a `MathOperands` struct;
downstream AI op consumes it. Show both together.

---

### Step 4 — Create go.mod

Write `/tmp/<op-slug>-demo/go.mod`:

```
module <op-slug>-demo

go 1.25.5

require github.com/akennis/sparsi-go v0.0.0

replace github.com/akennis/sparsi-go => /mnt/c/Users/albert.kennis/projects/sparsi-go
replace github.com/akennis/dagor => /mnt/c/Users/albert.kennis/projects/dagor
```

---

### Step 5 — Compile and run

```bash
cd /tmp/<op-slug>-demo
go mod tidy
go build -o demo .
./demo
```

If compilation fails, read the error, fix the code, and retry. Common mistakes:
- Wrong op name: verify from `operator.RegisterOp[XxxOp]()` in the library `init()`
- Wrong type assertion on `eng.GetOutput`: check the op struct's field type
- Missing `ConditionInput` when using `Condition`
- Using `map[string]any` for Params when the op's `Setup` calls `params.GetString` —
  use `map[string]string` instead

Verify the output looks correct for your input values.

---

### Step 6 — Update the HTML file

Replace the existing code-block section. The existing block always ends with
`</div>\n</section>` (possibly minified onto one line) and starts with the
`<h3 ...>Graph Config Example</h3>` heading. Read the file first to see the exact
whitespace so your replacement matches the surrounding style.

**Formatted (multi-line) files** — replace:
```html
    <h3 style="margin-top:2.5rem;margin-bottom:1rem;">Graph Config Example</h3>
    <div class="code-block">
      <div class="code-header"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-filename">graph.go</span></div>
      <pre><code>...old snippet...</code></pre>
    </div>
  </div>
</section>
```

**Minified (single-line) files** — the pattern is the same content on one line; do a
targeted find-and-replace of the `<h3>Graph Config Example</h3>...<pre><code>...` block.

In both cases, write:
```html
    <h3 style="margin-top:2.5rem;margin-bottom:1rem;">Complete Runnable Example</h3>
    <p style="color:var(--text-2);margin-bottom:1rem;">One-sentence description of what this program demonstrates.</p>
    <div class="code-block">
      <div class="code-header"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-filename">main.go</span></div>
      <pre><code>...full program...</code></pre>
    </div>
  </div>
</section>
```

**HTML escaping inside `<pre><code>`:** you must escape these characters in the Go source:
- `&` → `&amp;`
- `<` → `&lt;`
- `>` → `&gt;`

Go comparison operators (`>`, `<`, `>=`, `<=`) must be escaped. Square brackets
in `map[string]string` do NOT need escaping.

---

### Step 7 — Report back

Reply with:
- The file you updated
- Whether compilation and run succeeded
- The one-sentence description you used in the `<p>` tag
```

---

## Notes for the orchestrating session

- Spawn all sub-agents for a batch before waiting; do not spawn one at a time.
- If a sub-agent reports failure (compile error it could not fix), note the file and
  continue with the rest; do not block the batch.
- After all batches complete, report a summary: how many files updated successfully,
  and which (if any) failed.
