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
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// claudeMaxTokens caps one reply. A generated dashboard is well under 1,000
// output tokens (the gateway test: 445 for one widget, the catalogue's
// two-widget example about 600); the headroom covers the model correcting
// itself in the same reply, as it did once in that test.
const claudeMaxTokens = 8000

// Reasons a Claude reply could not become a dashboard. The handler maps them
// to statuses (a later step); the text says what happened without the
// model's output, which may echo the user's prompt.
var (
	// errGenerateCall: the call itself failed (the gateway answered an
	// error, the connection failed, or it timed out). Wraps the SDK error.
	errGenerateCall = errors.New("model call failed")
	// errGenerateRefused: the model refused (stop reason "refusal").
	errGenerateRefused = errors.New("model refused the request")
	// errGenerateTruncated: the reply hit claudeMaxTokens before finishing.
	errGenerateTruncated = errors.New("model reply was cut off")
	// errGenerateUnusable: the reply holds no JSON object, or none of them
	// is a valid generated dashboard (wraps dashboard.ErrGeneratedInvalid).
	errGenerateUnusable = errors.New("model reply is not a usable dashboard")
)

// generateDeclinedError is the model answering, as the catalogue tells it
// to when a request cannot be met, with {"error": "<what is missing>"}
// instead of a dashboard. Reason is the model's sentence.
type generateDeclinedError struct {
	Reason string
}

func (e *generateDeclinedError) Error() string {
	return "model declined: " + e.Reason
}

// catalogueText supplies the catalogue (catalogue.Source).
type catalogueText interface {
	Text() (string, error)
}

// claudeDashboardGenerator asks Claude for the dashboard: the catalogue is
// the system prompt; the user message is the prompt and, on a follow-up, the
// draft being edited in the file format (draftToDefinition). The reply is
// read as the last valid JSON object in the text (see jsonObjects), checked
// with dashboard.ParseGenerated, so it is held to the same rules as a
// definition file.
type claudeDashboardGenerator struct {
	client    anthropic.Client
	model     string
	timeout   time.Duration
	catalogue catalogueText
}

// NewClaudeDashboardGenerator returns a generator calling model through
// client (claude.NewClient) with the catalogue as its system prompt. timeout
// bounds one chat message as a whole, the client's retry included (the
// client's own timeout bounds each attempt); zero means no bound beyond the
// request's own.
func NewClaudeDashboardGenerator(client anthropic.Client, model string, timeout time.Duration, catalogue catalogueText) DashboardGenerator {
	return &claudeDashboardGenerator{client: client, model: model, timeout: timeout, catalogue: catalogue}
}

// Generate makes one model call for one chat message.
func (g *claudeDashboardGenerator) Generate(ctx context.Context, prompt string, current json.RawMessage) (GeneratedDashboard, error) {
	system, err := g.catalogue.Text()
	if err != nil {
		return GeneratedDashboard{}, fmt.Errorf("%w: %w", ErrGenerateUnavailable, err)
	}
	draft, err := draftToDefinition(current)
	if err != nil {
		return GeneratedDashboard{}, err
	}

	if g.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.timeout)
		defer cancel()
	}
	msg, err := g.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(g.model),
		MaxTokens: claudeMaxTokens,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage(prompt, draft)))},
	})
	if err != nil {
		return GeneratedDashboard{}, fmt.Errorf("%w: %w", errGenerateCall, err)
	}
	slog.InfoContext(ctx, "dashboard generate: model replied",
		"model", g.model, "stopReason", msg.StopReason,
		"inputTokens", msg.Usage.InputTokens, "outputTokens", msg.Usage.OutputTokens)

	switch msg.StopReason {
	case anthropic.StopReasonRefusal:
		return GeneratedDashboard{}, errGenerateRefused
	case anthropic.StopReasonMaxTokens:
		return GeneratedDashboard{}, errGenerateTruncated
	}

	var text strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	objects := jsonObjects(text.String())
	if len(objects) == 0 {
		return GeneratedDashboard{}, fmt.Errorf("%w: no JSON object in the reply", errGenerateUnusable)
	}
	if reason, ok := declineReason(objects[len(objects)-1]); ok {
		return GeneratedDashboard{}, &generateDeclinedError{Reason: reason}
	}

	// The last valid object wins: a self-correction (wrong, then right) takes
	// the correction, and a "correction" that breaks a valid answer (right,
	// then wrong) keeps the answer. With none valid, the last one's error.
	var lastErr error
	for i := len(objects) - 1; i >= 0; i-- {
		d, err := dashboard.ParseGenerated(objects[i])
		if err != nil {
			if lastErr == nil {
				lastErr = err
			}
			continue
		}
		if len(objects) > 1 {
			slog.InfoContext(ctx, "dashboard generate: reply held several JSON objects; using the last valid one",
				"count", len(objects), "used", i+1)
		}
		return GeneratedDashboard{Dashboard: d, Reply: generatedReply(d)}, nil
	}
	return GeneratedDashboard{}, fmt.Errorf("%w: %w", errGenerateUnusable, lastErr)
}

// userMessage is the prompt alone on a first message, or the draft followed
// by the prompt as the change to make.
func userMessage(prompt string, draft json.RawMessage) string {
	if draft == nil {
		return prompt
	}
	return "This is the dashboard I am editing:\n\n```json\n" + string(draft) +
		"\n```\n\nChange it as follows, and reply with the whole updated dashboard:\n\n" + prompt
}

// jsonObjects returns the top-level JSON objects in text, in order. A reply
// is meant to be exactly one object; this also reads one wrapped in a
// Markdown fence, and several separated by prose (the model correcting
// itself). A "{" that does not start a valid object (prose) is skipped; an
// object's own nested objects are not counted.
func jsonObjects(text string) []json.RawMessage {
	var objects []json.RawMessage
	for i := 0; i < len(text); {
		start := strings.IndexByte(text[i:], '{')
		if start < 0 {
			break
		}
		start += i
		dec := json.NewDecoder(strings.NewReader(text[start:]))
		var obj json.RawMessage
		if err := dec.Decode(&obj); err != nil {
			i = start + 1
			continue
		}
		objects = append(objects, obj)
		i = start + int(dec.InputOffset())
	}
	return objects
}

// declineReason reports whether raw is the catalogue's refusal shape,
// exactly {"error": "<sentence>"}, and returns the sentence.
func declineReason(raw json.RawMessage) (string, bool) {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil || len(obj) != 1 {
		return "", false
	}
	reason, ok := obj["error"].(string)
	return strings.TrimSpace(reason), ok
}

// generatedReply is the chat message shown with a generated dashboard.
func generatedReply(d dashboard.Dashboard) string {
	n := len(d.Widgets)
	plural := "s"
	if n == 1 {
		plural = ""
	}
	return fmt.Sprintf("Here's %q, %d widget%s.", d.DisplayName, n, plural)
}
