package symbols

import (
	ts "github.com/odvcencio/gotreesitter"
)

// methodOwnerSpec describes, for one language, how to find a method's owning
// type: the method-definition node types, the enclosing container node types
// (class / struct / impl / …), and the field naming the container.
type methodOwnerSpec struct {
	methodNodes   []string
	containerNode []string
	containerName string // field on the container giving its name node
}

// methodOwnerSpecs covers the class-based languages where "a method belongs to
// a type" is a clean syntactic nesting. Languages absent here (C — no methods;
// C++ — name buried in a declarator; Perl / R / MATLAB — no modelled nesting)
// produce no owners.
var methodOwnerSpecs = map[string]methodOwnerSpec{
	"java":       {methodNodes: []string{"method_declaration", "constructor_declaration"}, containerNode: []string{"class_declaration", "interface_declaration", "enum_declaration", "record_declaration"}, containerName: "name"},
	"csharp":     {methodNodes: []string{"method_declaration", "constructor_declaration"}, containerNode: []string{"class_declaration", "struct_declaration", "interface_declaration", "record_declaration"}, containerName: "name"},
	"kotlin":     {methodNodes: []string{"function_declaration"}, containerNode: []string{"class_declaration", "object_declaration"}, containerName: "name"},
	"scala":      {methodNodes: []string{"function_definition"}, containerNode: []string{"class_definition", "object_definition", "trait_definition"}, containerName: "name"},
	"php":        {methodNodes: []string{"method_declaration"}, containerNode: []string{"class_declaration", "interface_declaration", "trait_declaration", "enum_declaration"}, containerName: "name"},
	"python":     {methodNodes: []string{"function_definition"}, containerNode: []string{"class_definition"}, containerName: "name"},
	"ruby":       {methodNodes: []string{"method"}, containerNode: []string{"class", "module"}, containerName: "name"},
	"typescript": {methodNodes: []string{"method_definition"}, containerNode: []string{"class_declaration", "class"}, containerName: "name"},
	"javascript": {methodNodes: []string{"method_definition"}, containerNode: []string{"class_declaration", "class"}, containerName: "name"},
	"swift":      {methodNodes: []string{"function_declaration"}, containerNode: []string{"class_declaration", "protocol_declaration"}, containerName: "name"},
	"rust":       {methodNodes: []string{"function_item"}, containerNode: []string{"impl_item"}, containerName: "type"},
}

// methodOwners returns method→owner bindings for every method nested in a type
// container, or nil for languages without a spec.
func methodOwners(language string, ls *langState, tree *ts.Tree, src []byte) []MethodOwner {
	spec, ok := methodOwnerSpecs[language]
	if !ok || ls.lang == nil {
		return nil
	}
	methodSet := sliceToSet(spec.methodNodes)
	containerSet := sliceToSet(spec.containerNode)

	var out []MethodOwner
	var walk func(n *ts.Node)
	walk = func(n *ts.Node) {
		if n == nil {
			return
		}
		if methodSet[n.Type(ls.lang)] {
			if name := symbolName(n, "name", src, ls.lang); name != "" {
				if owner := enclosingOwner(n, containerSet, spec.containerName, src, ls.lang); owner != "" {
					out = append(out, MethodOwner{Method: name, Owner: owner})
				}
			}
		}
		for i := 0; i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(tree.RootNode())
	return dedupeOwners(out)
}

// enclosingOwner walks up from a method node to the nearest container and
// returns its base type name, or "".
func enclosingOwner(method *ts.Node, containerSet map[string]bool, nameField string, src []byte, lang *ts.Language) string {
	for p := method.Parent(); p != nil; p = p.Parent() {
		if containerSet[p.Type(lang)] {
			return baseTypeName(symbolName(p, nameField, src, lang))
		}
	}
	return ""
}

// nameNodeTypes are the node types that name a definition when a grammar
// exposes the name as a bare child rather than a `name` field.
var nameNodeTypes = map[string]bool{
	"identifier": true, "simple_identifier": true, "type_identifier": true,
	"field_identifier": true, "constant": true, "name": true,
}

// symbolName returns a node's name: the named `field` if present, else the
// first direct child whose type is a recognised name node.
func symbolName(n *ts.Node, field string, src []byte, lang *ts.Language) string {
	if c := n.ChildByFieldName(field, lang); c != nil {
		return c.Text(src)
	}
	for i := 0; i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch != nil && nameNodeTypes[ch.Type(lang)] {
			return ch.Text(src)
		}
	}
	return ""
}

// baseTypeName strips a generic suffix and surrounding whitespace:
// "Gen<T>" / "Gen[T]" → "Gen".
func baseTypeName(s string) string {
	for i, r := range s {
		if r == '<' || r == '[' || r == ' ' || r == '\n' || r == '\t' {
			return s[:i]
		}
	}
	return s
}

func sliceToSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}
