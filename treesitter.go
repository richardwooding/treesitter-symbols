package symbols

import (
	"strings"
	"sync"

	ts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// parseTimeoutMicros caps a single tree-sitter parse. A pathological grammar
// parse (notably Swift) can run for minutes on a small file and isn't
// cancellable; this bounds it so the source yields no symbols rather than
// hanging. 5 s is far above any healthy parse (milliseconds).
const parseTimeoutMicros = 5_000_000

// langState holds the concurrent-safe machinery for one language: a ParserPool
// (safe for concurrent Parse) plus compiled queries (safe for concurrent
// Execute). Built once per language on first use.
type langState struct {
	pool           *ts.ParserPool
	lang           *ts.Language
	tagsQuery      *ts.Query
	defQuery       *ts.Query
	importQuery    *ts.Query
	refQuery       *ts.Query
	typeRefQuery   *ts.Query
	exportedQuery  *ts.Query
	nonExpQuery    *ts.Query
	spanQuery      *ts.Query
	packageQuery   *ts.Query
	relImportQuery *ts.Query
}

var (
	cacheMu sync.Mutex
	cache   = map[string]*langState{} // language -> *langState; nil = unsupported
)

// langFor lazily builds and caches the tree-sitter machinery for a language,
// or returns nil when the language isn't supported or its grammar is
// unavailable in this build.
func langFor(language string) *langState {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if ls, ok := cache[language]; ok {
		return ls
	}
	ls := buildLangState(language)
	cache[language] = ls
	return ls
}

func compile(src string, lang *ts.Language, dst **ts.Query) {
	if src == "" {
		return
	}
	if q, err := ts.NewQuery(src, lang); err == nil {
		*dst = q
	}
}

func buildLangState(language string) *langState {
	sample, ok := detectFile[language]
	if !ok {
		return nil
	}
	entry := grammars.DetectLanguage(sample)
	if entry == nil {
		return nil
	}
	lang := entry.Language()
	if lang == nil {
		return nil
	}
	ls := &langState{lang: lang, pool: ts.NewParserPool(lang, ts.WithParserPoolTimeoutMicros(parseTimeoutMicros))}
	if tagsSrc := grammars.ResolveTagsQuery(*entry); tagsSrc != "" {
		compile(tagsSrc, lang, &ls.tagsQuery)
	}
	compile(defQuery[language], lang, &ls.defQuery)
	compile(importQuery[language], lang, &ls.importQuery)
	compile(refQuery[language], lang, &ls.refQuery)
	compile(typeRefQuery[language], lang, &ls.typeRefQuery)
	compile(exportedQuery[language], lang, &ls.exportedQuery)
	compile(nonExportedQuery[language], lang, &ls.nonExpQuery)
	compile(funcSpanQuery[language], lang, &ls.spanQuery)
	compile(packageQuery[language], lang, &ls.packageQuery)
	compile(relativeImportQuery[language], lang, &ls.relImportQuery)
	return ls
}

// funcSpan is a named function definition's byte span + 1-based line span.
type funcSpan struct {
	name               string
	start, end         uint32
	startLine, endLine uint32
}

func newFuncSpan(name string, n *ts.Node) funcSpan {
	return funcSpan{
		name: name, start: n.StartByte(), end: n.EndByte(),
		startLine: n.StartPoint().Row + 1, endLine: n.EndPoint().Row + 1,
	}
}

// extractTreeSitter parses src with the language's grammar and assembles its
// Symbols. The bool is false when the language isn't tree-sitter-backed; a
// parse failure/timeout returns a zero Symbols and true (best-effort empty).
func extractTreeSitter(language string, src []byte) (Symbols, bool) {
	ls := langFor(language)
	if ls == nil {
		return Symbols{}, false
	}
	tree, err := ls.pool.Parse(src)
	if err != nil || tree == nil {
		return Symbols{}, true
	}

	functions, types, spans := collectDefs(ls, tree, src)
	imports := collectImports(ls, tree, src)
	references, edges := collectReferences(ls, tree, src, spans)
	references = append(references, collectTypeRefs(ls, tree, src)...)

	s := Symbols{
		Functions:       dedupeStrings(functions),
		Types:           dedupeStrings(types),
		Imports:         dedupeStrings(imports),
		References:      dedupeStrings(references),
		CallEdges:       edges,
		MethodOwners:    methodOwners(language, ls, tree, src),
		Package:         declaredPackage(ls, tree, src),
		RelativeImports: relativeImports(ls, tree, src),
		FunctionSpans:   toFunctionSpans(spans),
	}
	s.Exported = exportedSet(language, ls, tree, src, s.Functions, s.Types)
	return s, true
}

func toFunctionSpans(spans []funcSpan) []FunctionSpan {
	if len(spans) == 0 {
		return nil
	}
	out := make([]FunctionSpan, 0, len(spans))
	for _, s := range spans {
		out = append(out, FunctionSpan{Name: s.name, StartLine: int(s.startLine), EndLine: int(s.endLine)})
	}
	return out
}

// collectDefs gathers function / type names and function spans from the bundled
// tags query plus the supplemental def + span queries.
func collectDefs(ls *langState, tree *ts.Tree, src []byte) (functions, types []string, spans []funcSpan) {
	if ls.tagsQuery != nil {
		for _, m := range ls.tagsQuery.Execute(tree) {
			var name, kind string
			var defNode *ts.Node
			for _, c := range m.Captures {
				switch {
				case c.Name == "name":
					name = c.Text(src)
				case strings.HasPrefix(c.Name, "definition."):
					kind = c.Name[len("definition."):]
					defNode = c.Node
				}
			}
			if name == "" {
				continue
			}
			switch kind {
			case "function", "method", "macro", "constructor":
				functions = append(functions, name)
				if defNode != nil {
					spans = append(spans, newFuncSpan(name, defNode))
				}
			case "class", "struct", "interface", "enum", "trait", "type", "module", "union", "protocol", "namespace":
				types = append(types, name)
			}
		}
	}
	if ls.defQuery != nil {
		for _, m := range ls.defQuery.Execute(tree) {
			for _, c := range m.Captures {
				switch c.Name {
				case "function":
					functions = append(functions, c.Text(src))
				case "type":
					types = append(types, c.Text(src))
				}
			}
		}
	}
	if ls.spanQuery != nil {
		for _, m := range ls.spanQuery.Execute(tree) {
			var name string
			var defNode *ts.Node
			for _, c := range m.Captures {
				switch c.Name {
				case "func.name":
					name = c.Text(src)
				case "func.def":
					defNode = c.Node
				}
			}
			if name != "" && defNode != nil {
				spans = append(spans, newFuncSpan(name, defNode))
			}
		}
	}
	return functions, types, spans
}

// collectImports gathers import paths via the per-language import query.
func collectImports(ls *langState, tree *ts.Tree, src []byte) (imports []string) {
	if ls.importQuery == nil {
		return nil
	}
	for _, m := range ls.importQuery.Execute(tree) {
		for _, c := range m.Captures {
			if c.Name != "import" {
				continue
			}
			p := strings.Trim(c.Text(src), "\"'`<>")
			if p = strings.TrimSpace(strings.TrimPrefix(p, "import ")); p != "" {
				imports = append(imports, p)
			}
		}
	}
	return imports
}

// collectReferences gathers call-site callee names and attributes each to the
// innermost enclosing function span as a CallEdge.
func collectReferences(ls *langState, tree *ts.Tree, src []byte, spans []funcSpan) (references []string, edges []CallEdge) {
	if ls.refQuery == nil {
		return nil, nil
	}
	for _, m := range ls.refQuery.Execute(tree) {
		for _, c := range m.Captures {
			if c.Name != "reference" {
				continue
			}
			r := c.Text(src)
			if r == "" {
				continue
			}
			references = append(references, r)
			if caller := innermostFuncSpan(spans, c.Node.StartByte()); caller != "" {
				edges = append(edges, CallEdge{Caller: caller, Callee: r})
			}
		}
	}
	return references, dedupeEdges(edges)
}

// collectTypeRefs gathers type-usage names via the per-language type-reference
// query. These join references but are never attributed as call edges.
func collectTypeRefs(ls *langState, tree *ts.Tree, src []byte) (refs []string) {
	if ls.typeRefQuery == nil {
		return nil
	}
	for _, m := range ls.typeRefQuery.Execute(tree) {
		for _, c := range m.Captures {
			if c.Name == "typeref" {
				if r := c.Text(src); r != "" {
					refs = append(refs, r)
				}
			}
		}
	}
	return refs
}

// declaredPackage returns the package / namespace the file declares, or "".
func declaredPackage(ls *langState, tree *ts.Tree, src []byte) string {
	if ls.packageQuery == nil {
		return ""
	}
	for _, m := range ls.packageQuery.Execute(tree) {
		for _, c := range m.Captures {
			if c.Name == "package" {
				if p := strings.TrimSpace(c.Text(src)); p != "" {
					return p
				}
			}
		}
	}
	return ""
}

// relativeImports returns the file's relative imports with leading dots
// preserved, or nil.
func relativeImports(ls *langState, tree *ts.Tree, src []byte) []string {
	if ls.relImportQuery == nil {
		return nil
	}
	var out []string
	for _, m := range ls.relImportQuery.Execute(tree) {
		for _, c := range m.Captures {
			if c.Name == "import" {
				if p := strings.TrimSpace(c.Text(src)); p != "" {
					out = append(out, p)
				}
			}
		}
	}
	return dedupeStrings(out)
}

// exportedSet computes the public subset of definitions: keyword-visibility
// languages capture public defs directly; default-public languages capture the
// non-public defs and subtract them from funcs+types; Python uses the
// "_"-prefix convention. Other tree-sitter languages have no clear rule → nil.
func exportedSet(language string, ls *langState, tree *ts.Tree, src []byte, functions, types []string) []string {
	switch {
	case ls.exportedQuery != nil:
		var out []string
		for _, m := range ls.exportedQuery.Execute(tree) {
			for _, c := range m.Captures {
				if c.Name == "exported" {
					if name := c.Text(src); name != "" {
						out = append(out, name)
					}
				}
			}
		}
		return dedupeStrings(out)
	case ls.nonExpQuery != nil:
		var nonPublic []string
		for _, m := range ls.nonExpQuery.Execute(tree) {
			var name, vis string
			for _, c := range m.Captures {
				switch c.Name {
				case "name":
					name = c.Text(src)
				case "vis":
					vis = c.Text(src)
				}
			}
			// An explicit `public` (Kotlin) is still exported.
			if name != "" && !strings.HasPrefix(strings.TrimSpace(vis), "public") {
				nonPublic = append(nonPublic, name)
			}
		}
		all := make([]string, 0, len(functions)+len(types))
		all = append(all, functions...)
		all = append(all, types...)
		return dedupeStrings(subtractStrings(all, nonPublic))
	case language == "python":
		all := make([]string, 0, len(functions)+len(types))
		all = append(all, functions...)
		all = append(all, types...)
		return exportedByConvention(all, false)
	}
	return nil
}

// innermostFuncSpan returns the name of the smallest function span containing
// pos, or "" if none does.
func innermostFuncSpan(spans []funcSpan, pos uint32) string {
	best := -1
	bestSize := ^uint32(0)
	for i, s := range spans {
		if pos < s.start || pos >= s.end {
			continue
		}
		if size := s.end - s.start; size < bestSize {
			bestSize = size
			best = i
		}
	}
	if best < 0 {
		return ""
	}
	return spans[best].name
}
