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

func TestLoadGenerateConfigDefaults(t *testing.T) {
	got, err := loadGenerateConfig(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := generateConfig{Model: defaultDashboardGenerateModel, Timeout: 45 * time.Second}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got.Enabled() {
		t.Error("Enabled() = true with no API key, want false")
	}
}

func TestLoadGenerateConfigOverrides(t *testing.T) {
	got, err := loadGenerateConfig(env(map[string]string{
		"ANTHROPIC_API_KEY":                 "  sk-test\n",
		"DASHBOARD_GENERATE_MODEL":          " claude-sonnet-5-5 ",
		"DASHBOARD_GENERATE_TIMEOUT":        "90s",
		"DASHBOARD_GENERATE_CATALOGUE_FILE": " /tmp/catalogue.md ",
		"DASHBOARD_GENERATE_BASE_URL":       " https://gateway.example.com/team/proxy ",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := generateConfig{APIKey: "sk-test", Model: "claude-sonnet-5-5", Timeout: 90 * time.Second,
		BaseURL: "https://gateway.example.com/team/proxy", CatalogueFile: "/tmp/catalogue.md"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !got.Enabled() {
		t.Error("Enabled() = false with an API key, want true")
	}
	if got.GatewayHost() != "gateway.example.com" {
		t.Errorf("GatewayHost() = %q, want only the host", got.GatewayHost())
	}
}

func TestLoadGenerateConfigBlankKeyIsDisabled(t *testing.T) {
	got, err := loadGenerateConfig(env(map[string]string{"ANTHROPIC_API_KEY": "   "}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled() {
		t.Error("Enabled() = true for a whitespace-only key, want false")
	}
}

func TestLoadGenerateConfigRejectsBadTimeout(t *testing.T) {
	for _, raw := range []string{"soon", "0s", "-5s"} {
		_, err := loadGenerateConfig(env(map[string]string{"DASHBOARD_GENERATE_TIMEOUT": raw}))
		if err == nil || !strings.Contains(err.Error(), "DASHBOARD_GENERATE_TIMEOUT") {
			t.Errorf("timeout %q: err = %v, want an error naming DASHBOARD_GENERATE_TIMEOUT", raw, err)
		}
	}
}

func TestLoadGenerateConfigDefaultGateway(t *testing.T) {
	got, err := loadGenerateConfig(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "claude-sonnet-5-5" || got.BaseURL != "" || got.GatewayHost() != "api.anthropic.com" {
		t.Errorf("got model %q, base URL %q, host %q; want claude-sonnet-5-5, none, api.anthropic.com",
			got.Model, got.BaseURL, got.GatewayHost())
	}
}

func TestLoadGenerateConfigRejectsBadBaseURL(t *testing.T) {
	for _, raw := range []string{"gateway.example.com/proxy", "ftp://gateway.example.com", "https://", "://bad", "/relative/path"} {
		_, err := loadGenerateConfig(env(map[string]string{"DASHBOARD_GENERATE_BASE_URL": raw}))
		if err == nil || !strings.Contains(err.Error(), "DASHBOARD_GENERATE_BASE_URL") {
			t.Errorf("base URL %q: err = %v, want an error naming DASHBOARD_GENERATE_BASE_URL", raw, err)
		}
	}
}
