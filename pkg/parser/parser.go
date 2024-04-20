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

package parser

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/dnaeon/go-graph.v1"
)

// ErrInvalidTarget is an error returned by the [Parser] when trying to parse an
// invalid target.
var ErrInvalidTarget = errors.New("invalid target")

const (
	// Name of the .PHONY target
	phonyTarget = ".PHONY"

	// Marker specifying the beginning of Files section
	filesSectionMarker = "# Files"
)

// Parser is a naive parser which knows how to parse the internal GNU Make
// database.
type Parser struct{}

// New creates a new [Parser]
func New() *Parser {
	p := &Parser{}

	return p
}

// Parse parses the data from the given [io.Reader] line by line and generates a
// dependency graph for the discovered targets.
func (p *Parser) Parse(r io.Reader) (graph.Graph[string], error) {
	g := graph.New[string](graph.KindDirected)
	scanner := bufio.NewScanner(r)

	// Navigate to the `Files` section
	for scanner.Scan() {
		line := scanner.Text()
		if line == filesSectionMarker {
			break
		}
	}

	// Parse the targets
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "# Not a target:":
			// Skip next line
			scanner.Scan()
			continue
		case strings.Contains(line, " = "), strings.Contains(line, " := "):
			// Variables
			continue
		case strings.HasPrefix(line, "#"):
			// Comment
			continue
		case strings.HasPrefix(line, "\t"):
			// Recipe
			continue
		case strings.Contains(line, ":"):
			// Target
			if err := p.parseVertices(g, line); err != nil {
				return nil, err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return g, nil
}

// parseVertices parses a single line, which represents a GNU Make target with
// it's prerequisites.
func (p *Parser) parseVertices(g graph.Graph[string], line string) error {
	// A typical target with pre-requisites looks like this
	//
	// target-name: pre-req-1 pre-req-2 ...
	items := strings.Split(line, ":")
	if len(items) < 2 {
		return fmt.Errorf("%w: %s", ErrInvalidTarget, line)
	}

	// Add the vertices and edges
	fromItems := items[0]
	toItems := items[1]
	for _, u := range strings.Split(fromItems, " ") {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if u != phonyTarget {
			g.AddVertex(u)
		}

		for _, v := range strings.Split(toItems, " ") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			g.AddVertex(v)
			if u != phonyTarget {
				g.AddEdge(u, v)
			}
		}
	}

	return nil
}
