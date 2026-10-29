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

package main

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateCodeEnabledIsOnOnlyForExactlyTrue(t *testing.T) {
	for raw, want := range map[string]bool{
		"": false, "false": false, "1": false, "TRUE": false, "True": false, "yes": false, "on": false,
		"true": true, "  true\n": true,
	} {
		if got := generateCodeEnabled(raw); got != want {
			t.Errorf("generateCodeEnabled(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestLoadGenerateCodeEnabledReadsEnv(t *testing.T) {
	t.Setenv(generateCodeFlag, "true")
	if !loadGenerateCodeEnabled() {
		t.Error("on: want true for exactly \"true\"")
	}
	t.Setenv(generateCodeFlag, "")
	if loadGenerateCodeEnabled() {
		t.Error("unset: want off")
	}
}

// The route is registered only inside the flag's branch, so a phase 1
// deployment (flag off) has no POST /dashboards/generate-code at all.
func TestTheGenerateCodeRouteIsRegisteredOnlyBehindTheFlag(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	const route = `route("POST /dashboards/generate-code"`
	if strings.Count(text, route) != 1 {
		t.Fatalf("want exactly one registration of %s", route)
	}
	before := text[:strings.Index(text, route)]
	guard := strings.LastIndex(before, "if dashboardGenerateCodeHandler != nil {")
	if guard < 0 || strings.Contains(before[guard:], "}") {
		t.Error("POST /dashboards/generate-code must be registered inside `if dashboardGenerateCodeHandler != nil {`")
	}
	if !strings.Contains(text, "if loadGenerateCodeEnabled() {") {
		t.Error("the code handler must be built only when loadGenerateCodeEnabled() is true")
	}
}
