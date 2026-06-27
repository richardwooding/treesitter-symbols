package symbols

import (
	ts "github.com/odvcencio/gotreesitter"
)

// ReferenceSite is one positional occurrence of a symbol reference: a 1-based
// line and how the symbol is used there.
type ReferenceSite struct {
	Line int
	Kind string // "call" | "type"
}

// ReferenceSites returns every position in src where symbol is referenced — as
// a call (the reference query) or as a type usage (the type-reference query) —
// for one of the tree-sitter languages. It is the positional counterpart to
// [Symbols.References], for pinpointing a known symbol's use sites within a
// file. Returns nil for an unsupported language, an empty symbol, or a symbol
// that never appears. Sites are not deduplicated or sorted — that's the
// caller's choice.
func ReferenceSites(language string, src []byte, symbol string) []ReferenceSite {
	if symbol == "" || len(src) == 0 {
		return nil
	}
	ls := langFor(language)
	if ls == nil {
		return nil
	}
	tree, err := ls.pool.Parse(src)
	if err != nil || tree == nil {
		return nil
	}
	var out []ReferenceSite
	collect := func(q *ts.Query, capName, kind string) {
		if q == nil {
			return
		}
		for _, m := range q.Execute(tree) {
			for _, c := range m.Captures {
				if c.Name == capName && c.Text(src) == symbol {
					out = append(out, ReferenceSite{Line: int(c.Node.StartPoint().Row) + 1, Kind: kind})
				}
			}
		}
	}
	collect(ls.refQuery, "reference", "call")
	collect(ls.typeRefQuery, "typeref", "type")
	return out
}
