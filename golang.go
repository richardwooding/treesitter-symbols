package symbols

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ExtractGo analyses Go source via the standard library's go/ast — no external
// dependency. Both top-level functions and receiver-bound methods land in
// Functions (bare names); methods also appear in MethodOwners with their
// receiver type. References combines call sites, function-value uses, and type
// usages. Exported is the capitalised subset of functions and types.
//
// Parsing is best-effort: a partial parse still yields the recovered symbols
// with a nil error; only a total parse failure (no tree) returns the error.
func ExtractGo(src []byte) (Symbols, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if f == nil {
		return Symbols{}, err
	}
	var s Symbols
	if f.Name != nil {
		s.Package = f.Name.Name
	}

	var callEdges []CallEdge
	var methodOwners []MethodOwner
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name == nil {
				continue
			}
			s.Functions = append(s.Functions, d.Name.Name)
			if d.Body != nil {
				for _, callee := range goCallees(d.Body) {
					callEdges = append(callEdges, CallEdge{Caller: d.Name.Name, Callee: callee})
				}
				s.FunctionSpans = append(s.FunctionSpans, FunctionSpan{
					Name:      d.Name.Name,
					StartLine: fset.Position(d.Pos()).Line,
					EndLine:   fset.Position(d.End()).Line,
				})
			}
			if owner := goReceiverType(d); owner != "" {
				methodOwners = append(methodOwners, MethodOwner{Method: d.Name.Name, Owner: owner})
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch sp := spec.(type) {
				case *ast.TypeSpec:
					if sp.Name != nil {
						s.Types = append(s.Types, sp.Name.Name)
					}
				case *ast.ImportSpec:
					if sp.Path != nil {
						s.Imports = append(s.Imports, strings.Trim(sp.Path.Value, `"`))
					}
				}
			}
		}
	}

	var refs []string
	refs = append(refs, goValueRefs(f)...)
	refs = append(refs, goCallRefs(f)...)
	refs = append(refs, goTypeRefs(f)...)

	s.Functions = dedupeStrings(s.Functions)
	s.Types = dedupeStrings(s.Types)
	s.Imports = dedupeStrings(s.Imports)
	s.References = dedupeStrings(refs)
	s.CallEdges = dedupeEdges(callEdges)
	s.MethodOwners = dedupeOwners(methodOwners)

	exportable := make([]string, 0, len(s.Functions)+len(s.Types))
	exportable = append(exportable, s.Functions...)
	exportable = append(exportable, s.Types...)
	s.Exported = exportedByConvention(exportable, true)
	return s, nil
}

// isUpperFirst reports whether name's first rune is upper-case (Go's export
// convention).
func isUpperFirst(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// goReceiverType returns the base receiver type name of a method (pointer and
// generic wrappers stripped: "*Stack[T]" → "Stack"), or "" for a plain func.
func goReceiverType(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	return baseExprName(fn.Recv.List[0].Type)
}

func baseExprName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return baseExprName(t.X)
	case *ast.IndexExpr:
		return baseExprName(t.X)
	case *ast.IndexListExpr:
		return baseExprName(t.X)
	}
	return ""
}

// goCallee returns the bare callee name of a call expression, or "".
func goCallee(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		if fn.Sel != nil {
			return fn.Sel.Name
		}
	}
	return ""
}

// goCallees returns every callee name reached from node (deduped).
func goCallees(node ast.Node) []string {
	var out []string
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if name := goCallee(call); name != "" {
				out = append(out, name)
			}
		}
		return true
	})
	return dedupeStrings(out)
}

// goCallRefs returns the bare callee name of every call site in the file.
func goCallRefs(f *ast.File) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if name := goCallee(call); name != "" {
				out = append(out, name)
			}
		}
		return true
	})
	return out
}

// goValueRefs captures function/method names used as VALUES — passed as a call
// argument (e.g. a handler registered via a callback) rather than called.
func goValueRefs(f *ast.File) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, arg := range call.Args {
			switch a := arg.(type) {
			case *ast.Ident:
				out = append(out, a.Name)
			case *ast.SelectorExpr:
				if a.Sel != nil {
					out = append(out, a.Sel.Name)
				}
			}
		}
		return true
	})
	return out
}

// goPredeclared is the set of Go predeclared type names, filtered from type
// references so they don't pollute the reference set.
var goPredeclared = map[string]bool{
	"bool": true, "byte": true, "rune": true, "string": true, "error": true,
	"any": true, "comparable": true, "uintptr": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
}

// goTypeRefs collects the bare names of every type used in a type position
// (field types, var/const types, composite-literal types, type assertions, and
// type-definition RHS). Predeclared types are dropped.
func goTypeRefs(f *ast.File) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Field:
			collectTypeIdents(x.Type, &out)
		case *ast.ValueSpec:
			collectTypeIdents(x.Type, &out)
		case *ast.TypeSpec:
			collectTypeIdents(x.Type, &out)
		case *ast.CompositeLit:
			collectTypeIdents(x.Type, &out)
		case *ast.TypeAssertExpr:
			collectTypeIdents(x.Type, &out)
		}
		return true
	})
	return out
}

// collectTypeIdents appends the base type name(s) of a type expression to out,
// descending through pointers, slices/arrays, maps, channels, variadics,
// parens, and generic instantiations. Struct / interface / func type literals
// contribute no name themselves — their members are visited as *ast.Field by
// the caller's ast.Inspect.
func collectTypeIdents(expr ast.Expr, out *[]string) {
	var children []ast.Expr
	switch t := expr.(type) {
	case *ast.Ident:
		if !goPredeclared[t.Name] {
			*out = append(*out, t.Name)
		}
	case *ast.SelectorExpr:
		if t.Sel != nil {
			*out = append(*out, t.Sel.Name)
		}
	case *ast.StarExpr:
		children = []ast.Expr{t.X}
	case *ast.ArrayType:
		children = []ast.Expr{t.Elt}
	case *ast.Ellipsis:
		children = []ast.Expr{t.Elt}
	case *ast.MapType:
		children = []ast.Expr{t.Key, t.Value}
	case *ast.ChanType:
		children = []ast.Expr{t.Value}
	case *ast.ParenExpr:
		children = []ast.Expr{t.X}
	case *ast.IndexExpr:
		children = []ast.Expr{t.X, t.Index}
	case *ast.IndexListExpr:
		children = append([]ast.Expr{t.X}, t.Indices...)
	}
	for _, child := range children {
		collectTypeIdents(child, out)
	}
}

func dedupeEdges(edges []CallEdge) []CallEdge {
	if len(edges) == 0 {
		return nil
	}
	seen := make(map[CallEdge]struct{}, len(edges))
	out := edges[:0]
	for _, e := range edges {
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out
}

func dedupeOwners(owners []MethodOwner) []MethodOwner {
	if len(owners) == 0 {
		return nil
	}
	seen := make(map[MethodOwner]struct{}, len(owners))
	out := owners[:0]
	for _, o := range owners {
		if _, ok := seen[o]; ok {
			continue
		}
		seen[o] = struct{}{}
		out = append(out, o)
	}
	return out
}
