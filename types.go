// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"bytes"
	"go/importer"
	"go/types"
	"io"
	"regexp"
)

//go:generate go run generate/std/main.go generate/std/pkgs.go

type funcSign struct {
	params  []types.Type
	results []types.Type
}

func (fs funcSign) String() string {
	var b bytes.Buffer
	io.WriteString(&b, "[")
	for _, t := range fs.params {
		io.WriteString(&b, t.String())
		io.WriteString(&b, ",")
	}
	io.WriteString(&b, "] [")
	for _, t := range fs.results {
		io.WriteString(&b, t.String())
		io.WriteString(&b, ",")
	}
	io.WriteString(&b, "]")
	return b.String()
}

type ifaceSign struct {
	name string
	t    *types.Interface

	funcs map[string]funcSign
}

type cache struct {
	imp types.Importer

	done map[string]struct{}

	// key is importPath.typeName
	stdIfaces map[string][]ifaceSign

	// TODO: do something about duplicates, especially to behave
	// deterministically if two keys map to equal ifaceSigns.
	// This is solved in stdIfaces by sorting standard libary
	// packages by length and alphabetically. We should sort the
	// flattened list of imports by depth (BFS). Right now it's a
	// DFS.
	pkgIfaces map[string][]ifaceSign

	// Useful to separate unknown packages from packages with proper
	// import paths
	nextUnknown int

	curPaths []string

	funcs map[string]funcSign
}

func typesInit() error {
	c = &cache{
		imp:       importer.Default(),
		done:      make(map[string]struct{}),
		stdIfaces: make(map[string][]ifaceSign),
		pkgIfaces: make(map[string][]ifaceSign),
		funcs:     make(map[string]funcSign),
	}
	for _, p := range pkgs {
		c.done[p.path] = struct{}{}
		if len(p.names) == 0 {
			continue
		}
		scope := types.Universe
		if p.path != "" {
			pkg, err := c.imp.Import(p.path)
			if err != nil {
				return err
			}
			scope = pkg.Scope()
		}
		c.grabNames(scope, p.path, p.names)
	}
	delete(c.done, "")
	return nil
}

func (c *cache) grabNames(scope *types.Scope, path string, names []string) {
	for _, name := range names {
		tn := scope.Lookup(name).(*types.TypeName)
		switch x := tn.Type().Underlying().(type) {
		case *types.Interface:
			c.addInterface(c.stdIfaces, path, name, x)
		case *types.Signature:
			c.addFunc(x)
		}
	}
}

func (c *cache) addInterface(m map[string][]ifaceSign, path, name string, iface *types.Interface) {
	ifsign := ifaceSign{
		name:  name,
		t:     iface,
		funcs: make(map[string]funcSign, iface.NumMethods()),
	}
	for i := 0; i < iface.NumMethods(); i++ {
		f := iface.Method(i)
		sign := f.Type().(*types.Signature)
		ifsign.funcs[f.Name()] = c.addFunc(sign)
	}
	m[path] = append(m[path], ifsign)
}

func (c *cache) addFunc(sign *types.Signature) funcSign {
	fsign := funcSign{
		params:  typeList(sign.Params()),
		results: typeList(sign.Results()),
	}
	s := fsign.String()
	if _, e := c.funcs[s]; !e {
		c.funcs[s] = fsign
	}
	return fsign
}

var exported = regexp.MustCompile(`^[A-Z]`)

func (c *cache) grabExported(scope *types.Scope, path string) {
	c.done[path] = struct{}{}
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
			c.addInterface(c.pkgIfaces, path, name, x)
		case *types.Signature:
			c.addFunc(x)
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
