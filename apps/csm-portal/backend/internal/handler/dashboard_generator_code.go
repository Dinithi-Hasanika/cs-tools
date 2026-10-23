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
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go"
)

// CodeDashboardGenerator turns one chat message into dashboard source code
// for POST /dashboards/generate-code (experimental).
//
// prompt is already trimmed and length-checked by the handler. current is
// the source being edited, as the webapp sent it, or "" for the first
// message. Implementations: sampleCodeDashboardGenerator (no API key) and
// claudeCodeDashboardGenerator.
type CodeDashboardGenerator interface {
	Generate(ctx context.Context, prompt, current string) (GeneratedCodeDashboard, error)
}

// GeneratedCodeDashboard is one generator turn: the source to run and the
// chat reply to show.
type GeneratedCodeDashboard struct {
	Source string
	Reply  string
}

// sampleCodeDashboardGenerator answers every message with
// sampleCodeDashboardSource, ignoring the prompt and the draft. It stays the
// fallback when no API key is configured, like sampleDashboardGenerator.
type sampleCodeDashboardGenerator struct{}

// NewSampleCodeDashboardGenerator returns the sample generator.
func NewSampleCodeDashboardGenerator() CodeDashboardGenerator {
	return sampleCodeDashboardGenerator{}
}

// Generate returns the sample.
func (sampleCodeDashboardGenerator) Generate(context.Context, string, string) (GeneratedCodeDashboard, error) {
	return GeneratedCodeDashboard{Source: sampleCodeDashboardSource, Reply: sampleCodeDashboardReply}, nil
}

// claudeCodeMaxTokens caps one reply. Generated code is longer than a JSON
// definition (the catalogue's example is about 1,500 output tokens); the
// catalogue asks for under 200 lines, which stays well inside this.
const claudeCodeMaxTokens = 10000

// generatedCodeMaxLength caps a source, in characters (runes): a reply over
// it is unusable, and a draft over it is refused by the handler before any
// model call, since it is sent back to the model on every follow-up.
const generatedCodeMaxLength = 60000

// generatedCodeReplyMaxLength caps the model's sentence shown in the chat.
const generatedCodeReplyMaxLength = 500

// generatedCodeDefaultReply is shown when the model wrote no sentence.
const generatedCodeDefaultReply = "Here's your dashboard."

// codeFenceRe matches a fenced code block (```js, ```javascript or a bare
// ```), capturing its body.
var codeFenceRe = regexp.MustCompile("(?s)```[ \\t]*(?:js|javascript)?[ \\t]*\\r?\\n(.*?)\\r?\\n?```")

// renderFuncRe is the one thing a source must contain: a render function.
var renderFuncRe = regexp.MustCompile(`\bfunction\s+render\s*\(`)

// codeDeclinePrefix starts the catalogue's refusal line.
const codeDeclinePrefix = "CANNOT:"

// claudeCodeDashboardGenerator asks Claude for dashboard code: the code
// catalogue is the system prompt; the user message is the prompt and, on a
// follow-up, the source being edited. The reply is read as the catalogue's
// output contract (parseCodeReply). Errors are the JSON generator's own
// (errGenerateCall, errGenerateRefused, errGenerateTruncated,
// errGenerateUnusable, generateDeclinedError), so writeGenerateError maps
// them the same way.
type claudeCodeDashboardGenerator struct {
	client    anthropic.Client
	model     string
	timeout   time.Duration
	catalogue catalogueText
}

// NewClaudeCodeDashboardGenerator returns a generator calling model through
// client (claude.NewClient) with the code catalogue as its system prompt.
// timeout bounds one chat message as a whole; zero means no bound beyond the
// request's own.
func NewClaudeCodeDashboardGenerator(client anthropic.Client, model string, timeout time.Duration, catalogue catalogueText) CodeDashboardGenerator {
	return &claudeCodeDashboardGenerator{client: client, model: model, timeout: timeout, catalogue: catalogue}
}

// Generate makes one model call for one chat message.
func (g *claudeCodeDashboardGenerator) Generate(ctx context.Context, prompt, current string) (GeneratedCodeDashboard, error) {
	system, err := g.catalogue.Text()
	if err != nil {
		return GeneratedCodeDashboard{}, fmt.Errorf("%w: %w", ErrGenerateUnavailable, err)
	}

	if g.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.timeout)
		defer cancel()
	}
	started := time.Now()
	msg, err := g.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(g.model),
		MaxTokens: claudeCodeMaxTokens,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(codeUserMessage(prompt, current)))},
	})
	if err != nil {
		return GeneratedCodeDashboard{}, fmt.Errorf("%w: %w", errGenerateCall, err)
	}
	slog.InfoContext(ctx, "dashboard generate-code: model replied",
		"model", g.model, "stopReason", msg.StopReason, "elapsed", time.Since(started).Round(time.Millisecond),
		"inputTokens", msg.Usage.InputTokens, "outputTokens", msg.Usage.OutputTokens)

	switch msg.StopReason {
	case anthropic.StopReasonRefusal:
		return GeneratedCodeDashboard{}, errGenerateRefused
	case anthropic.StopReasonMaxTokens:
		return GeneratedCodeDashboard{}, errGenerateTruncated
	}

	var text strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	return parseCodeReply(text.String())
}

// codeUserMessage is the prompt alone on a first message, or the source
// followed by the prompt as the change to make.
func codeUserMessage(prompt, current string) string {
	if current == "" {
		return prompt
	}
	return "This is the dashboard code I am editing:\n\n```js\n" + current +
		"\n```\n\nChange it as follows, and reply with the whole updated code:\n\n" + prompt
}

// parseCodeReply reads a reply in the code catalogue's output contract: a
// sentence or two, then one fenced code block. The last block is taken (the
// model correcting itself), and it must define a render function. A reply
// with no block that starts with "CANNOT:" is the catalogue's refusal and
// becomes a generateDeclinedError; anything else without a block is
// unusable. The reply text is the prose before the block, collapsed onto one
// line and capped.
func parseCodeReply(text string) (GeneratedCodeDashboard, error) {
	matches := codeFenceRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		trimmed := strings.TrimSpace(text)
		if reason, ok := strings.CutPrefix(trimmed, codeDeclinePrefix); ok {
			return GeneratedCodeDashboard{}, &generateDeclinedError{Reason: capRunes(collapseSpace(reason), generatedCodeReplyMaxLength)}
		}
		return GeneratedCodeDashboard{}, fmt.Errorf("%w: no code block in the reply", errGenerateUnusable)
	}
	last := matches[len(matches)-1]
	source := strings.TrimSpace(text[last[2]:last[3]])
	if source == "" {
		return GeneratedCodeDashboard{}, fmt.Errorf("%w: empty code block", errGenerateUnusable)
	}
	if utf8.RuneCountInString(source) > generatedCodeMaxLength {
		return GeneratedCodeDashboard{}, fmt.Errorf("%w: code over %d characters", errGenerateUnusable, generatedCodeMaxLength)
	}
	if !renderFuncRe.MatchString(source) {
		return GeneratedCodeDashboard{}, fmt.Errorf("%w: code defines no render function", errGenerateUnusable)
	}

	reply := capRunes(collapseSpace(text[:matches[0][0]]), generatedCodeReplyMaxLength)
	if reply == "" {
		reply = generatedCodeDefaultReply
	}
	return GeneratedCodeDashboard{Source: source + "\n", Reply: reply}, nil
}

// collapseSpace trims s and joins its lines and runs of spaces with one space.
func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// capRunes cuts s to at most n characters, ending in "..." when cut.
func capRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n-3])) + "..."
}
