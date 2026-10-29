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
	"strings"
	"testing"
	"time"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestLoadConfigDefaults(t *testing.T) {
	c, err := loadConfig(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Enabled() || c.Model != defaultClaudeModel || c.Timeout != defaultClaudeTimeout ||
		c.BaseURL != "" || c.CatalogueFile != "" || c.CodeCatalogueFile != "" {
		t.Errorf("defaults = %+v", c)
	}
	if c.GatewayHost() != "none" {
		t.Errorf("GatewayHost() = %q, want none with model calls off", c.GatewayHost())
	}
}

func TestLoadConfigReadsAndTrimsEveryVariable(t *testing.T) {
	c, err := loadConfig(env(map[string]string{
		"ANTHROPIC_API_KEY":   " key\n",
		"CLAUDE_MODEL":        " claude-opus-5-5 ",
		"CLAUDE_TIMEOUT":      "90s",
		"CLAUDE_BASE_URL":     " https://gateway.example.test/v1/anthropic?x=1 ",
		"CATALOGUE_FILE":      " /mnt/catalogue.md ",
		"CODE_CATALOGUE_FILE": "/mnt/code.md",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := config{APIKey: "key", Model: "claude-opus-5-5", Timeout: 90 * time.Second,
		BaseURL: "https://gateway.example.test/v1/anthropic?x=1", CatalogueFile: "/mnt/catalogue.md", CodeCatalogueFile: "/mnt/code.md"}
	if c != want {
		t.Errorf("config = %+v, want %+v", c, want)
	}
	if !c.Enabled() {
		t.Error("Enabled() = false with a key")
	}
	if c.GatewayHost() != "gateway.example.test" {
		t.Errorf("GatewayHost() = %q, want the host only", c.GatewayHost())
	}
}

func TestLoadConfigNeedsTheGatewayWithAKey(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"key, no gateway":    {"ANTHROPIC_API_KEY": "key"},
		"key, blank gateway": {"ANTHROPIC_API_KEY": "key", "CLAUDE_BASE_URL": "   "},
	} {
		_, err := loadConfig(env(vars))
		if err == nil || !strings.Contains(err.Error(), "CLAUDE_BASE_URL is required") {
			t.Errorf("%s: err = %v, want the gateway required", name, err)
		}
		if err != nil && strings.Contains(err.Error(), "key\"") {
			t.Errorf("%s: error %q holds the key", name, err)
		}
	}
	// No key at all still starts (model calls off); a gateway without a key
	// is harmless and still checked.
	if c, err := loadConfig(env(map[string]string{"CLAUDE_BASE_URL": "https://gateway.example.test"})); err != nil || c.Enabled() {
		t.Errorf("gateway without a key: config %+v err %v, want off and no error", c, err)
	}
}

func TestLoadConfigRejectsBadValues(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"timeout not a duration": {"CLAUDE_TIMEOUT": "soon"},
		"timeout zero":           {"CLAUDE_TIMEOUT": "0s"},
		"timeout negative":       {"CLAUDE_TIMEOUT": "-1s"},
		"base url relative":      {"CLAUDE_BASE_URL": "/v1"},
		"base url other scheme":  {"CLAUDE_BASE_URL": "ftp://gateway.example.test"},
		"base url no host":       {"CLAUDE_BASE_URL": "https://"},
	} {
		if _, err := loadConfig(env(vars)); err == nil {
			t.Errorf("%s: no error", name)
		} else if strings.Contains(err.Error(), "key") {
			t.Errorf("%s: error %q mentions the key", name, err)
		}
	}
}

func TestWriteTimeoutLeavesRoomForTheModelCall(t *testing.T) {
	for model, want := range map[time.Duration]time.Duration{
		45 * time.Second: 2 * time.Minute,
		90 * time.Second: 2*time.Minute + 30*time.Second,
	} {
		if got := writeTimeout(model); got != want {
			t.Errorf("writeTimeout(%v) = %v, want %v", model, got, want)
		}
	}
}

func TestCodeGenerationIsOnOnlyForExactlyTrue(t *testing.T) {
	for raw, want := range map[string]bool{
		"": false, "false": false, "1": false, "TRUE": false, "True": false, "yes": false, "on": false,
		"true": true, " true ": true,
	} {
		c, err := loadConfig(env(map[string]string{"DASHBOARD_GENERATE_CODE_ENABLED": raw}))
		if err != nil {
			t.Fatal(err)
		}
		if c.CodeGenerationEnabled != want {
			t.Errorf("DASHBOARD_GENERATE_CODE_ENABLED=%q: enabled %v, want %v", raw, c.CodeGenerationEnabled, want)
		}
	}
}
