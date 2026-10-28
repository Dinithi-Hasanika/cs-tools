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

// Package dashboards asks the model for a dashboard: a definition (POST
// /ai/dashboards/generate, this file) or JavaScript code (POST
// /ai/dashboards/generate-code, code.go). It builds the prompt (the dashboard catalogue
// plus the Teams section as the system prompt; the user's prompt, and on a
// follow-up the draft being edited, as the user message), makes the call and
// reads the reply into the JSON objects it holds. Whether one of them is a
// valid dashboard is the CSM portal backend's call (dashboard.ParseGenerated
// and the per-type rules stay there), so every object goes back, in order.
package dashboards

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/teams"
)

// maxTokens caps one reply. A generated dashboard is well under 1,000 output
// tokens; the headroom covers the model correcting itself in the same reply.
const maxTokens = 8000

// Reasons a reply gives no dashboard candidates. The text says what happened
// without the model's output, which may echo the user's prompt.
var (
	// ErrCatalogue: the catalogue could not be read (a mounted file that
	// has gone or become empty).
	ErrCatalogue = errors.New("catalogue unavailable")
	// ErrCall: the call itself failed (the gateway answered an error, the
	// connection failed, or it timed out). Wraps the SDK error.
	ErrCall = errors.New("model call failed")
	// ErrRefused: the model refused (stop reason "refusal").
	ErrRefused = errors.New("model refused the request")
	// ErrTruncated: the reply hit maxTokens before finishing.
	ErrTruncated = errors.New("model reply was cut off")
	// ErrUnusable: the reply holds nothing usable (no JSON object; for code,
	// no usable code block). Wrapped with the reason.
	ErrUnusable = errors.New("model reply is not usable")
)

// Catalogue supplies the catalogue's text (catalogue.Source).
type Catalogue interface {
	Text() (string, error)
}

// Request is one chat message. Draft is the dashboard being edited in the
// definition file format, or nil on a first message.
type Request struct {
	Prompt string
	Draft  json.RawMessage
	Teams  []teams.Team
}

// Result is what the model answered: Declined holds its sentence when it
// answered the catalogue's refusal shape ({"error": "<sentence>"}) as its
// last object; otherwise Candidates holds every JSON object in the reply, in
// order (the last valid one is the answer: see the backend).
type Result struct {
	Candidates   []json.RawMessage
	Declined     string
	InputTokens  int64
	OutputTokens int64
}

// Generator makes the model call.
type Generator struct {
	client    anthropic.Client
	model     string
	timeout   time.Duration
	catalogue Catalogue
}

// NewGenerator returns a generator calling model through client
// (claude.NewClient). timeout bounds one chat message as a whole, the
// client's retry included; zero means no bound beyond the request's own.
func NewGenerator(client anthropic.Client, model string, timeout time.Duration, catalogue Catalogue) *Generator {
	return &Generator{client: client, model: model, timeout: timeout, catalogue: catalogue}
}

// Generate makes one model call for one chat message.
func (g *Generator) Generate(ctx context.Context, req Request) (Result, error) {
	text, err := g.catalogue.Text()
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrCatalogue, err)
	}
	system := teams.WithTeams(text, req.Teams)

	if g.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.timeout)
		defer cancel()
	}
	msg, err := g.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(g.model),
		MaxTokens: maxTokens,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage(req.Prompt, req.Draft)))},
	})
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrCall, err)
	}
	slog.InfoContext(ctx, "dashboard generate: model replied",
		"model", g.model, "stopReason", msg.StopReason,
		"inputTokens", msg.Usage.InputTokens, "outputTokens", msg.Usage.OutputTokens)

	switch msg.StopReason {
	case anthropic.StopReasonRefusal:
		return Result{}, ErrRefused
	case anthropic.StopReasonMaxTokens:
		return Result{}, ErrTruncated
	}

	objects := jsonObjects(messageText(msg))
	if len(objects) == 0 {
		return Result{}, fmt.Errorf("%w: no JSON object in the reply", ErrUnusable)
	}
	res := Result{InputTokens: msg.Usage.InputTokens, OutputTokens: msg.Usage.OutputTokens}
	if reason, ok := declineReason(objects[len(objects)-1]); ok {
		res.Declined = reason
		return res, nil
	}
	res.Candidates = objects
	return res, nil
}

// messageText is the text of a model reply, its text blocks joined.
func messageText(msg *anthropic.Message) string {
	var text strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	return text.String()
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
