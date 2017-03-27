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
	ifaces map[string]string
	funcs  map[string]string
}

func (c *cache) isFuncType(t string) bool {
	return c.cur.funcs[t] != ""
}

func (c *cache) ifaceOf(t string) string {
	return c.cur.ifaces[t]
}

func (c *cache) fillCache(pkg *types.Package) {
	path := pkg.Path()
	c.cur = pkgCache{
		ifaces: make(map[string]string),
		funcs:  make(map[string]string),
	}
	addTypes := func(impPath string, ifs, funs map[string]string, top bool) {
		fullName := func(name string) string {
			if !top {
				return impPath + "." + name
			}
			return name
		}
		for iftype, name := range ifs {
			// only suggest exported interfaces
			if ast.IsExported(name) {
				c.cur.ifaces[iftype] = fullName(name)
			}
		}
		for ftype, name := range funs {
			// ignore non-exported func signatures too
			c.cur.funcs[ftype] = fullName(name)
		}
	}
	for _, imp := range pkg.Imports() {
		ifs, funs := fromScope(imp.Scope())
		addTypes(imp.Path(), ifs, funs, false)
	}
	ifs, funs := fromScope(pkg.Scope())
	addTypes(path, ifs, funs, true)
}
