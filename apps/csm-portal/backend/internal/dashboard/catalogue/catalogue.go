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

// Package catalogue holds the dashboard catalogue: the text that tells the
// model which widgets, fields, filters and values a generated dashboard may
// use (POST /dashboards/generate). It is sent as the system prompt, and is the
// only thing about the portal the model sees besides the user's prompt and
// the draft being edited.
//
// The catalogue ships inside the binary (catalogue.md, embedded at build
// time), so it is changed and reviewed like code. For tuning it locally,
// DASHBOARD_GENERATE_CATALOGUE_FILE points at a file that is re-read on every
// request, so an edit applies without a rebuild.
package catalogue

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strings"
)

//go:embed catalogue.md
var embedded string

// Embedded returns the catalogue built into the binary.
func Embedded() string { return embedded }

//go:embed code_catalogue.md
var codeEmbedded string

// CodeSource supplies the experimental code dashboard catalogue
// (code_catalogue.md, POST /dashboards/generate-code): the system prompt for
// a dashboard written as JavaScript. It is kept apart from the JSON
// catalogue so the two approaches can change, or be removed, independently.
// Embedded only; there is no file override yet.
type CodeSource struct{}

// Text returns the code catalogue built into the binary.
func (CodeSource) Text() (string, error) { return codeEmbedded, nil }

// Source supplies the catalogue text. The zero value is the embedded catalogue.
type Source struct {
	file string
}

// New returns the embedded catalogue when file is empty, else a Source that
// reads file on every call to Text. A file that cannot be read now is an
// error, so a mistyped path fails at startup rather than on the first request.
func New(file string) (Source, error) {
	s := Source{file: strings.TrimSpace(file)}
	if _, err := s.Text(); err != nil {
		return Source{}, err
	}
	return s, nil
}

// Text returns the catalogue. A file source is re-read each time; an empty
// file is an error, since an empty system prompt would let the model invent
// fields.
func (s Source) Text() (string, error) {
	if s.file == "" {
		return embedded, nil
	}
	raw, err := os.ReadFile(s.file) // #nosec G304 -- path is deployment configuration, not user input
	if err != nil {
		return "", fmt.Errorf("read dashboard catalogue %q: %w", s.file, err)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return "", fmt.Errorf("dashboard catalogue %q: %w", s.file, errEmpty)
	}
	return string(raw), nil
}

// Origin names where the catalogue comes from, for the startup log line:
// "embedded" or the file path.
func (s Source) Origin() string {
	if s.file == "" {
		return "embedded"
	}
	return s.file
}

var errEmpty = errors.New("file is empty")
