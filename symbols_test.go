package symbols_test

import (
	"errors"
	"slices"
	"testing"

	symbols "github.com/richardwooding/treesitter-symbols"
)

func mustContain(t *testing.T, label string, got, want []string) {
	t.Helper()
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("%s: missing %q; got %v", label, w, got)
		}
	}
}

func names(spans []symbols.FunctionSpan) []string {
	out := make([]string, len(spans))
	for i, s := range spans {
		out[i] = s.Name
	}
	return out
}

func TestExtract_TreeSitter(t *testing.T) {
	cases := []struct {
		language    string
		src         string
		wantFuncs   []string
		wantTypes   []string
		wantImports []string
	}{
		{
			language: "rust",
			src: `use std::collections::HashMap;
use serde::Serialize;

pub struct Widget { name: String }
pub trait Greeter { fn greet(&self) -> String; }
pub fn build() -> Widget { Widget { name: String::new() } }
`,
			wantFuncs:   []string{"build", "greet"},
			wantTypes:   []string{"Widget", "Greeter"},
			wantImports: []string{"std::collections::HashMap", "serde::Serialize"},
		},
		{
			language: "typescript",
			src: `import { Foo } from "./foo";
import * as path from "path";

export class Service {
  handle(): void {}
}
export function run(): number { return 1; }
interface Opts { x: number }
`,
			wantFuncs:   []string{"handle", "run"},
			wantTypes:   []string{"Service", "Opts"},
			wantImports: []string{"./foo", "path"},
		},
		{
			language: "javascript",
			src: `import { Foo } from "./foo";

export class Service {
  handle() {}
}
export function run() { return 1; }
`,
			wantFuncs:   []string{"handle", "run"},
			wantTypes:   []string{"Service"},
			wantImports: []string{"./foo"},
		},
		{
			language: "ruby",
			src: `require "json"

class Widget
  def greet
    "hi"
  end
end

module Greetable
end
`,
			wantFuncs:   []string{"greet"},
			wantTypes:   []string{"Widget", "Greetable"},
			wantImports: []string{"json"},
		},
		{
			language: "swift",
			src: `import Foundation
import UIKit

class Widget {
  func greet() -> String { return "hi" }
}
struct Point { var x: Int }
protocol Greeter { func greet() -> String }
`,
			wantFuncs:   []string{"greet"},
			wantTypes:   []string{"Widget", "Point", "Greeter"},
			wantImports: []string{"Foundation", "UIKit"},
		},
		{
			language: "kotlin",
			src: `import kotlin.collections.List
import java.util.Date

object Registry

class Widget {
  fun greet(): String = "hi"
}
`,
			wantFuncs:   []string{"greet"},
			wantTypes:   []string{"Widget", "Registry"},
			wantImports: []string{"kotlin.collections.List", "java.util.Date"},
		},
		{
			language: "c",
			src: `#include <stdio.h>
#include "local.h"

struct Point { int x; };
int add(int a, int b) { return a + b; }
`,
			wantFuncs:   []string{"add"},
			wantTypes:   []string{"Point"},
			wantImports: []string{"stdio.h", "local.h"},
		},
		{
			language: "cpp",
			src: `#include <vector>
#include "widget.h"

class Widget {
public:
  void greet();
};
int main() { return 0; }
`,
			wantFuncs:   []string{"main"},
			wantTypes:   []string{"Widget"},
			wantImports: []string{"vector", "widget.h"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.language, func(t *testing.T) {
			s, err := symbols.Extract(tc.language, []byte(tc.src))
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			mustContain(t, "Functions", s.Functions, tc.wantFuncs)
			mustContain(t, "Types", s.Types, tc.wantTypes)
			mustContain(t, "Imports", s.Imports, tc.wantImports)
		})
	}
}

func TestExtract_Go(t *testing.T) {
	src := []byte(`package widget

import (
	"fmt"
	"strings"
)

type Buffer struct{ data []byte }

func New() *Buffer { return &Buffer{} }

func (b *Buffer) Write(p []byte) int {
	helper(p)
	return len(p)
}

func helper(p []byte) { fmt.Println(strings.TrimSpace("x")) }
`)
	s, err := symbols.Extract("go", src)
	if err != nil {
		t.Fatalf("Extract(go): %v", err)
	}
	if s.Package != "widget" {
		t.Errorf("Package = %q, want widget", s.Package)
	}
	mustContain(t, "Functions", s.Functions, []string{"New", "Write", "helper"})
	mustContain(t, "Types", s.Types, []string{"Buffer"})
	mustContain(t, "Imports", s.Imports, []string{"fmt", "strings"})
	mustContain(t, "References", s.References, []string{"helper", "Println", "TrimSpace"})
	mustContain(t, "Exported", s.Exported, []string{"New", "Write", "Buffer"})
	if slices.Contains(s.Exported, "helper") {
		t.Errorf("Exported should not contain unexported helper: %v", s.Exported)
	}
	// Method owner: Write belongs to Buffer.
	if !slices.Contains(s.MethodOwners, symbols.MethodOwner{Method: "Write", Owner: "Buffer"}) {
		t.Errorf("MethodOwners missing Write->Buffer; got %v", s.MethodOwners)
	}
	// Call edge: Write -> helper.
	if !slices.Contains(s.CallEdges, symbols.CallEdge{Caller: "Write", Callee: "helper"}) {
		t.Errorf("CallEdges missing Write->helper; got %v", s.CallEdges)
	}
	mustContain(t, "FunctionSpans", names(s.FunctionSpans), []string{"New", "Write", "helper"})
}

func TestExtract_GoPartialParse(t *testing.T) {
	// A trailing syntax error must not lose the well-formed decl above it.
	src := []byte("package p\n\nfunc Good() {}\n\nfunc Bad( {\n")
	s, err := symbols.Extract("go", src)
	if err != nil {
		t.Fatalf("Extract returned error on partial parse: %v", err)
	}
	if !slices.Contains(s.Functions, "Good") {
		t.Errorf("Good not recovered; got %v", s.Functions)
	}
}

func TestExportedTreeSitter(t *testing.T) {
	// Rust: only pub items are exported.
	s, err := symbols.Extract("rust", []byte("pub fn Public() {}\nfn private_fn() {}\npub struct Widget;\n"))
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "Exported", s.Exported, []string{"Public", "Widget"})
	if slices.Contains(s.Exported, "private_fn") {
		t.Errorf("private_fn must not be exported: %v", s.Exported)
	}

	// Kotlin (default-public): private members excluded, public kept.
	ks, err := symbols.Extract("kotlin", []byte("fun openFn() {}\nprivate fun hidden() {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(ks.Exported, "openFn") {
		t.Errorf("Kotlin openFn should be exported (default-public); got %v", ks.Exported)
	}
	if slices.Contains(ks.Exported, "hidden") {
		t.Errorf("Kotlin private hidden must not be exported: %v", ks.Exported)
	}
}

func TestMethodOwnersAndCallEdges_Python(t *testing.T) {
	src := []byte(`class Widget:
    def greet(self):
        return helper()

def helper():
    return 1
`)
	s, err := symbols.Extract("python", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(s.MethodOwners, symbols.MethodOwner{Method: "greet", Owner: "Widget"}) {
		t.Errorf("MethodOwners missing greet->Widget; got %v", s.MethodOwners)
	}
	if !slices.Contains(s.CallEdges, symbols.CallEdge{Caller: "greet", Callee: "helper"}) {
		t.Errorf("CallEdges missing greet->helper; got %v", s.CallEdges)
	}
	// Python exported by convention: greet/helper public, _private not.
}

func TestDeclaredPackage(t *testing.T) {
	s, err := symbols.Extract("java", []byte("package com.example.app;\nclass C { void m() {} }\n"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Package != "com.example.app" {
		t.Errorf("Package = %q, want com.example.app", s.Package)
	}
}

func spanByName(t *testing.T, spans []symbols.FunctionSpan, name string) symbols.FunctionSpan {
	t.Helper()
	for _, s := range spans {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no FunctionSpan named %q in %+v", name, spans)
	return symbols.FunctionSpan{}
}

func TestMetrics_Go(t *testing.T) {
	src := []byte(`package p

func classify(n int) string {
	if n < 0 {
		return "neg"
	} else if n == 0 {
		return "zero"
	}
	return "pos"
}
`)
	s, err := symbols.ExtractGo(src)
	if err != nil {
		t.Fatal(err)
	}
	f := spanByName(t, s.FunctionSpans, "classify")
	if f.Cyclomatic != 3 { // 1 + if + else-if
		t.Errorf("Cyclomatic = %d, want 3", f.Cyclomatic)
	}
	if f.Cognitive == nil || *f.Cognitive != 2 { // if(1) + else-if(1)
		t.Errorf("Cognitive = %v, want 2", f.Cognitive)
	}
}

func TestMetrics_TreeSitter(t *testing.T) {
	// Python branchy: cyclomatic 1+if+for+if = 4; cognitive if(1)+for(2)+if(3) = 6.
	src := "def branchy(x):\n" +
		"    if x > 0:\n" +
		"        for i in range(x):\n" +
		"            if i % 2 == 0:\n" +
		"                return i\n" +
		"    return 0\n"
	s, err := symbols.Extract("python", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	f := spanByName(t, s.FunctionSpans, "branchy")
	if f.Cyclomatic != 4 {
		t.Errorf("Cyclomatic = %d, want 4", f.Cyclomatic)
	}
	if f.Cognitive == nil || *f.Cognitive != 6 {
		t.Errorf("Cognitive = %v, want 6", f.Cognitive)
	}
}

func TestMetrics_SwiftCognitive(t *testing.T) {
	// Swift gained a cognitive spec once gotreesitter v0.20.7 fixed the else-if
	// mis-parse (codemetrics v0.5.0; file-search-on#491). A single if scores 1.
	s, err := symbols.Extract("swift", []byte("func f(_ x: Int) -> Int {\n  if x > 0 { return 1 }\n  return 0\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	f := spanByName(t, s.FunctionSpans, "f")
	if f.Cognitive == nil || *f.Cognitive != 1 {
		t.Errorf("Swift Cognitive = %v, want 1", f.Cognitive)
	}
	if f.Cyclomatic < 1 {
		t.Errorf("Swift Cyclomatic = %d, want >= 1", f.Cyclomatic)
	}
}

// TestMetrics_SwiftElseIf locks in the else-if fix (previously 0 symbols on
// gotreesitter v0.20.6): the function extracts and its else-if chain charges the
// flat continuation cost (cognitive 1+1 = 2, cyclomatic 3).
func TestMetrics_SwiftElseIf(t *testing.T) {
	s, err := symbols.Extract("swift", []byte("func f(_ x: Int) -> Int {\n  if x > 0 { return 1 } else if x < 0 { return 2 }\n  return 0\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Functions) == 0 {
		t.Fatal("no functions extracted for swift else-if (regression: gotreesitter else-if mis-parse)")
	}
	f := spanByName(t, s.FunctionSpans, "f")
	if f.Cognitive == nil || *f.Cognitive != 2 {
		t.Errorf("Swift else-if Cognitive = %v, want 2", f.Cognitive)
	}
	if f.Cyclomatic != 3 {
		t.Errorf("Swift else-if Cyclomatic = %d, want 3", f.Cyclomatic)
	}
}

func TestUnsupportedLanguage(t *testing.T) {
	_, err := symbols.Extract("cobol", []byte("x"))
	if !errors.Is(err, symbols.ErrUnsupportedLanguage) {
		t.Errorf("error = %v, want ErrUnsupportedLanguage", err)
	}
}

func TestSupportedLanguages(t *testing.T) {
	got := symbols.SupportedLanguages()
	if len(got) != 17 {
		t.Errorf("SupportedLanguages() has %d entries, want 17: %v", len(got), got)
	}
	if !slices.Contains(got, "go") {
		t.Errorf("missing go in %v", got)
	}
}

func TestReferenceSites(t *testing.T) {
	// helper() is called twice (a call ref); Widget is used as a type once.
	src := []byte(`package p

type Widget struct{ n int }

func use(w Widget) {
	helper()
	helper()
}
`)
	sites := symbols.ReferenceSites("go", src, "helper")
	if len(sites) != 0 {
		t.Errorf("go is not a tree-sitter language; want 0 sites, got %v", sites)
	}

	rsrc := []byte("fn use_it() {\n    helper();\n    helper();\n}\n")
	rs := symbols.ReferenceSites("rust", rsrc, "helper")
	calls := 0
	for _, s := range rs {
		if s.Kind == "call" {
			calls++
		}
	}
	if calls != 2 {
		t.Errorf("rust helper call sites = %d, want 2 (%v)", calls, rs)
	}
	if len(symbols.ReferenceSites("rust", rsrc, "nonexistent")) != 0 {
		t.Error("want 0 sites for an absent symbol")
	}
}
