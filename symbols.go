package symbols

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrUnsupportedLanguage is returned by [Extract] for a language with no
// extractor. Test for it with errors.Is.
var ErrUnsupportedLanguage = errors.New("symbols: unsupported language")

// CallEdge is a name-based call attribution: Caller (an enclosing function or
// method name) invokes Callee.
type CallEdge struct {
	Caller string
	Callee string
}

// MethodOwner binds a method name to the type that declares it (e.g. method
// "String" on owner "Buffer"). Lets a consumer disambiguate same-named methods
// across types.
type MethodOwner struct {
	Method string
	Owner  string
}

// FunctionSpan is a function or method definition's name, 1-based inclusive
// line span, and complexity metrics.
type FunctionSpan struct {
	Name      string
	StartLine int
	EndLine   int
	// Cyclomatic is the McCabe cyclomatic complexity (1 + branch points).
	Cyclomatic int
	// Cognitive is the SonarSource cognitive complexity, or nil when the
	// language's analyzer does not compute it. Always set for Go, and for every
	// tree-sitter language with a cognitive spec (Swift included, since
	// codemetrics v0.5.0 / gotreesitter v0.20.7).
	Cognitive *int
}

// Symbols is the result of analysing one source file.
//
// All name slices are deduplicated, first-occurrence order preserved. Fields a
// language doesn't support are nil/empty (see the per-field notes); none of
// this is an error.
type Symbols struct {
	// Functions are function and method definition names (bare, not
	// receiver-qualified — see MethodOwners for the owning type).
	Functions []string
	// Types are type / class / struct / interface / enum / trait / … names.
	Types []string
	// Imports are imported module / package paths (quotes and angle brackets
	// stripped).
	Imports []string
	// References are the bare names a file uses: call-site callees plus type
	// usages (a type named as a field/param/return/generic type). Name-based.
	References []string
	// Exported is the subset of definitions visible outside the file's
	// module/package. Computed for Go (capitalised), Python (not "_"-prefixed),
	// the keyword-visibility languages (Rust/TS/JS/Java/C#) and the
	// default-public languages (Kotlin/Scala). Nil for languages with no clear
	// rule (Ruby/PHP/C/C++/Perl/R/MATLAB/Swift).
	Exported []string
	// CallEdges attribute each call site to its innermost enclosing function.
	CallEdges []CallEdge
	// MethodOwners bind methods to their owning type, where the language nests
	// methods in a type container (most class-based languages; not C/C++).
	MethodOwners []MethodOwner
	// Package is the file's declared package / namespace, for languages that
	// declare one in source (Java/C#/Kotlin/Scala/PHP/Perl). "" otherwise.
	Package string
	// RelativeImports are imports with their leading dots preserved
	// (Python today); kept separate from Imports.
	RelativeImports []string
	// FunctionSpans are the line ranges of every named function/method.
	FunctionSpans []FunctionSpan
}

// SupportedLanguages returns every language identifier [Extract] accepts,
// sorted. It includes "go" (parsed via go/ast) and the tree-sitter languages.
func SupportedLanguages() []string {
	out := []string{"go"}
	for l := range detectFile {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// Extract analyses src as the named language and returns its symbols.
// Recognised identifiers are those from [SupportedLanguages]; "go" (alias
// "golang") routes to [ExtractGo], the rest to the tree-sitter backend. An
// unknown or unavailable language returns a wrapped [ErrUnsupportedLanguage].
//
// Extraction is best-effort: source that only partially parses yields the
// symbols recovered so far. A parse that fails or times out yields a zero
// Symbols and a nil error.
func Extract(language string, src []byte) (Symbols, error) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "go", "golang":
		return ExtractGo(src)
	default:
		s, ok := extractTreeSitter(language, src)
		if !ok {
			return Symbols{}, fmt.Errorf("%w: %q", ErrUnsupportedLanguage, language)
		}
		return s, nil
	}
}

// exportedByConvention returns the members of names whose first rune marks them
// exported under a name convention: upper-case (Go) when wantUpper, or simply
// not "_"-prefixed (Python) otherwise.
func exportedByConvention(names []string, wantUpper bool) []string {
	var out []string
	for _, n := range names {
		if wantUpper {
			if isUpperFirst(n) {
				out = append(out, n)
			}
		} else if !strings.HasPrefix(n, "_") {
			out = append(out, n)
		}
	}
	return dedupeStrings(out)
}

// dedupeStrings returns s with duplicates removed, preserving first-seen order.
// Returns nil for empty input.
func dedupeStrings(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(s))
	out := s[:0]
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// subtractStrings returns the members of all not present in remove, preserving
// order. Used to derive the exported set (defs minus non-public) for
// default-public languages.
func subtractStrings(all, remove []string) []string {
	if len(remove) == 0 {
		return all
	}
	rm := make(map[string]struct{}, len(remove))
	for _, r := range remove {
		rm[r] = struct{}{}
	}
	out := make([]string, 0, len(all))
	for _, a := range all {
		if _, ok := rm[a]; !ok {
			out = append(out, a)
		}
	}
	return out
}
