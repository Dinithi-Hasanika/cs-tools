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
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// generateCodeDashboardRequest is the body of POST /dashboards/generate-code.
//
// Prompt is the user's latest chat message, checked exactly as on
// POST /dashboards/generate. Current is the source of the dashboard being
// edited, if any (the previous reply's source), at most
// generatedCodeMaxLength characters; it is sent to the model with the prompt.
type generateCodeDashboardRequest struct {
	Prompt  string `json:"prompt"`
	Current string `json:"current,omitempty"`
}

// generateCodeDashboardResponse is the reply: the dashboard's source code and
// a chat message.
type generateCodeDashboardResponse struct {
	Source string `json:"source"`
	Reply  string `json:"reply"`
}

// errMsgGenerateCodeDraftTooLong refuses a draft over generatedCodeMaxLength.
const errMsgGenerateCodeDraftTooLong = "The dashboard being edited is too large. Start a new one."

// sampleCodeDashboardReply is the sample generator's chat message.
const sampleCodeDashboardReply = "Here is a sample code dashboard: your open cases by severity and the latest escalations. " +
	"This is a fixed sample: dashboard generation is not configured."

// sampleCodeDashboardSource is the dashboard the sample generator returns
// for every message (no API key configured). It shows the shape generated
// code takes (code_catalogue.md): a `render(portal, root)` function that
// asks for its data through the fixed portal APIs (`portal.currentUser`,
// `portal.countCases`, `portal.searchCases`) and draws into `root`. It
// holds no data of its own, so it shows live numbers every time it runs.
const sampleCodeDashboardSource = `async function render(portal, root) {
  const me = await portal.currentUser();

  const bySeverity = await portal.countCases({
    filters: [
      { field: "assignedUserId", op: "in", values: [me.id] },
      { field: "state", op: "notIn", values: ["closed"] },
    ],
    groupBy: "severity",
  });

  const escalations = await portal.searchCases({
    filters: [{ field: "escalation", op: "isNotEmpty" }],
    sortBy: { field: "createdOn", order: "desc" },
    limit: 10,
  });

  root.innerHTML = "";

  const title = document.createElement("h2");
  title.textContent = "My Cases and Escalations";
  root.appendChild(title);

  const counts = document.createElement("ul");
  for (const group of bySeverity.groups) {
    const item = document.createElement("li");
    item.textContent = group.label + ": " + group.count;
    counts.appendChild(item);
  }
  root.appendChild(counts);

  const list = document.createElement("ol");
  for (const c of escalations.items) {
    const item = document.createElement("li");
    item.textContent = c.number + " - " + c.subject;
    list.appendChild(item);
  }
  root.appendChild(list);
}
`

// DashboardGenerateCodeHandler serves POST /dashboards/generate-code, the
// experimental code-based counterpart of POST /dashboards/generate: a chat
// message is turned into dashboard source code instead of a dashboard
// definition. It checks the request and asks its CodeDashboardGenerator
// (Claude with the code catalogue, or the sample without an API key; decided
// in main.go). It shares the prompt checks and writeGenerateError's status
// mapping with DashboardGenerateHandler and changes nothing on that path.
// The source is not checked beyond defining a render function: it runs only
// in the webapp's sandboxed frame, where its data comes through the fixed
// portal APIs with the viewer's own permissions. Nothing is stored.
type DashboardGenerateCodeHandler struct {
	generator CodeDashboardGenerator
}

// NewDashboardGenerateCodeHandler returns the handler using generator.
func NewDashboardGenerateCodeHandler(generator CodeDashboardGenerator) *DashboardGenerateCodeHandler {
	return &DashboardGenerateCodeHandler{generator: generator}
}

// GenerateCodeDashboard handles POST /dashboards/generate-code.
func (h *DashboardGenerateCodeHandler) GenerateCodeDashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	var req generateCodeDashboardRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		writeError(w, http.StatusBadRequest, errMsgGeneratePromptEmpty)
		return
	}
	if utf8.RuneCountInString(prompt) > generatePromptMaxLength {
		writeError(w, http.StatusBadRequest, errMsgGeneratePromptTooLong)
		return
	}

	if utf8.RuneCountInString(req.Current) > generatedCodeMaxLength {
		writeError(w, http.StatusBadRequest, errMsgGenerateCodeDraftTooLong)
		return
	}

	result, err := h.generator.Generate(r.Context(), prompt, req.Current)
	if err != nil {
		writeGenerateError(w, r, err)
		return
	}
	writeJSONValue(w, http.StatusOK, generateCodeDashboardResponse{
		Source: result.Source,
		Reply:  result.Reply,
	})
}
