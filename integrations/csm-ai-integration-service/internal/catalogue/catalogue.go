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
// CATALOGUE_FILE. The file is read once, when the service starts (a missing
// or empty file fails the start, so a mistyped path is caught there), and
// requests use that text with no file access. A changed mount applies on the
// next start; in Choreo, changing a mount restarts the pod anyway.
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

// Source is the catalogue's text, from the built-in copy or a file read
// when it was made.
type Source struct {
	text string
	file string // "" for the built-in copy
}

// NewDashboard is the dashboard catalogue: the built-in copy when file is
// empty, else file, read now. A file that cannot be read, or is empty, is an
// error.
func NewDashboard(file string) (Source, error) {
	return newSource("dashboard catalogue", dashboardEmbedded, file)
}

// newSource is a catalogue source for file (trimmed; empty means the built-in
// copy). A file is read here, once: an empty system prompt would let the model
// invent fields, so an empty file is an error rather than a fall back to the
// built-in copy, which would hide the mounted one.
func newSource(name, embedded, file string) (Source, error) {
	file = strings.TrimSpace(file)
	if file == "" {
		return Source{text: embedded}, nil
	}
	raw, err := os.ReadFile(file) // #nosec G304 -- path is deployment configuration (CATALOGUE_FILE), not user input
	if err != nil {
		return Source{}, fmt.Errorf("read %s %q: %w", name, file, err)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return Source{}, fmt.Errorf("%s %q: %w", name, file, errEmpty)
	}
	return Source{text: string(raw), file: file}, nil
}

// Text returns the catalogue: the text read when the source was made.
func (s Source) Text() string { return s.text }

// Origin names where the catalogue comes from, for the startup log line:
// "built-in" or the file path.
func (s Source) Origin() string {
	if s.file == "" {
		return "built-in"
	}
	return s.file
}

var errEmpty = errors.New("file is empty")
