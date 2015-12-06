/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
)

var known = map[string][]string{
	"io.Closer": {"Close"},
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
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "stdin.go", os.Stdin, 0)
	if err != nil {
		log.Fatal(err)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("", fset, []*ast.File{f}, nil)
	if err != nil {
		log.Fatal(err)
	}

	v := &Visitor{
		fset:  fset,
		scope: pkg.Scope(),
	}
	ast.Walk(v, f)
}

type Visitor struct {
	fset  *token.FileSet
	scope *types.Scope

	stack []ast.Node

	args map[string]types.Type
	used map[string]map[string]struct{}
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	switch x := node.(type) {
	case *ast.File:
	case *ast.FuncDecl:
		name := x.Name.Name
		f := v.scope.Lookup(name).(*types.Func)
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
		top := v.stack[len(v.stack)-1]
		v.stack = v.stack[:len(v.stack)-1]
		if _, ok := top.(*ast.FuncDecl); !ok {
			return nil
		}
		for name, methods := range v.used {
			iface := interfaceMatching(methods)
			if iface == "" {
				continue
			}
			if iface == v.args[name].String() {
				continue
			}
			pos := v.fset.Position(top.Pos())
			fmt.Printf("%s:%d: %s can be %s\n", pos.Filename, pos.Line, name, iface)
		}
		v.args = nil
		v.used = nil
	default:
		return nil
	}
	if node != nil {
		v.stack = append(v.stack, node)
	}
	return v
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
	//args := c.Args
	vname := left.Name
	fname := right.Name
	if _, e := v.used[vname]; !e {
		v.used[vname] = make(map[string]struct{}, 0)
	}
	v.used[vname][fname] = struct{}{}
}
