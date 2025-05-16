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

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"gopkg.in/dnaeon/go-graph.v1"

	"github.com/dnaeon/makefile-graph/pkg/parser"
)

var errNoTargetName = errors.New("Must specify target name")
var errInvalidLayoutDirection = errors.New("Invalid layout direction")
var errInvalidFormat = errors.New("Invalid format specified")

const (
	formatDot      = "dot"
	formatTopoSort = "tsort"
	formatEcharts  = "echarts"
)

func main() {
	var relatedOnly, highlight bool
	var makefile, target, highlightColor, direction, format, theme string

	flag.StringVar(&makefile, "makefile", "Makefile", "path to Makefile")
	flag.StringVar(&target, "target", "", "name of a target")
	flag.BoolVar(&highlight, "highlight", false, "highlight target and related targets")
	flag.StringVar(&highlightColor, "highlight-color", "green", "color to use for highlighting")
	flag.BoolVar(&relatedOnly, "related-only", false, "return only related vertices for a target")
	flag.StringVar(&direction, "direction", "TB", "layout direction: TB, BT, LR or RL")
	flag.StringVar(&format, "format", "dot", "format to use: dot, tsort or echarts")
	flag.StringVar(&theme, "theme", "default", "echarts theme to use, e.g. white, dark, vintage, etc.")
	flag.Parse()

	// What format to print the graph in: Dot representation or topo sort
	formats := []string{formatDot, formatTopoSort, formatEcharts}
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

	// Set layout direction (applicable for Dot format only)
	attrs := g.GetDotAttributes()
	attrs["rankdir"] = direction

	// Highlight, if requested
	if highlight {
		if err := highlightVertices(g, target, highlightColor); err != nil {
			printErrAndExit(err)
		}
	}

	// Keep only vertices related to the specified target
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
	case formatEcharts:
		if err := writeEchartsTree(g, direction, theme, os.Stdout); err != nil {
			printErrAndExit(err)
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
		// Dot attributes
		v.DotAttributes["color"] = color
		v.DotAttributes["fillcolor"] = color

		// Echarts attributes
		v.EchartsStyle = &opts.ItemStyle{
			Color: color,
		}

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
	dir := path.Clean(path.Dir(file))
	args := []string{
		"--makefile",
		file,
		"--directory",
		dir,
		"--print-data-base",
		"--no-builtin-rules",
		"--no-builtin-variables",
		"--dry-run",
		"--always-make",
		"--question",
	}

	// Pass in the calling process environment, which might be needed when
	// evaluating dynamic targets coming from Makefile variables calling out
	// to shell. Also, sanitize the environment from LC_* vars and set
	// LC_ALL=C, so that we have deterministic output of the internal
	// database.
	env := os.Environ()
	sanitizedEnv := slices.DeleteFunc(env, func(item string) bool {
		return strings.HasPrefix(item, "LC_")
	})
	sanitizedEnv = append(sanitizedEnv, "LC_ALL=C")

	cmd := exec.Command("make", args...)
	cmd.Env = sanitizedEnv
	cmd.Dir = dir

	output, err := cmd.Output()
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

// writeEchartsTree generates a tree of the Makefile targets using echarts.
func writeEchartsTree(g graph.Graph[string], direction string, theme string, w io.Writer) error {
	// Build a map of the tree nodes and use it later for building the tree.
	nodesMap := make(map[string]*opts.TreeData)
	for _, u := range g.GetVertices() {
		node := &opts.TreeData{
			Name:       u.Label,
			SymbolSize: 15,
			ItemStyle:  u.EchartsStyle,
		}
		nodesMap[u.Label] = node
	}

	// Topowalk the graph and build the tree data
	treeData := make([]*opts.TreeData, 0)
	walker := func(u *graph.Vertex[string]) error {
		node := nodesMap[u.Label]
		children := make([]*opts.TreeData, 0)
		for _, v := range g.GetNeighbourVertices(u.Value) {
			child := nodesMap[v.Label]
			children = append(children, child)
		}
		node.Children = children

		// Add only top-level targets to the tree. Children nodes will
		// already be attached to their respective parents.
		if u.Degree.In == 0 {
			treeData = append(treeData, node)
		}

		return nil
	}
	if err := graph.WalkTopoOrder(g, walker); err != nil {
		return err
	}

	// Attach nodes to a root node.
	root := opts.TreeData{
		Name:     "Root",
		Children: treeData,
	}

	tree := charts.NewTree()
	globalOpts := []charts.GlobalOpts{
		charts.WithInitializationOpts(
			opts.Initialization{
				Width:  "100%",
				Height: "95vh",
				Theme:  theme,
			},
		),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true)}),
	}
	seriesOpts := []charts.SeriesOpts{
		charts.WithTreeOpts(
			opts.TreeChart{
				Roam:              opts.Bool(true),
				ExpandAndCollapse: opts.Bool(true),
				SymbolKeepAspect:  opts.Bool(true),
				Layout:            "orthogonal",
				Orient:            direction,
				InitialTreeDepth:  2,
				Leaves: &opts.TreeLeaves{
					Label: &opts.Label{Show: opts.Bool(true), Position: "top"},
				},
			},
		),
		charts.WithLabelOpts(opts.Label{Show: opts.Bool(true), Position: "top"}),
	}

	tree.SetGlobalOptions(globalOpts...)
	tree.AddSeries("Targets", []opts.TreeData{root}).SetSeriesOptions(seriesOpts...)
	tree.AddJSFuncStrs(`%MY_ECHARTS%.setOption({"emphasis": {"focus": "descendant"}});`)
	page := components.NewPage()
	page.AddCharts(tree)

	return page.Render(w)
}
