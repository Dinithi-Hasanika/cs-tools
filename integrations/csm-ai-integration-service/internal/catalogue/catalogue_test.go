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

func TestBuiltInCatalogue(t *testing.T) {
	if text := DashboardEmbedded(); !strings.HasPrefix(text, "# CSM Portal dashboard catalogue") ||
		!strings.Contains(text, "## 1. Output contract") || !strings.Contains(text, "## 2. Dashboard object") {
		t.Errorf("built-in dashboard catalogue starts with %q, want the catalogue", firstLine(text))
	}
}

// TestNoFileIsTheBuiltInCopy checks that an empty or blank file setting uses
// the built-in catalogue.
func TestNoFileIsTheBuiltInCopy(t *testing.T) {
	for _, file := range []string{"", "   "} {
		d, err := NewDashboard(file)
		if err != nil {
			t.Fatalf("NewDashboard(%q): %v", file, err)
		}
		if d.Origin() != "built-in" {
			t.Errorf("origin = %q, want built-in", d.Origin())
		}
		if d.Text() != DashboardEmbedded() {
			t.Error("dashboard Text() is not the built-in catalogue")
		}
	}
}

// TestAMountedFileOverridesAndIsReadOnceAtStart checks that a mounted file
// replaces the built-in catalogue and that an edit after start is not seen.
func TestAMountedFileOverridesAndIsReadOnceAtStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalogue.md")
	write(t, path, "first version")

	s, err := NewDashboard("  " + path + "  ")
	if err != nil {
		t.Fatalf("NewDashboard: %v", err)
	}
	if s.Origin() != path {
		t.Errorf("Origin() = %q, want the trimmed path %q", s.Origin(), path)
	}
	if s.Text() != "first version" {
		t.Fatalf("Text() = %q, want the file", s.Text())
	}
	// Requests use the text read at start, with no file access: an edit
	// applies on the next start.
	write(t, path, "edited version")
	if s.Text() != "first version" {
		t.Errorf("after an edit Text() = %q, want the text read at start", s.Text())
	}
}

func TestAMissingOrEmptyFileFailsTheStart(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.md")
	write(t, empty, " \n\t")
	for name, path := range map[string]string{"missing": filepath.Join(dir, "nope.md"), "empty": empty} {
		if _, err := NewDashboard(path); err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "dashboard catalogue") {
			t.Errorf("dashboard, %s: err = %v, want an error naming the catalogue and %s", name, err, path)
		}
	}
}

// TestAFileRemovedAfterStartKeepsTheTextReadAtStart checks that removing the
// mounted file after start keeps its text, not the built-in copy.
func TestAFileRemovedAfterStartKeepsTheTextReadAtStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalogue.md")
	write(t, path, "mounted copy")
	s, err := NewDashboard(path)
	if err != nil {
		t.Fatalf("NewDashboard: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if s.Text() != "mounted copy" {
		t.Errorf("Text() = %q, want the mounted copy read at start, not the built-in one", firstLine(s.Text()))
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
