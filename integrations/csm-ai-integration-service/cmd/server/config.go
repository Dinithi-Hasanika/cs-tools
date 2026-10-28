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
	defaultClaudeModel   = "claude-sonnet-5-5"
	defaultClaudeTimeout = 45 * time.Second
)

// config is the model call's configuration, shared by every feature.
//
// APIKey is a secret: never log it. An empty APIKey means model calls are
// off: the service still starts (health answers) and the AI routes answer
// 503, so a local stack without a key still works.
//
// BaseURL, when set, sends the model calls to an Anthropic-compatible gateway
// instead of the Anthropic API (the key then belongs to that gateway). It is
// deployment configuration: keep internal addresses out of committed files.
//
// CatalogueFile / CodeCatalogueFile, when set, replace the catalogues built
// into the binary (see package catalogue); in Choreo, file mounts.
type config struct {
	APIKey            string
	Model             string
	Timeout           time.Duration
	BaseURL           string
	CatalogueFile     string
	CodeCatalogueFile string
}

// GatewayHost is the host model calls go to, for the startup log line:
// BaseURL's host, or "api.anthropic.com" when no BaseURL is set. Never the
// full URL, so a path or query carrying anything sensitive stays out of logs.
func (c config) GatewayHost() string {
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
func (c config) Enabled() bool { return c.APIKey != "" }

// loadConfig reads ANTHROPIC_API_KEY, CLAUDE_MODEL (default
// defaultClaudeModel), CLAUDE_TIMEOUT (a Go duration, default
// defaultClaudeTimeout, must be > 0), CLAUDE_BASE_URL (optional; an absolute
// http or https URL), CATALOGUE_FILE and CODE_CATALOGUE_FILE (optional).
// Values are trimmed, so a key pasted with a trailing newline still works.
// Pure, so tests pass their own getenv.
func loadConfig(getenv func(string) string) (config, error) {
	timeout := defaultClaudeTimeout
	if raw := strings.TrimSpace(getenv("CLAUDE_TIMEOUT")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return config{}, fmt.Errorf("CLAUDE_TIMEOUT: invalid duration %q: %w", raw, err)
		}
		if d <= 0 {
			return config{}, fmt.Errorf("CLAUDE_TIMEOUT: duration must be > 0, got %q", raw)
		}
		timeout = d
	}
	baseURL := strings.TrimSpace(getenv("CLAUDE_BASE_URL"))
	if baseURL != "" {
		u, err := url.Parse(baseURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return config{}, fmt.Errorf("CLAUDE_BASE_URL must be an absolute http or https URL")
		}
	}
	model := strings.TrimSpace(getenv("CLAUDE_MODEL"))
	if model == "" {
		model = defaultClaudeModel
	}
	return config{
		APIKey:            strings.TrimSpace(getenv("ANTHROPIC_API_KEY")),
		Model:             model,
		Timeout:           timeout,
		BaseURL:           baseURL,
		CatalogueFile:     strings.TrimSpace(getenv("CATALOGUE_FILE")),
		CodeCatalogueFile: strings.TrimSpace(getenv("CODE_CATALOGUE_FILE")),
	}, nil
}
