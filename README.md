# treesitter-symbols

[![Go Reference](https://pkg.go.dev/badge/github.com/richardwooding/treesitter-symbols.svg)](https://pkg.go.dev/github.com/richardwooding/treesitter-symbols)

Extract code symbols — **function/type definitions, imports, references, call
edges, method→owner relations, the declared package, and the exported set** —
from source files in **17 languages**, with one small API.

Go is parsed with the standard library's `go/ast`. The other 16 —
Python · JavaScript · TypeScript · Java · Rust · C · C++ · C# · Kotlin · PHP ·
Ruby · Scala · R · MATLAB · Perl · Swift — are parsed with the pure-Go
tree-sitter runtime [`gotreesitter`][gotreesitter] and its bundled grammars.

Extraction is **name-based**: references and call edges record bare callee
names, not type-resolved targets. That's intentionally lightweight — enough to
build cross-language call graphs, coupling metrics, and dead-code / unused-
export analysis without a full type checker.

## Install

```sh
go get github.com/richardwooding/treesitter-symbols
```

## Usage

```go
import symbols "github.com/richardwooding/treesitter-symbols"

s, err := symbols.Extract("rust", src) // any supported language
s, err := symbols.ExtractGo(src)       // go/ast path

fmt.Println(s.Functions)    // ["build", "greet", ...]
fmt.Println(s.Imports)      // ["std::collections::HashMap", ...]
fmt.Println(s.CallEdges)    // [{Caller:"Write" Callee:"helper"}, ...]
fmt.Println(s.MethodOwners) // [{Method:"Write" Owner:"Buffer"}, ...]
```

```go
type Symbols struct {
	Functions       []string       // function + method names
	Types           []string       // type / class / interface / enum / ...
	Imports         []string       // import paths
	References      []string       // call-site callees + type usages
	Exported        []string       // exported/public subset (see below)
	CallEdges       []CallEdge     // caller -> callee
	MethodOwners    []MethodOwner  // method -> owning type
	Package         string         // declared package / namespace
	RelativeImports []string       // relative imports, dots preserved (Python)
	FunctionSpans   []FunctionSpan // name, 1-based line range, + complexity
}

type FunctionSpan struct {
	Name       string
	StartLine  int
	EndLine    int
	Cyclomatic int   // McCabe (1 + branch points)
	Cognitive  *int  // SonarSource; nil when unavailable (Swift)
}
```

### Complexity, from the same parse

Each `FunctionSpan` carries **cyclomatic** and **cognitive** complexity,
computed by [`codemetrics`][gcm] over the *same* parse tree
(`treesitter.MetricsFromTree`) — so symbols and metrics cost a single parse, not
two. Cognitive is nil only where the analyzer has no spec (Swift).

`SupportedLanguages()` lists the 17 identifiers. An unknown language returns a
wrapped `ErrUnsupportedLanguage`. Parsing is best-effort: a partial parse yields
the symbols recovered; a failed/timed-out parse yields a zero `Symbols` and a
nil error.

### Exported set

`Exported` is computed where a language has a clear rule:

| Rule | Languages |
|---|---|
| Capitalised name | Go |
| Not `_`-prefixed | Python |
| Keyword visibility (`pub` / `export` / `public`) | Rust, TypeScript, JavaScript, Java, C# |
| Default-public minus `private`/`internal`/`protected` | Kotlin, Scala |

For the remaining languages (Ruby, PHP, C, C++, Perl, R, MATLAB, Swift)
`Exported` is nil — there's no unambiguous syntactic rule.

## Dependencies & binary size

The module requires `gotreesitter`; the **Go path** (`ExtractGo`) needs no
grammar and is unaffected. A plain build embeds every bundled grammar (~22 MB).
To embed only the languages you use, build with the gotreesitter subset tags:

```sh
go build -tags 'grammar_subset grammar_subset_rust grammar_subset_python' ./...
```

## Related

- [`codemetrics`](https://github.com/richardwooding/codemetrics) —
  cyclomatic + cognitive complexity for the same languages.

## License

MIT — see [LICENSE](LICENSE).

[gotreesitter]: https://github.com/odvcencio/gotreesitter

[gcm]: https://github.com/richardwooding/codemetrics
