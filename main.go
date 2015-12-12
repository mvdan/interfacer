/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	verbose = flag.Bool("v", false, "print the names of packages as they are checked")
)

func init() {
	if err := typesInit(); err != nil {
		errExit(err)
	}
}

func main() {
	flag.Parse()
	if err := checkPaths(flag.Args(), os.Stdout); err != nil {
		errExit(err)
	}
}

func errExit(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}
