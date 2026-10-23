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

package catalogue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedCatalogueIsTheCasesCatalogue(t *testing.T) {
	text := Embedded()
	if !strings.HasPrefix(text, "# CSM Portal dashboard catalogue") {
		t.Fatalf("embedded catalogue starts with %q, want the catalogue heading", firstLine(text))
	}
	for _, want := range []string{"## 1. Output contract", "## 2. Dashboard object"} {
		if !strings.Contains(text, want) {
			t.Errorf("embedded catalogue has no %q section", want)
		}
	}
}

func TestNewWithNoFileIsEmbedded(t *testing.T) {
	for _, file := range []string{"", "   "} {
		s, err := New(file)
		if err != nil {
			t.Fatalf("New(%q): %v", file, err)
		}
		if s.Origin() != "embedded" {
			t.Errorf("New(%q).Origin() = %q, want embedded", file, s.Origin())
		}
		if text, _ := s.Text(); text != Embedded() {
			t.Errorf("New(%q).Text() is not the embedded catalogue", file)
		}
	}
	if text, err := (Source{}).Text(); err != nil || text != Embedded() {
		t.Errorf("zero Source: text matches embedded = %v, err = %v", text == Embedded(), err)
	}
}

func TestFileSourceIsReReadOnEveryCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalogue.md")
	write(t, path, "first version")

	s, err := New("  " + path + "  ")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.Origin() != path {
		t.Errorf("Origin() = %q, want the trimmed path %q", s.Origin(), path)
	}
	if text, _ := s.Text(); text != "first version" {
		t.Fatalf("Text() = %q, want the file", text)
	}

	write(t, path, "edited version")
	if text, _ := s.Text(); text != "edited version" {
		t.Errorf("after an edit Text() = %q, want the edited file (no rebuild or restart)", text)
	}
}

func TestNewRejectsAMissingOrEmptyFile(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.md")
	write(t, empty, " \n\t")

	for name, path := range map[string]string{
		"missing": filepath.Join(dir, "nope.md"),
		"empty":   empty,
	} {
		if _, err := New(path); err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("%s: err = %v, want an error naming %s", name, err, path)
		}
	}
}

func TestFileRemovedAfterStartupIsAnErrorNotTheEmbeddedCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalogue.md")
	write(t, path, "tuning copy")
	s, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if text, err := s.Text(); err == nil {
		t.Errorf("Text() = %q with no error, want an error: silently falling back would hide the tuning copy", firstLine(text))
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
