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

package dashboards

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/catalogue"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/claude"
)

// validCodeSource is a dashboard as the model writes it.
const validCodeSource = `async function render(portal, root) {
  const r = await portal.countCases({ groupBy: "severity" });
  root.textContent = String(r.total);
}`

// validCodeReply is a whole reply in the code catalogue's output contract.
const validCodeReply = "Your cases by severity.\n\n```js\n" + validCodeSource + "\n```\n"

func newTestCodeGenerator(t *testing.T, gw *fakeGateway, cat Catalogue, timeout time.Duration) *CodeGenerator {
	t.Helper()
	srv := httptest.NewServer(gw)
	t.Cleanup(srv.Close)
	client := claude.NewClient(claude.Config{APIKey: "test-key", BaseURL: srv.URL + "/proxy", Timeout: timeout})
	return NewCodeGenerator(client, "claude-sonnet-5-5", timeout, cat)
}

func TestCodeGeneratorReadsTheCode(t *testing.T) {
	replies := map[string]string{
		"js fence":         validCodeReply,
		"javascript fence": "Your cases by severity.\n```javascript\n" + validCodeSource + "\n```",
		"bare fence":       "Your cases by severity.\n```\n" + validCodeSource + "\n```",
		"self-corrected":   "Your cases by severity.\n```js\nfunction draw() {}\n```\nThat is wrong; corrected:\n```js\n" + validCodeSource + "\n```",
	}
	for name, reply := range replies {
		t.Run(name, func(t *testing.T) {
			gw := &fakeGateway{text: reply}
			got, err := newTestCodeGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE"}, 5*time.Second).
				Generate(context.Background(), "My cases by severity.", "")
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got.Source != validCodeSource+"\n" || got.Declined != "" {
				t.Fatalf("result = %+v, want the (last) block", got)
			}
			if got.Reply != "Your cases by severity." {
				t.Errorf("reply = %q", got.Reply)
			}
			if got.InputTokens != 5000 || got.OutputTokens != 400 {
				t.Errorf("usage = %d/%d", got.InputTokens, got.OutputTokens)
			}
			system, user, maxTok, model := gw.request(t)
			if system != "CODE CATALOGUE" || user != "My cases by severity." || maxTok != codeMaxTokens || model != "claude-sonnet-5-5" {
				t.Errorf("request = system %q, user %q, max_tokens %d, model %q", system, user, maxTok, model)
			}
		})
	}
}

func TestCodeGeneratorFollowUpSendsTheSource(t *testing.T) {
	gw := &fakeGateway{text: validCodeReply}
	if _, err := newTestCodeGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE"}, 5*time.Second).
		Generate(context.Background(), "Add a list of escalations.", validCodeSource); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	_, user, _, _ := gw.request(t)
	want := "This is the dashboard code I am editing:\n\n```js\n" + validCodeSource +
		"\n```\n\nChange it as follows, and reply with the whole updated code:\n\nAdd a list of escalations."
	if user != want {
		t.Errorf("user message =\n%q\nwant\n%q", user, want)
	}
}

func TestCodeGeneratorErrors(t *testing.T) {
	cases := []struct {
		name    string
		gw      *fakeGateway
		timeout time.Duration
		want    error
	}{
		{"prose only", &fakeGateway{text: "Here is your dashboard."}, 0, ErrUnusable},
		{"empty reply", &fakeGateway{text: ""}, 0, ErrUnusable},
		{"no render function", &fakeGateway{text: "Cases.\n```js\nfunction draw(portal, root) {}\n```"}, 0, ErrUnusable},
		{"empty block", &fakeGateway{text: "Cases.\n```js\n\n```"}, 0, ErrUnusable},
		{"code too long", &fakeGateway{text: "Cases.\n```js\n" + validCodeSource + strings.Repeat("\n// x", CodeMaxLength/4) + "\n```"}, 0, ErrUnusable},
		{"refusal", &fakeGateway{text: "", stopReason: "refusal"}, 0, ErrRefused},
		{"cut off", &fakeGateway{text: "Cases.\n```js\nasync function render(", stopReason: "max_tokens"}, 0, ErrTruncated},
		{"gateway 401", &fakeGateway{status: http.StatusUnauthorized, body: `{"error":"Unauthorized","message":"Valid API key required"}`}, 0, ErrCall},
		{"timeout", &fakeGateway{text: validCodeReply, delay: 2 * time.Second}, 100 * time.Millisecond, ErrCall},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			timeout := tc.timeout
			if timeout == 0 {
				timeout = 5 * time.Second
			}
			_, err := newTestCodeGenerator(t, tc.gw, fixedCatalogue{text: "CODE CATALOGUE"}, timeout).
				Generate(context.Background(), "x", "")
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCodeGeneratorDeclined(t *testing.T) {
	gw := &fakeGateway{text: "CANNOT: Unassigned cases cannot be filtered\nin this catalogue."}
	got, err := newTestCodeGenerator(t, gw, fixedCatalogue{text: "CODE CATALOGUE"}, 5*time.Second).
		Generate(context.Background(), "Unassigned cases.", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Declined != "Unassigned cases cannot be filtered in this catalogue." || got.Source != "" {
		t.Fatalf("result = %+v, want the model's sentence on one line and no source", got)
	}
}

func TestCodeGeneratorMakesNoCallWithoutACatalogue(t *testing.T) {
	gw := &fakeGateway{text: validCodeReply}
	gen := newTestCodeGenerator(t, gw, fixedCatalogue{err: errors.New("read catalogue: missing")}, 5*time.Second)
	if _, err := gen.Generate(context.Background(), "x", ""); !errors.Is(err, ErrCatalogue) {
		t.Errorf("err = %v, want ErrCatalogue", err)
	}
	if n := gw.calls.Load(); n != 0 {
		t.Errorf("gateway calls = %d, want 0", n)
	}
}

func TestParseCodeReplyReplyText(t *testing.T) {
	long := strings.Repeat("word ", 200)
	cases := map[string]struct {
		text string
		want string
	}{
		"no sentence":    {"```js\n" + validCodeSource + "\n```", codeDefaultReply},
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
				if n := len([]rune(got.Reply)); n > codeReplyMaxLength || !strings.HasSuffix(got.Reply, "...") {
					t.Fatalf("reply is %d characters (%q), want at most %d ending in ...", n, got.Reply, codeReplyMaxLength)
				}
				return
			}
			if got.Reply != tc.want {
				t.Errorf("reply = %q, want %q", got.Reply, tc.want)
			}
		})
	}
}

func TestBuiltInCodeCatalogueHoldsTheContract(t *testing.T) {
	text := catalogue.CodeEmbedded()
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
