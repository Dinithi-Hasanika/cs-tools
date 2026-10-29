// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Package catalogue holds the catalogue the model is given as its system
// prompt: catalogue.md, for a dashboard definition (POST
// /ai/dashboards/generate). It tells the model what it may use, and is the
// only thing about the portal the model sees besides the user's prompt, the
// draft being edited and the team list.
//
// It ships inside the binary (embedded at build time), so it is changed and
// reviewed like code and a deployment without a file still works. It can be
// overridden by a file: in Choreo a file mount (Configs & Secrets), named by
// CATALOGUE_FILE. A file is checked when the service
// starts (a missing or empty file fails the start, so a mistyped path is
// caught there) and re-read on every request, so a changed mount applies
// without a rebuild.
package catalogue

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strings"
)

//go:embed catalogue.md
var dashboardEmbedded string

// DashboardEmbedded returns the dashboard catalogue built into the binary.
func DashboardEmbedded() string { return dashboardEmbedded }

// Source supplies the catalogue's text: the built-in copy, or a file.
type Source struct {
	name     string // for messages: "dashboard catalogue"
	embedded string
	file     string
}

// NewDashboard is the dashboard catalogue: the built-in copy when file is
// empty, else file. A file that cannot be read now is an error.
func NewDashboard(file string) (Source, error) {
	return newSource("dashboard catalogue", dashboardEmbedded, file)
}

// newSource is a catalogue source for file (trimmed; empty means the built-in
// copy), read once now so a missing or empty file fails at startup.
func newSource(name, embedded, file string) (Source, error) {
	s := Source{name: name, embedded: embedded, file: strings.TrimSpace(file)}
	if _, err := s.Text(); err != nil {
		return Source{}, err
	}
	return s, nil
}

// Text returns the catalogue. A file source is re-read each time, and a file
// that has gone or become empty is an error rather than a silent fall back to
// the built-in copy, which would hide the mounted one. An empty system prompt
// would let the model invent fields.
func (s Source) Text() (string, error) {
	if s.file == "" {
		return s.embedded, nil
	}
	raw, err := os.ReadFile(s.file) // #nosec G304 -- path is deployment configuration (CATALOGUE_FILE), not user input
	if err != nil {
		return "", fmt.Errorf("read %s %q: %w", s.name, s.file, err)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return "", fmt.Errorf("%s %q: %w", s.name, s.file, errEmpty)
	}
	return string(raw), nil
}

// Origin names where the catalogue comes from, for the startup log line:
// "built-in" or the file path.
func (s Source) Origin() string {
	if s.file == "" {
		return "built-in"
	}
	return s.file
}

var errEmpty = errors.New("file is empty")
