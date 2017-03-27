// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interfacer

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/loader"
)

type cache struct {
	loader.Config

	cur pkgCache
}

type pkgCache struct {
	exp, unexp typeSet
}

type typeSet struct {
	ifaces map[string]string
	funcs  map[string]string
}

func (c *cache) isFuncType(t string) bool {
	if s := c.cur.exp.funcs[t]; s != "" {
		return true
	}
	return c.cur.unexp.funcs[t] != ""
}

func (c *cache) ifaceOf(t string) string {
	if s := c.cur.exp.ifaces[t]; s != "" {
		return s
	}
	return c.cur.unexp.ifaces[t]
}

func (c *cache) fillCache(pkg *types.Package) {
	path := pkg.Path()
	c.cur = pkgCache{
		exp: typeSet{
			ifaces: make(map[string]string),
			funcs:  make(map[string]string),
		},
		unexp: typeSet{
			ifaces: make(map[string]string),
			funcs:  make(map[string]string),
		},
	}
	addTypes := func(impPath string, ifs, funs map[string]string, top bool) {
		fullName := func(name string) string {
			if !top {
				return impPath + "." + name
			}
			return name
		}
		for iftype, name := range ifs {
			if ast.IsExported(name) {
				c.cur.exp.ifaces[iftype] = fullName(name)
			}
		}
		for ftype, name := range funs {
			if ast.IsExported(name) {
				c.cur.exp.funcs[ftype] = fullName(name)
			} else {
				c.cur.unexp.funcs[ftype] = fullName(name)
			}
		}
	}
	for _, imp := range pkg.Imports() {
		ifs, funs := fromScope(imp.Scope())
		addTypes(imp.Path(), ifs, funs, false)
	}
	ifs, funs := fromScope(pkg.Scope())
	addTypes(path, ifs, funs, true)
}
