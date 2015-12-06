/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"log"
	"os"
)

var known = map[string][]string{
	"io.Closer": {"Close()"},
}

func interfaceMatching(methods map[string]struct{}) string {
	matches := func(funcs []string) bool {
		if len(methods) > len(funcs) {
			return false
		}
		for _, f := range funcs {
			if _, e := methods[f]; !e {
				return false
			}
		}
		return true
	}
	for name, funcs := range known {
		if matches(funcs) {
			return name
		}
	}
	return ""
}

func main() {
	parseFile(os.Stdin, os.Stdout)
}

func parseFile(r io.Reader, w io.Writer) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "stdin.go", r, 0)
	if err != nil {
		log.Fatal(err)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("", fset, []*ast.File{f}, nil)
	if err != nil {
		log.Fatal(err)
	}

	v := &Visitor{
		w:      w,
		fset:   fset,
		scopes: []*types.Scope{pkg.Scope()},
	}
	ast.Walk(v, f)
}

type Visitor struct {
	w      io.Writer
	fset   *token.FileSet
	scopes []*types.Scope

	nodes []ast.Node

	args map[string]types.Type
	used map[string]map[string]struct{}
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	switch x := node.(type) {
	case *ast.File:
	case *ast.FuncDecl:
		name := x.Name.Name
		scope := v.scopes[len(v.scopes)-1]
		f := scope.Lookup(name).(*types.Func)

		v.scopes = append(v.scopes, f.Scope())
		sign := f.Type().(*types.Signature)
		params := sign.Params()

		v.args = make(map[string]types.Type, params.Len())
		for i := 0; i < params.Len(); i++ {
			p := params.At(i)
			v.args[p.Name()] = p.Type()
		}
		v.used = make(map[string]map[string]struct{}, 0)
	case *ast.BlockStmt:
	case *ast.ExprStmt:
	case *ast.CallExpr:
		v.onCall(x)
	case nil:
		top := v.nodes[len(v.nodes)-1]
		v.nodes = v.nodes[:len(v.nodes)-1]
		if _, ok := top.(*ast.FuncDecl); !ok {
			return nil
		}
		v.scopes = v.scopes[:len(v.scopes)-1]
		for name, methods := range v.used {
			iface := interfaceMatching(methods)
			if iface == "" {
				continue
			}
			if iface == v.args[name].String() {
				continue
			}
			pos := v.fset.Position(top.Pos())
			fmt.Fprintf(v.w, "%s:%d: %s can be %s\n",
				pos.Filename, pos.Line, name, iface)
		}
		v.args = nil
		v.used = nil
	default:
		return nil
	}
	if node != nil {
		v.nodes = append(v.nodes, node)
	}
	return v
}

func getType(scope *types.Scope, name string) string {
	if scope == nil {
		return ""
	}
	obj := scope.Lookup(name)
	if obj == nil {
		return getType(scope.Parent(), name)
	}
	switch x := obj.(type) {
	case *types.Var:
		return x.Type().String()
	default:
		return ""
	}
}

func (v *Visitor) typeStr(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		scope := v.scopes[len(v.scopes)-1]
		return getType(scope, x.Name)
	default:
		return ""
	}
}

func (v *Visitor) onCall(c *ast.CallExpr) {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	left, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	right := sel.Sel
	vname := left.Name
	fname := right.Name
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s(", fname)
	for i, a := range c.Args {
		if i > 0 {
			fmt.Fprintf(&b, ", ")
		}
		fmt.Fprintf(&b, v.typeStr(a))
	}
	fmt.Fprintf(&b, ")")
	fulltype := b.String()
	if _, e := v.used[vname]; !e {
		v.used[vname] = make(map[string]struct{}, 0)
	}
	v.used[vname][fulltype] = struct{}{}
}
