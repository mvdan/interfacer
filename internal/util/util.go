// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package util

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/types"
)

type ByAlph []string

func (l ByAlph) Len() int           { return len(l) }
func (l ByAlph) Less(i, j int) bool { return l[i] < l[j] }
func (l ByAlph) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

var exported = regexp.MustCompile(`^[A-Z]`)

type Methoder interface {
	NumMethods() int
	Method(int) *types.Func
}

func MethoderFuncMap(m Methoder) map[string]string {
	ifuncs := make(map[string]string)
	for i := 0; i < m.NumMethods(); i++ {
		f := m.Method(i)
		if !exported.MatchString(f.Name()) {
			continue
		}
		sign := f.Type().(*types.Signature)
		ifuncs[f.Name()] = SignString(sign)
	}
	return ifuncs
}

func FuncMapString(iface map[string]string) string {
	var fnames []string
	for fname := range iface {
		fnames = append(fnames, fname)
	}
	sort.Sort(ByAlph(fnames))
	var b bytes.Buffer
	for i, fname := range fnames {
		if i > 0 {
			io.WriteString(&b, "; ")
		}
		io.WriteString(&b, fname)
		io.WriteString(&b, iface[fname])
	}
	return b.String()
}

func tupleStrs(t *types.Tuple) []string {
	var l []string
	for i := 0; i < t.Len(); i++ {
		v := t.At(i)
		l = append(l, v.Type().String())
	}
	return l
}

func SignString(sign *types.Signature) string {
	ps := tupleStrs(sign.Params())
	rs := tupleStrs(sign.Results())
	return fmt.Sprintf("(%s) (%s)", strings.Join(ps, ", "), strings.Join(rs, ", "))
}

func FromScope(scope *types.Scope) (map[string]string, map[string]string) {
	ifaces := make(map[string]string)
	funcs := make(map[string]string)
	getFunc := func(sign *types.Signature) string {
		if sign.Params().Len() == 0 {
			return ""
		}
		s := SignString(sign)
		if len(s) > 160 {
			return ""
		}
		return s
	}
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		switch x := tn.Type().Underlying().(type) {
		case *types.Interface:
			iface := MethoderFuncMap(x)
			if len(iface) == 0 {
				continue
			}
			for i := 0; i < x.NumMethods(); i++ {
				f := x.Method(i)
				if _, e := iface[f.Name()]; !e {
					continue
				}
				sign := f.Type().(*types.Signature)
				if s := getFunc(sign); s != "" {
					funcs[s] = tn.Name() + "." + f.Name()
				}
			}
			s := FuncMapString(iface)
			if len(s) > 160 {
				continue
			}
			ifaces[s] = tn.Name()
		case *types.Signature:
			if s := getFunc(x); s != "" {
				funcs[s] = tn.Name()
			}
		}
	}
	return ifaces, funcs
}

func FullName(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func PkgName(fullname string) string {
	sp := strings.Split(fullname, ".")
	if len(sp) == 1 {
		return ""
	}
	return sp[0]
}
