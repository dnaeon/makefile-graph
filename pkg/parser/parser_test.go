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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/dnaeon/makefile-graph/fixtures"
	"gopkg.in/dnaeon/go-graph.v1"
)

func TestWithSampleDatabases(t *testing.T) {
	type testCase struct {
		desc    string
		db      string
		wantVs  int
		wantEs  int
		wantErr error
	}

	testCases := []testCase{
		{
			desc:    "GNU Make 3.81 database",
			db:      fixtures.SampleDb_v3_81,
			wantVs:  6,
			wantEs:  2,
			wantErr: nil,
		},
		{
			desc:    "GNU Make 4.4.1 database",
			db:      fixtures.SampleDb_v4_4_1,
			wantVs:  6,
			wantEs:  2,
			wantErr: nil,
		},
	}

	p := New()
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			r := strings.NewReader(tc.db)
			g, err := p.Parse(r)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got unexpected error: %s", err)
			}

			gotVs := len(g.GetVertices())
			gotEs := len(g.GetEdges())
			if tc.wantVs != gotVs {
				t.Errorf("want |V|=%d, got |V|=%d", tc.wantVs, gotVs)
			}

			if tc.wantEs != gotEs {
				t.Errorf("want |E|=%d, got |E|=%d", tc.wantEs, gotEs)
			}
		})
	}
}

func TestParseVerticesLine(t *testing.T) {
	type testCase struct {
		// A line representing a target
		line string
		// expected error
		wantErr error
		// number of expected vertices
		wantVs int
		// number of expected edges
		wantEs int
	}

	testCases := []testCase{
		{line: "foo:", wantErr: nil, wantVs: 1, wantEs: 0},
		{line: "foo: bar", wantErr: nil, wantVs: 2, wantEs: 1},
		{line: "foo bar: baz qux", wantErr: nil, wantVs: 4, wantEs: 4},
		{line: "foo", wantErr: ErrInvalidTarget},
	}

	p := New()
	for _, tc := range testCases {
		name := fmt.Sprintf("line(%s), |V|=%d, |E|=%d", tc.line, tc.wantVs, tc.wantEs)
		t.Run(name, func(t *testing.T) {
			g := graph.New[string](graph.KindDirected)
			err := p.parseVertices(g, tc.line)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("want err %s, got err %s", tc.wantErr, err)
			}
			gotVs := len(g.GetVertices())
			gotEs := len(g.GetEdges())

			if gotVs != tc.wantVs {
				t.Errorf("want |V|=%d, got |V|=%d", tc.wantVs, gotVs)
			}

			if gotEs != tc.wantEs {
				t.Errorf("want |E|=%d, got |E|=%d", tc.wantEs, gotEs)
			}
		})
	}
}
