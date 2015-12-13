// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"go/importer"
	"go/types"
	"regexp"
)

//go:generate go run generate/std/main.go generate/std/pkgs.go

type funcSign struct {
	params  []types.Type
	results []types.Type
}

type ifaceSign struct {
	name string
	t    *types.Interface

	funcs map[string]funcSign
}

type cache struct {
	done map[string]struct{}

	// key is importPath.typeName
	// TODO: do something about duplicates, especially to behave
	// deterministically if two keys map to equal ifaceSigns.
	stdIfaces map[string][]ifaceSign

	pkgIfaces map[string][]ifaceSign

	curPaths []string

	// TODO: avoid duplicates
	funcs []funcSign
}

func typesInit() error {
	c = &cache{
		done:      make(map[string]struct{}),
		stdIfaces: make(map[string][]ifaceSign),
		pkgIfaces: make(map[string][]ifaceSign),
	}
	imp := importer.Default()
	for path, names := range pkgs {
		pkg, err := imp.Import(path)
		if err != nil {
			return err
		}
		c.grabNames(pkg.Scope(), path, names)
	}
	c.grabNames(types.Universe, "", []string{"error"})
	delete(c.done, "")
	return nil
}

func (c *cache) grabNames(scope *types.Scope, path string, names []string) {
	pkgs := c.stdIfaces
	c.done[path] = struct{}{}
	for _, name := range names {
		tn := scope.Lookup(name).(*types.TypeName)
		switch x := tn.Type().Underlying().(type) {
		case *types.Interface:
			ifsign := ifaceSign{
				name:  name,
				t:     x,
				funcs: make(map[string]funcSign, x.NumMethods()),
			}
			for i := 0; i < x.NumMethods(); i++ {
				f := x.Method(i)
				sign := f.Type().(*types.Signature)
				fsign := funcSign{
					params:  typeList(sign.Params()),
					results: typeList(sign.Results()),
				}
				c.funcs = append(c.funcs, fsign)
				ifsign.funcs[f.Name()] = fsign
			}
			pkgs[path] = append(pkgs[path], ifsign)
		case *types.Signature:
			fsign := funcSign{
				params:  typeList(x.Params()),
				results: typeList(x.Results()),
			}
			c.funcs = append(c.funcs, fsign)
		}
	}
}

var exported = regexp.MustCompile(`^[A-Z]`)

func (c *cache) grabFromScope(scope *types.Scope, impPath string) {
	pkgs := c.pkgIfaces
	c.done[impPath] = struct{}{}
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		if !exported.MatchString(tn.Name()) {
			continue
		}
		switch x := tn.Type().Underlying().(type) {
		case *types.Interface:
			if x.NumMethods() == 0 {
				continue
			}
			ifsign := ifaceSign{
				name:  name,
				t:     x,
				funcs: make(map[string]funcSign, x.NumMethods()),
			}
			for i := 0; i < x.NumMethods(); i++ {
				f := x.Method(i)
				sign := f.Type().(*types.Signature)
				fsign := funcSign{
					params:  typeList(sign.Params()),
					results: typeList(sign.Results()),
				}
				c.funcs = append(c.funcs, fsign)
				ifsign.funcs[f.Name()] = fsign
			}
			pkgs[impPath] = append(pkgs[impPath], ifsign)
		case *types.Signature:
			fsign := funcSign{
				params:  typeList(x.Params()),
				results: typeList(x.Results()),
			}
			c.funcs = append(c.funcs, fsign)
		}
	}
}

func typeList(t *types.Tuple) []types.Type {
	var l []types.Type
	for i := 0; i < t.Len(); i++ {
		v := t.At(i)
		l = append(l, v.Type())
	}
	return l
}
