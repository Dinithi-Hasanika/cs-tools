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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/claude"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard/catalogue"
)

// validCodeSource is a dashboard as the model writes it.
const validCodeSource = `async function render(portal, root) {
  const r = await portal.countCases({ groupBy: "severity" });
  root.textContent = String(r.total);
}`

// validCodeReply is a whole reply in the code catalogue's output contract.
const validCodeReply = "Your cases by severity.\n\n```js\n" + validCodeSource + "\n```\n"

func newClaudeCodeTestGenerator(t *testing.T, gw *fakeGateway, cat catalogueText, timeout time.Duration) CodeDashboardGenerator {
	t.Helper()
	srv := httptest.NewServer(gw)
	t.Cleanup(srv.Close)
	client := claude.NewClient(claude.Config{APIKey: "test-key", BaseURL: srv.URL + "/proxy", Timeout: timeout})
	return NewClaudeCodeDashboardGenerator(client, "claude-sonnet-5-5", timeout, cat)
}

func TestClaudeCodeGenerator_ReadsTheCode(t *testing.T) {
	replies := map[string]string{
		"js fence":         validCodeReply,
		"javascript fence": "Your cases by severity.\n```javascript\n" + validCodeSource + "\n```",
		"bare fence":       "Your cases by severity.\n```\n" + validCodeSource + "\n```",
		"self-corrected":   "Your cases by severity.\n```js\nfunction draw() {}\n```\nThat is wrong; corrected:\n```js\n" + validCodeSource + "\n```",
	}
	for name, reply := range replies {
		t.Run(name, func(t *testing.T) {
			gw := &fakeGateway{text: reply}
			got, err := newClaudeCodeTestGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE"}, 5*time.Second).
				Generate(context.Background(), "My cases by severity.", "")
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got.Source != validCodeSource+"\n" {
				t.Fatalf("source = %q, want the (last) block", got.Source)
			}
			if got.Reply != "Your cases by severity." {
				t.Errorf("reply = %q", got.Reply)
			}
			system, user, maxTokens, model := gw.request(t)
			if system != "CODE CATALOGUE" || user != "My cases by severity." || maxTokens != claudeCodeMaxTokens || model != "claude-sonnet-5-5" {
				t.Errorf("request = system %q, user %q, max_tokens %d, model %q", system, user, maxTokens, model)
			}
		})
	}
}

func TestClaudeCodeGenerator_FollowUpSendsTheSource(t *testing.T) {
	gw := &fakeGateway{text: validCodeReply}
	if _, err := newClaudeCodeTestGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE"}, 5*time.Second).
		Generate(context.Background(), "Add a list of escalations.", validCodeSource); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	_, user, _, _ := gw.request(t)
	for _, want := range []string{"```js\n" + validCodeSource + "\n```", "reply with the whole updated code", "Add a list of escalations."} {
		if !strings.Contains(user, want) {
			t.Errorf("user message lacks %q:\n%s", want, user)
		}
	}
}

func TestClaudeCodeGenerator_Errors(t *testing.T) {
	cases := []struct {
		name    string
		gw      *fakeGateway
		timeout time.Duration
		want    error
	}{
		{"prose only", &fakeGateway{text: "Here is your dashboard."}, 0, errGenerateUnusable},
		{"empty reply", &fakeGateway{text: ""}, 0, errGenerateUnusable},
		{"no render function", &fakeGateway{text: "Cases.\n```js\nfunction draw(portal, root) {}\n```"}, 0, errGenerateUnusable},
		{"empty block", &fakeGateway{text: "Cases.\n```js\n\n```"}, 0, errGenerateUnusable},
		{"code too long", &fakeGateway{text: "Cases.\n```js\n" + validCodeSource + strings.Repeat("\n// x", generatedCodeMaxLength/4) + "\n```"}, 0, errGenerateUnusable},
		{"refusal", &fakeGateway{text: "", stopReason: "refusal"}, 0, errGenerateRefused},
		{"cut off", &fakeGateway{text: "Cases.\n```js\nasync function render(", stopReason: "max_tokens"}, 0, errGenerateTruncated},
		{"gateway 401", &fakeGateway{status: http.StatusUnauthorized, body: `{"error":"Unauthorized","message":"Valid API key required"}`}, 0, errGenerateCall},
		{"timeout", &fakeGateway{text: validCodeReply, delay: 2 * time.Second}, 100 * time.Millisecond, errGenerateCall},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			timeout := tc.timeout
			if timeout == 0 {
				timeout = 5 * time.Second
			}
			_, err := newClaudeCodeTestGenerator(t, tc.gw, fixedCatalogue{text: "CODE CATALOGUE"}, timeout).
				Generate(context.Background(), "x", "")
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestClaudeCodeGenerator_Declined(t *testing.T) {
	gw := &fakeGateway{text: "CANNOT: Unassigned cases cannot be filtered\nin this catalogue."}
	_, err := newClaudeCodeTestGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE"}, 5*time.Second).
		Generate(context.Background(), "Unassigned cases.", "")
	var declined *generateDeclinedError
	if !errors.As(err, &declined) || declined.Reason != "Unassigned cases cannot be filtered in this catalogue." {
		t.Fatalf("err = %v, want generateDeclinedError with the model's sentence on one line", err)
	}
}

func TestClaudeCodeGenerator_NoCallWithoutCatalogue(t *testing.T) {
	gw := &fakeGateway{text: validCodeReply}
	gen := newClaudeCodeTestGenerator(t, gw, fixedCatalogue{err: errors.New("read catalogue: missing")}, 5*time.Second)
	if _, err := gen.Generate(context.Background(), "x", ""); !errors.Is(err, ErrGenerateUnavailable) {
		t.Errorf("err = %v, want ErrGenerateUnavailable", err)
	}
	if n := gw.calls.Load(); n != 0 {
		t.Errorf("gateway calls = %d, want 0", n)
	}
}

func TestParseCodeReply_ReplyText(t *testing.T) {
	long := strings.Repeat("word ", 200)
	cases := map[string]struct {
		text string
		want string
	}{
		"no sentence":    {"```js\n" + validCodeSource + "\n```", generatedCodeDefaultReply},
		"two lines":      {"Your cases\nby severity.\n```js\n" + validCodeSource + "\n```", "Your cases by severity."},
		"capped":         {long + "\n```js\n" + validCodeSource + "\n```", ""},
		"prose after it": {"Cases.\n```js\n" + validCodeSource + "\n```\nLet me know.", "Cases."},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := parseCodeReply(tc.text)
			if err != nil {
				t.Fatalf("parseCodeReply: %v", err)
			}
			if name == "capped" {
				if n := len([]rune(got.Reply)); n > generatedCodeReplyMaxLength || !strings.HasSuffix(got.Reply, "...") {
					t.Fatalf("reply is %d characters (%q), want at most %d ending in ...", n, got.Reply, generatedCodeReplyMaxLength)
				}
				return
			}
			if got.Reply != tc.want {
				t.Errorf("reply = %q, want %q", got.Reply, tc.want)
			}
		})
	}
}

func TestCodeCatalogue_IsSeparateAndHoldsTheContract(t *testing.T) {
	text, err := catalogue.CodeSource{}.Text()
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	if text == catalogue.Embedded() {
		t.Fatal("code catalogue is the JSON catalogue")
	}
	for _, want := range []string{"async function render(portal, root)", "portal.currentUser()", "portal.countCases(", "portal.searchCases(", codeDeclinePrefix} {
		if !strings.Contains(text, want) {
			t.Errorf("code catalogue lacks %q", want)
		}
	}
	// The catalogue's own example must pass the reply checks it describes.
	start := strings.Index(text, "## 9. A complete example")
	if start < 0 {
		t.Fatal("code catalogue has no example section")
	}
	if _, err := parseCodeReply(text[start:]); err != nil {
		t.Fatalf("the catalogue's example is not a usable reply: %v", err)
	}
}
