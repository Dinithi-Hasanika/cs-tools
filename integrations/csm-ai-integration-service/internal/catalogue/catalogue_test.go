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

func TestBuiltInCatalogues(t *testing.T) {
	if text := DashboardEmbedded(); !strings.HasPrefix(text, "# CSM Portal dashboard catalogue") ||
		!strings.Contains(text, "## 1. Output contract") || !strings.Contains(text, "## 2. Dashboard object") {
		t.Errorf("built-in dashboard catalogue starts with %q, want the catalogue", firstLine(text))
	}
	if strings.TrimSpace(CodeEmbedded()) == "" || CodeEmbedded() == DashboardEmbedded() {
		t.Error("built-in code catalogue is empty or the dashboard one")
	}
}

func TestNoFileIsTheBuiltInCopy(t *testing.T) {
	for _, file := range []string{"", "   "} {
		d, err := NewDashboard(file)
		if err != nil {
			t.Fatalf("NewDashboard(%q): %v", file, err)
		}
		c, err := NewCode(file)
		if err != nil {
			t.Fatalf("NewCode(%q): %v", file, err)
		}
		if d.Origin() != "built-in" || c.Origin() != "built-in" {
			t.Errorf("origins = %q, %q, want built-in", d.Origin(), c.Origin())
		}
		if text, _ := d.Text(); text != DashboardEmbedded() {
			t.Error("dashboard Text() is not the built-in catalogue")
		}
		if text, _ := c.Text(); text != CodeEmbedded() {
			t.Error("code Text() is not the built-in catalogue")
		}
	}
}

func TestAMountedFileOverridesAndIsReReadOnEveryCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalogue.md")
	write(t, path, "first version")

	s, err := NewDashboard("  " + path + "  ")
	if err != nil {
		t.Fatalf("NewDashboard: %v", err)
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

func TestAMissingOrEmptyFileFailsTheStart(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.md")
	write(t, empty, " \n\t")
	for name, path := range map[string]string{"missing": filepath.Join(dir, "nope.md"), "empty": empty} {
		if _, err := NewDashboard(path); err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "dashboard catalogue") {
			t.Errorf("dashboard, %s: err = %v, want an error naming the catalogue and %s", name, err, path)
		}
		if _, err := NewCode(path); err == nil || !strings.Contains(err.Error(), "code catalogue") {
			t.Errorf("code, %s: err = %v, want an error naming the code catalogue", name, err)
		}
	}
}

func TestAFileRemovedAfterStartIsAnErrorNotTheBuiltInCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalogue.md")
	write(t, path, "mounted copy")
	s, err := NewDashboard(path)
	if err != nil {
		t.Fatalf("NewDashboard: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if text, err := s.Text(); err == nil {
		t.Errorf("Text() = %q with no error, want an error: falling back would hide the mounted copy", firstLine(text))
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
