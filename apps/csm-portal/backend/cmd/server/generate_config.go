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
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	defaultDashboardGenerateModel   = "claude-sonnet-5-5"
	defaultDashboardGenerateTimeout = 45 * time.Second
)

// generateConfig configures POST /dashboards/generate's model call.
//
// APIKey is a secret: never log it. An empty APIKey means generation is off
// and the endpoint keeps answering with its sample dashboard, so a local stack
// without a key still works. Nothing reads this yet beyond the startup log
// line; the Claude call that uses it is a later step.
//
// BaseURL, when set, sends the model calls to an Anthropic-compatible gateway
// instead of the Anthropic API (the key then belongs to that gateway). It is
// deployment configuration: keep internal addresses out of committed files.
//
// CatalogueFile, when set, replaces the catalogue built into the binary with a
// file re-read on every request (local tuning only; see package catalogue).
type generateConfig struct {
	APIKey        string
	Model         string
	Timeout       time.Duration
	BaseURL       string
	CatalogueFile string
}

// GatewayHost is the host model calls go to, for the startup log line:
// BaseURL's host, or "api.anthropic.com" when no BaseURL is set. Never the
// full URL, so a path or query carrying anything sensitive stays out of logs.
func (c generateConfig) GatewayHost() string {
	if c.BaseURL == "" {
		return "api.anthropic.com"
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return "invalid"
	}
	return u.Host
}

// Enabled reports whether a model call can be made at all.
func (c generateConfig) Enabled() bool { return c.APIKey != "" }

// loadGenerateConfig reads ANTHROPIC_API_KEY, DASHBOARD_GENERATE_MODEL
// (default defaultDashboardGenerateModel), DASHBOARD_GENERATE_TIMEOUT (a Go
// duration, default defaultDashboardGenerateTimeout, must be > 0) and
// DASHBOARD_GENERATE_BASE_URL (optional; an absolute http or https URL) and
// DASHBOARD_GENERATE_CATALOGUE_FILE (optional). Values are
// trimmed, so a key pasted with a trailing newline still works. Pure, like
// loadTimeouts, so tests pass their own getenv.
func loadGenerateConfig(getenv func(string) string) (generateConfig, error) {
	timeout, err := durationEnv(getenv, "DASHBOARD_GENERATE_TIMEOUT", defaultDashboardGenerateTimeout)
	if err != nil {
		return generateConfig{}, err
	}
	baseURL := strings.TrimSpace(getenv("DASHBOARD_GENERATE_BASE_URL"))
	if baseURL != "" {
		u, err := url.Parse(baseURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return generateConfig{}, fmt.Errorf("DASHBOARD_GENERATE_BASE_URL must be an absolute http or https URL")
		}
	}
	model := strings.TrimSpace(getenv("DASHBOARD_GENERATE_MODEL"))
	if model == "" {
		model = defaultDashboardGenerateModel
	}
	return generateConfig{
		APIKey:        strings.TrimSpace(getenv("ANTHROPIC_API_KEY")),
		Model:         model,
		Timeout:       timeout,
		BaseURL:       baseURL,
		CatalogueFile: strings.TrimSpace(getenv("DASHBOARD_GENERATE_CATALOGUE_FILE")),
	}, nil
}
