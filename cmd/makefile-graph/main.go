// Copyright (c) 2024 Marin Atanasov Nikolov <dnaeon@gmail.com>
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
//   1. Redistributions of source code must retain the above copyright
//      notice, this list of conditions and the following disclaimer.
//   2. Redistributions in binary form must reproduce the above copyright
//      notice, this list of conditions and the following disclaimer in the
//      documentation and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/dnaeon/makefile-graph/pkg/parser"

	"gopkg.in/dnaeon/go-graph.v1"
)

var errNoTargetName = errors.New("Must specify target name")
var errInvalidLayoutDirection = errors.New("Invalid layout direction")
var errInvalidFormat = errors.New("Invalid format specified")

const (
	formatDot      = "dot"
	formatTopoSort = "tsort"
)

func main() {
	var makefile string
	var target string
	var relatedOnly bool
	var highlight bool
	var highlightColor string
	var direction string
	var format string

	flag.StringVar(&makefile, "makefile", "Makefile", "path to Makefile")
	flag.StringVar(&target, "target", "", "name of a target")
	flag.BoolVar(&highlight, "highlight", false, "highlight target and related targets")
	flag.StringVar(&highlightColor, "highlight-color", "green", "color to use for highlighting")
	flag.BoolVar(&relatedOnly, "related-only", false, "return only related vertices for a target")
	flag.StringVar(&direction, "direction", "TB", "layout direction: TB, BT, LR or RL")
	flag.StringVar(&format, "format", "dot", "format to use: dot or tsort")
	flag.Parse()

	// What format to print the graph in: Dot representation or topo sort
	formats := []string{formatDot, formatTopoSort}
	if !slices.Contains(formats, format) {
		printErrAndExit(errInvalidFormat)
	}

	// Valid directions
	directions := []string{"TB", "BT", "LR", "RL"}
	if !slices.Contains(directions, direction) {
		printErrAndExit(errInvalidLayoutDirection)
	}

	if relatedOnly && target == "" {
		printErrAndExit(errNoTargetName)
	}

	if highlight && target == "" {
		printErrAndExit(errNoTargetName)
	}

	info, err := os.Stat(makefile)
	if err != nil {
		printErrAndExit(err)
	}
	if info.IsDir() {
		printErrAndExit(fmt.Errorf("Invalid Makefile: %s is a directory", makefile))
	}

	// Dump and parse the db
	reader, err := dumpMakeDb(makefile)
	if err != nil {
		printErrAndExit(err)
	}

	p := parser.New()
	g, err := p.Parse(reader)
	if err != nil {
		printErrAndExit(err)
	}

	// Set layout direction
	attrs := g.GetDotAttributes()
	attrs["rankdir"] = direction

	// Highlight, if requested
	if highlight {
		if err := highlightVertices(g, target, highlightColor); err != nil {
			printErrAndExit(err)
		}
	}

	// Print only vertices related to the specified target
	if relatedOnly {
		if err := keepRelatedVerticesOnly(g, target); err != nil {
			printErrAndExit(err)
		}
	}

	switch format {
	case formatDot:
		if err := graph.WriteDot(g, os.Stdout); err != nil {
			printErrAndExit(err)
		}
	case formatTopoSort:
		collector := g.NewCollector()
		if err := graph.WalkTopoOrder(g, collector.WalkFunc); err != nil {
			printErrAndExit(err)
		}
		for _, v := range collector.Get() {
			fmt.Println(v.Value)
		}
	}
}

// keepRelatedVerticesOnly removes all vertices from the graph, which are not
// reachable from the given source vertex.
func keepRelatedVerticesOnly(g graph.Graph[string], source string) error {
	// A dummy walker which we use only so that we can paint the vertices.
	// The ones which remain graph.White are not related to our source
	// vertex, since they are not reachable from it.
	dummyWalker := func(*graph.Vertex[string]) error {
		return nil
	}
	if err := graph.WalkPostOrderDFS(g, source, dummyWalker); err != nil {
		return err
	}

	toRemove := make([]*graph.Vertex[string], 0)
	for _, v := range g.GetVertices() {
		if v.Color == graph.White {
			toRemove = append(toRemove, v)
		}
	}
	for _, v := range toRemove {
		g.DeleteVertex(v.Value)
	}

	return nil
}

// highlightTarget colors all vertices reachable from source with the given
// color.
func highlightVertices(g graph.Graph[string], source string, color string) error {
	walker := func(v *graph.Vertex[string]) error {
		v.DotAttributes["color"] = color
		v.DotAttributes["fillcolor"] = color
		return nil
	}

	return graph.WalkPostOrderDFS(g, source, walker)
}

// printErrAndExit prints the given error and calls [os.Exit]
func printErrAndExit(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}

// dumpMakeDb dumps the internal make(1) database and returns it
func dumpMakeDb(file string) (io.Reader, error) {
	args := []string{
		"--makefile",
		file,
		"--print-data-base",
		"--no-builtin-rules",
		"--no-builtin-variables",
		"--dry-run",
		"--always-make",
		"--question",
	}

	output, err := exec.Command("make", args...).Output()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// We ignore exit code 1 and 2 here. Exit code 1 will be
			// returned when a target is not up-to-date and usually
			// exit code 2 is returned when a pre-requisite file is
			// missing.  In both cases we can ignore the exit codes,
			// since we are interested in dumping the internal db
			// only.
			exitCode := exiterr.ExitCode()
			if exitCode != 1 && exitCode != 2 {
				return nil, err
			}
		} else {
			// Some other error occurred, bubble it up
			return nil, err
		}
	}

	r := strings.NewReader(string(output))

	return r, nil
}
