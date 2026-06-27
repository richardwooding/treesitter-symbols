package symbols

// detectFile maps each canonical language identifier to a representative
// filename so grammars.DetectLanguage resolves the right grammar.
var detectFile = map[string]string{
	"rust":       "x.rs",
	"typescript": "x.ts",
	"javascript": "x.js",
	"ruby":       "x.rb",
	"swift":      "x.swift",
	"kotlin":     "x.kt",
	"c":          "x.c",
	"cpp":        "x.cpp",
	"python":     "x.py",
	"java":       "x.java",
	"csharp":     "x.cs",
	"php":        "x.php",
	"perl":       "x.pl",
	"r":          "x.r",
	"matlab":     "x.m",
	"scala":      "x.scala",
}

// defQuery supplements the grammar's bundled tags query for languages whose
// tags query is incomplete. Captures @function / @type; run in addition to the
// tags query (results deduped).
var defQuery = map[string]string{
	"ruby": `(method name: (identifier) @function)
(singleton_method name: (identifier) @function)
(class name: (constant) @type)
(module name: (constant) @type)`,
	"swift": `(function_declaration (simple_identifier) @function)
(protocol_function_declaration (simple_identifier) @function)
(class_declaration (type_identifier) @type)
(protocol_declaration (type_identifier) @type)`,
	"kotlin": `(function_declaration (simple_identifier) @function)
(class_declaration (type_identifier) @type)
(object_declaration (type_identifier) @type)`,
	"java": `(interface_declaration (identifier) @type)
(enum_declaration (identifier) @type)
(record_declaration (identifier) @type)`,
	"csharp": `(struct_declaration (identifier) @type)
(interface_declaration (identifier) @type)
(enum_declaration (identifier) @type)
(record_declaration (identifier) @type)`,
	"scala": `(object_definition (identifier) @type)
(trait_definition (identifier) @type)
(enum_definition (identifier) @type)`,
	"matlab": `(class_definition (identifier) @type)`,
	"php": `(class_declaration (name) @type)
(interface_declaration (name) @type)
(trait_declaration (name) @type)
(enum_declaration (name) @type)
(function_definition (name) @function)
(method_declaration (name) @function)`,
	"perl": `(subroutine_declaration_statement (bareword) @function)
(package_statement (package) @type)`,
	"r": `(binary_operator (identifier) @function (function_definition))`,
}

// importQuery captures the import path as @import. Empty/missing → imports
// left unpopulated.
var importQuery = map[string]string{
	"rust": `(use_declaration argument: (_) @import)`,
	"typescript": `(import_statement source: (string) @import)
((call_expression function: (identifier) @_f arguments: (arguments (string) @import)) (#eq? @_f "require"))`,
	"javascript": `(import_statement source: (string) @import)
((call_expression function: (identifier) @_f arguments: (arguments (string) @import)) (#eq? @_f "require"))`,
	"ruby":   `((call method: (identifier) @_m arguments: (argument_list (string) @import)) (#match? @_m "^require"))`,
	"swift":  `(import_declaration (identifier) @import)`,
	"kotlin": `(import_header (identifier) @import)`,
	"c":      `(preproc_include path: (_) @import)`,
	"cpp":    `(preproc_include path: (_) @import)`,
	"python": `(import_statement (dotted_name) @import)
(import_from_statement module_name: (dotted_name) @import)`,
	"java": `(import_declaration (scoped_identifier) @import)`,
	"csharp": `(using_directive (qualified_name) @import)
(using_directive (identifier) @import)`,
	"php":   `(namespace_use_clause (qualified_name) @import)`,
	"perl":  `(use_statement (package) @import)`,
	"r":     `((call function: (identifier) @_f arguments: (arguments (argument (identifier) @import))) (#match? @_f "^(library|require|requireNamespace)$"))`,
	"scala": `(import_declaration) @import`,
}

// refQuery captures call-site callee names as @reference. Bare names only.
var refQuery = map[string]string{
	"rust": `(call_expression function: (identifier) @reference)
(call_expression function: (scoped_identifier name: (identifier) @reference))
(call_expression function: (field_expression field: (field_identifier) @reference))
(macro_invocation macro: (identifier) @reference)`,
	"typescript": `(call_expression function: (identifier) @reference)
(call_expression function: (member_expression property: (property_identifier) @reference))`,
	"javascript": `(call_expression function: (identifier) @reference)
(call_expression function: (member_expression property: (property_identifier) @reference))`,
	"ruby": `(call method: (identifier) @reference)`,
	"swift": `(call_expression (simple_identifier) @reference)
(call_expression (navigation_expression suffix: (navigation_suffix (simple_identifier) @reference)))`,
	"kotlin": `(call_expression (simple_identifier) @reference)
(call_expression (navigation_expression (navigation_suffix (simple_identifier) @reference)))`,
	"c": `(call_expression function: (identifier) @reference)`,
	"cpp": `(call_expression function: (identifier) @reference)
(call_expression function: (field_expression field: (field_identifier) @reference))`,
	"python": `(call function: (identifier) @reference)
(call function: (attribute attribute: (identifier) @reference))`,
	"java": `(method_invocation name: (identifier) @reference)`,
	"csharp": `(invocation_expression function: (identifier) @reference)
(invocation_expression function: (member_access_expression name: (identifier) @reference))`,
	"php": `(function_call_expression (name) @reference)
(member_call_expression name: (name) @reference)
(scoped_call_expression name: (name) @reference)`,
	"perl": `(ambiguous_function_call_expression (function) @reference)
(method_call_expression (method) @reference)`,
	"r":      `(call function: (identifier) @reference)`,
	"scala":  `(call_expression (identifier) @reference)`,
	"matlab": `(function_call (identifier) @reference)`,
}

// typeRefQuery captures type USAGES (field/param/return/generic type) as
// @typeref. These join the reference set so a type used only as a field type
// counts as referenced. Languages with no static types are absent.
var typeRefQuery = map[string]string{
	"rust": `(field_declaration (type_identifier) @typeref)
(parameter (type_identifier) @typeref)
(generic_type (type_identifier) @typeref)
(type_arguments (type_identifier) @typeref)
(function_item (type_identifier) @typeref)
(let_declaration (type_identifier) @typeref)
(reference_type (type_identifier) @typeref)`,
	"typescript": `(type_annotation (type_identifier) @typeref)
(type_arguments (type_identifier) @typeref)
(new_expression constructor: (identifier) @typeref)`,
	"javascript": `(new_expression constructor: (identifier) @typeref)`,
	"ruby": `(superclass (constant) @typeref)
(call receiver: (constant) @typeref)`,
	"python": `(type (identifier) @typeref)`,
	"java": `(field_declaration (type_identifier) @typeref)
(formal_parameter (type_identifier) @typeref)
(local_variable_declaration (type_identifier) @typeref)
(method_declaration (type_identifier) @typeref)
(type_arguments (type_identifier) @typeref)
(object_creation_expression (type_identifier) @typeref)`,
	"csharp": `(variable_declaration (identifier) @typeref)`,
	"c": `(field_declaration (struct_specifier (type_identifier) @typeref))
(parameter_declaration (struct_specifier (type_identifier) @typeref))
(declaration (struct_specifier (type_identifier) @typeref))
(field_declaration (type_identifier) @typeref)
(parameter_declaration (type_identifier) @typeref)
(declaration (type_identifier) @typeref)`,
	"cpp": `(field_declaration (type_identifier) @typeref)
(parameter_declaration (type_identifier) @typeref)
(declaration (type_identifier) @typeref)
(function_definition (type_identifier) @typeref)
(template_argument_list (type_descriptor (type_identifier) @typeref))`,
	"kotlin": `(user_type (type_identifier) @typeref)`,
	"swift":  `(user_type (type_identifier) @typeref)`,
	"scala": `(class_parameter (type_identifier) @typeref)
(parameter (type_identifier) @typeref)
(function_definition (type_identifier) @typeref)
(val_definition (type_identifier) @typeref)`,
	"php": `(named_type (name) @typeref)`,
}

// exportedQuery captures the names of PUBLIC definitions as @exported, for
// keyword-visibility languages.
var exportedQuery = map[string]string{
	"rust": `(function_item (visibility_modifier) (identifier) @exported)
(struct_item (visibility_modifier) (type_identifier) @exported)
(enum_item (visibility_modifier) (type_identifier) @exported)
(trait_item (visibility_modifier) (type_identifier) @exported)
(type_item (visibility_modifier) (type_identifier) @exported)`,
	"typescript": `(export_statement (function_declaration (identifier) @exported))
(export_statement (class_declaration (type_identifier) @exported))
(export_statement (interface_declaration (type_identifier) @exported))
(export_statement (type_alias_declaration (type_identifier) @exported))
(export_statement (abstract_class_declaration (type_identifier) @exported))`,
	"javascript": `(export_statement (function_declaration (identifier) @exported))
(export_statement (class_declaration (identifier) @exported))`,
	"java": `(class_declaration (modifiers "public") name: (identifier) @exported)
(interface_declaration (modifiers "public") name: (identifier) @exported)
(enum_declaration (modifiers "public") name: (identifier) @exported)
(record_declaration (modifiers "public") name: (identifier) @exported)
(method_declaration (modifiers "public") name: (identifier) @exported)`,
	"csharp": `(class_declaration (modifier) @_m name: (identifier) @exported (#eq? @_m "public"))
(interface_declaration (modifier) @_m name: (identifier) @exported (#eq? @_m "public"))
(struct_declaration (modifier) @_m name: (identifier) @exported (#eq? @_m "public"))
(enum_declaration (modifier) @_m name: (identifier) @exported (#eq? @_m "public"))
(record_declaration (modifier) @_m name: (identifier) @exported (#eq? @_m "public"))
(method_declaration (modifier) @_m name: (identifier) @exported (#eq? @_m "public"))`,
}

// nonExportedQuery is the inverse for DEFAULT-PUBLIC languages (Kotlin/Scala):
// it captures NON-public defs (name @name + modifier @vis); the exported set is
// funcs+types minus these. Kotlin's explicit `public` is text-filtered out.
var nonExportedQuery = map[string]string{
	"kotlin": `(function_declaration (modifiers (visibility_modifier) @vis) (simple_identifier) @name)
(class_declaration (modifiers (visibility_modifier) @vis) (type_identifier) @name)
(object_declaration (modifiers (visibility_modifier) @vis) (type_identifier) @name)`,
	"scala": `(function_definition (modifiers (access_modifier) @vis) (identifier) @name)
(class_definition (modifiers (access_modifier) @vis) (identifier) @name)
(object_definition (modifiers (access_modifier) @vis) (identifier) @name)
(trait_definition (modifiers (access_modifier) @vis) (identifier) @name)`,
}

// funcSpanQuery supplements the tags query for languages whose tags don't
// expose a function span. Captures @func.def + @func.name.
var funcSpanQuery = map[string]string{
	"ruby": `(method name: (identifier) @func.name) @func.def
(singleton_method name: (identifier) @func.name) @func.def`,
	"swift":  `(function_declaration (simple_identifier) @func.name) @func.def`,
	"kotlin": `(function_declaration (simple_identifier) @func.name) @func.def`,
	"php": `(function_definition (name) @func.name) @func.def
(method_declaration (name) @func.name) @func.def`,
	"perl": `(subroutine_declaration_statement (bareword) @func.name) @func.def`,
	"r":    `(binary_operator (identifier) @func.name (function_definition)) @func.def`,
}

// packageQuery captures the file's declared package / namespace as @package.
var packageQuery = map[string]string{
	"java":   `(package_declaration [(scoped_identifier) (identifier)] @package)`,
	"csharp": `[(namespace_declaration name: (_) @package) (file_scoped_namespace_declaration name: (_) @package)]`,
	"kotlin": `(package_header (identifier) @package)`,
	"scala":  `(package_clause (package_identifier) @package)`,
	"php":    `(namespace_definition name: (namespace_name) @package)`,
	"perl":   `(package_statement (package) @package)`,
}

// relativeImportQuery captures relative imports with leading dots preserved.
var relativeImportQuery = map[string]string{
	"python": `(import_from_statement module_name: (relative_import) @import)`,
}
