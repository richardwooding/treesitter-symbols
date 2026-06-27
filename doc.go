// Package symbols extracts code symbols — function and type definitions,
// imports, references (call sites + type usages), call edges, method→owner
// relations, the declared package, and the exported set — from source files in
// 17 languages.
//
// Go is parsed with the standard library's go/ast (no external dependency for
// that path). The other 16 languages — Python, JavaScript, TypeScript, Java,
// Rust, C, C++, C#, Kotlin, PHP, Ruby, Scala, R, MATLAB, Perl, Swift — are
// parsed with the pure-Go tree-sitter runtime github.com/odvcencio/gotreesitter
// and its bundled grammars, using each grammar's tags query plus a small set of
// per-language supplemental queries.
//
// Extraction is name-based: references and call edges record bare callee names,
// not type-resolved targets. This is intentionally lightweight — enough to
// build cross-language call graphs, coupling metrics, and dead-code / unused-
// export analysis without a full type checker.
//
//	s, err := symbols.Extract("rust", src)   // any supported language
//	s, err := symbols.ExtractGo(src)         // go/ast path
//
// A plain build embeds every bundled grammar (~22 MB). To embed only the
// languages you use, build with the gotreesitter subset tags, e.g.
//
//	-tags 'grammar_subset grammar_subset_rust grammar_subset_python'
//
// (The Go path needs no grammar and is unaffected.)
package symbols
