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

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/claude"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// validGenerated is a dashboard as the model writes it (file format, no id).
const validGenerated = `{"displayName":"Top Customers by Open Cases","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,
"widgets":[{"id":"top","displayName":"Top 10","resourceType":"case","shape":"bar","gridWidth":12,
"query":{"filters":[{"field":"state","op":"notIn","values":["closed"]}]},"groupBy":{"field":"account","maxGroups":10}}]}`

type fixedCatalogue struct {
	text string
	err  error
}

func (c fixedCatalogue) Text() (string, error) { return c.text, c.err }

// fakeGateway answers every Messages call with one text block and the given
// stop reason (or with status/body when status is set), and records the
// last request body.
type fakeGateway struct {
	text       string
	stopReason string
	status     int
	body       string
	delay      time.Duration
	calls      atomic.Int32
	lastBody   atomic.Value
}

func (f *fakeGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	raw, _ := io.ReadAll(r.Body)
	f.lastBody.Store(string(raw))
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-r.Context().Done():
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, f.body)
		return
	}
	stop := f.stopReason
	if stop == "" {
		stop = "end_turn"
	}
	reply, _ := json.Marshal(map[string]any{
		"id": "msg_1", "type": "message", "role": "assistant", "model": "claude-sonnet-5-5",
		"content":     []any{map[string]any{"type": "text", "text": f.text}},
		"stop_reason": stop, "stop_sequence": nil,
		"usage": map[string]any{"input_tokens": 5000, "output_tokens": 400},
	})
	_, _ = w.Write(reply)
}

// request is what the generator sent, decoded.
func (f *fakeGateway) request(t *testing.T) (system, user string, maxTokens int, model string) {
	t.Helper()
	var req struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		System    []struct {
			Text string `json:"text"`
		} `json:"system"`
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	body, _ := f.lastBody.Load().(string)
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("decode request %s: %v", body, err)
	}
	if len(req.System) != 1 || len(req.Messages) != 1 || req.Messages[0].Role != "user" || len(req.Messages[0].Content) != 1 {
		t.Fatalf("request shape = %+v, want one system block and one user message", req)
	}
	return req.System[0].Text, req.Messages[0].Content[0].Text, req.MaxTokens, req.Model
}

func newClaudeTestGenerator(t *testing.T, gw *fakeGateway, cat catalogueText, timeout time.Duration) DashboardGenerator {
	t.Helper()
	srv := httptest.NewServer(gw)
	t.Cleanup(srv.Close)
	client := claude.NewClient(claude.Config{APIKey: "test-key", BaseURL: srv.URL + "/proxy", Timeout: timeout})
	return NewClaudeDashboardGenerator(client, "claude-sonnet-5-5", timeout, cat)
}

func TestClaudeGenerator_ReadsTheDashboard(t *testing.T) {
	replies := map[string]string{
		"one object":     validGenerated,
		"fenced":         "```json\n" + validGenerated + "\n```",
		"self-corrected": `{"displayName":"First try","widgets":[{"id":"x","linkResourceType":"account"}]}` + "\n\nWait, that is wrong. Corrected output:\n\n" + validGenerated,
		"with prose":     "Here is the dashboard {as requested}:\n" + validGenerated + "\nLet me know if you want changes.",
		"wrongly corrected": validGenerated + "\n\nWait, linkResourceType goes on the widget. Corrected:\n\n" +
			`{"displayName":"Second try","widgets":[{"id":"x","linkResourceType":"account"}]}`,
	}
	for name, reply := range replies {
		t.Run(name, func(t *testing.T) {
			gw := &fakeGateway{text: reply}
			got, err := newClaudeTestGenerator(t, gw, fixedCatalogue{text: "CATALOGUE"}, 5*time.Second).
				Generate(context.Background(), "Top 10 customers by open cases.", nil)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got.Dashboard.DisplayName != "Top Customers by Open Cases" || len(got.Dashboard.Widgets) != 1 {
				t.Fatalf("dashboard = %+v, want the last valid one", got.Dashboard)
			}
			if got.Dashboard.ID != dashboard.GeneratedPlaceholderID {
				t.Errorf("id = %q, want the placeholder (the handler sets the real one)", got.Dashboard.ID)
			}
			if got.Reply != `Here's "Top Customers by Open Cases", 1 widget.` {
				t.Errorf("reply = %q", got.Reply)
			}
			system, user, maxTokens, model := gw.request(t)
			if system != "CATALOGUE" || user != "Top 10 customers by open cases." || maxTokens != claudeMaxTokens || model != "claude-sonnet-5-5" {
				t.Errorf("request = system %q, user %q, max_tokens %d, model %q", system, user, maxTokens, model)
			}
		})
	}
}

func TestClaudeGenerator_FollowUpSendsTheDraftInFileFormat(t *testing.T) {
	_, served := servedDraft(t, authoredDraftDashboard)
	gw := &fakeGateway{text: validGenerated}
	if _, err := newClaudeTestGenerator(t, gw, fixedCatalogue{text: "CATALOGUE"}, 5*time.Second).
		Generate(context.Background(), "Make the trend 60 days.", served); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	_, user, _, _ := gw.request(t)
	def, _ := draftToDefinition(served)
	for _, want := range []string{string(def), "Change it as follows", "Make the trend 60 days."} {
		if !strings.Contains(user, want) {
			t.Errorf("user message lacks %q:\n%s", want, user)
		}
	}
	if strings.Contains(user, `"widgetId"`) || strings.Contains(user, `"values":["case"]`) {
		t.Errorf("user message carries the served form, want the file format:\n%s", user)
	}
}

func TestClaudeGenerator_Errors(t *testing.T) {
	cases := []struct {
		name    string
		gw      *fakeGateway
		timeout time.Duration
		want    error
	}{
		{"broken JSON", &fakeGateway{text: `{"displayName": "x", "widgets": [`}, 0, errGenerateUnusable},
		{"prose only", &fakeGateway{text: "I can't make that dashboard."}, 0, errGenerateUnusable},
		{"empty reply", &fakeGateway{text: ""}, 0, errGenerateUnusable},
		{"invalid dashboard", &fakeGateway{text: strings.Replace(validGenerated, `"groupBy":{"field":"account","maxGroups":10}`, `"days":5`, 1)}, 0, dashboard.ErrGeneratedInvalid},
		{"refusal", &fakeGateway{text: "", stopReason: "refusal"}, 0, errGenerateRefused},
		{"cut off", &fakeGateway{text: `{"displayName":"x"`, stopReason: "max_tokens"}, 0, errGenerateTruncated},
		{"gateway 401", &fakeGateway{status: http.StatusUnauthorized, body: `{"error":"Unauthorized","message":"Valid API key required"}`}, 0, errGenerateCall},
		{"timeout", &fakeGateway{text: validGenerated, delay: 2 * time.Second}, 100 * time.Millisecond, errGenerateCall},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			timeout := tc.timeout
			if timeout == 0 {
				timeout = 5 * time.Second
			}
			_, err := newClaudeTestGenerator(t, tc.gw, fixedCatalogue{text: "CATALOGUE"}, timeout).
				Generate(context.Background(), "x", nil)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestClaudeGenerator_Declined(t *testing.T) {
	gw := &fakeGateway{text: `{"error": "Average resolution time per engineer is not something this catalogue can show."}`}
	_, err := newClaudeTestGenerator(t, gw, fixedCatalogue{text: "CATALOGUE"}, 5*time.Second).
		Generate(context.Background(), "Average resolution time per engineer.", nil)
	var declined *generateDeclinedError
	if !errors.As(err, &declined) || !strings.HasPrefix(declined.Reason, "Average resolution time") {
		t.Fatalf("err = %v, want generateDeclinedError with the model's sentence", err)
	}
}

func TestClaudeGenerator_NoCallWithoutCatalogueOrWithABadDraft(t *testing.T) {
	gw := &fakeGateway{text: validGenerated}
	gen := newClaudeTestGenerator(t, gw, fixedCatalogue{err: errors.New("read catalogue: missing")}, 5*time.Second)
	if _, err := gen.Generate(context.Background(), "x", nil); !errors.Is(err, ErrGenerateUnavailable) {
		t.Errorf("unreadable catalogue: err = %v, want ErrGenerateUnavailable", err)
	}

	gen = newClaudeTestGenerator(t, gw, fixedCatalogue{text: "CATALOGUE"}, 5*time.Second)
	if _, err := gen.Generate(context.Background(), "x", json.RawMessage(`{"id":"ai-x","owner":"me"}`)); !errors.Is(err, errDraftInvalid) {
		t.Errorf("bad draft: err = %v, want errDraftInvalid", err)
	}
	if n := gw.calls.Load(); n != 0 {
		t.Errorf("gateway calls = %d, want 0", n)
	}
}

func TestJSONObjects(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{`{"a":1}`, []string{`{"a":1}`}},
		{`{"a":{"b":{"c":1}}}`, []string{`{"a":{"b":{"c":1}}}`}},
		{"x {not json} `{\"a\":1}` y {\"b\":2}", []string{`{"a":1}`, `{"b":2}`}},
		{"no objects here", nil},
		{`[{"a":1}]`, []string{`{"a":1}`}},
	}
	for _, tc := range cases {
		var got []string
		for _, raw := range jsonObjects(tc.text) {
			got = append(got, string(raw))
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("jsonObjects(%q) = %q; want %q", tc.text, got, tc.want)
		}
	}
}
