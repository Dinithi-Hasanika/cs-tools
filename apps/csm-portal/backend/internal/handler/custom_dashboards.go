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
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// Limits entity-service enforces on a saved dashboard (its migration 0249),
// checked here first so the user gets a readable message.
const (
	customDashboardNameMaxLength      = 255
	customDashboardDefinitionMaxBytes = 65536
)

// Messages for a dashboard that cannot be saved, written to be shown to the
// user as they are. The validation detail is logged, not shown: a draft this
// backend served passes, so a failure means a stale page or a hand-made body.
const (
	errMsgCustomDashboardMissing       = "There is no dashboard to save."
	errMsgCustomDashboardIDMismatch    = "The dashboard does not match the address it is being saved to."
	errMsgCustomDashboardInvalid       = "This dashboard can't be saved because it isn't valid. Try generating it again."
	errMsgCustomDashboardNameEmpty     = "The dashboard needs a name before it can be saved."
	errMsgCustomDashboardNameTooLong   = "The dashboard name is too long: keep it under 255 characters."
	errMsgCustomDashboardTooLarge      = "This dashboard is too large to save. Try one with fewer widgets."
	errMsgCustomDashboardSaveFailed    = "The dashboard couldn't be saved. Try again in a moment."
	errMsgCustomDashboardListFailed    = "Your saved dashboards couldn't be loaded. Try again in a moment."
	errMsgCustomDashboardLoadFailed    = "The dashboard couldn't be loaded. Try again in a moment."
	errMsgCustomDashboardDeleteFailed  = "The dashboard couldn't be deleted. Try again in a moment."
	errMsgCustomDashboardSharesFailed  = "The people this dashboard is shared with couldn't be loaded. Try again in a moment."
	errMsgCustomDashboardShareFailed   = "The dashboard couldn't be shared. Try again in a moment."
	errMsgCustomDashboardUnshareFailed = "The dashboard couldn't be unshared. Try again in a moment."
	errMsgSharedDashboardListFailed    = "The dashboards shared with you couldn't be loaded. Try again in a moment."
	errMsgSharedDashboardOutdated      = "This shared dashboard can't be shown any more because dashboards have changed since it was saved. Ask its owner to update it."
	errMsgCustomDashboardOutdated      = "This saved dashboard can't be shown any more because dashboards have changed since it was saved. Generate it again."
)

// entityCustomDashboardClient is the part of the entity client the custom
// dashboard routes use.
type entityCustomDashboardClient interface {
	ListCustomDashboards(ctx context.Context) ([]entity.CustomDashboardSummary, error)
	GetCustomDashboard(ctx context.Context, id string) (entity.CustomDashboard, error)
	DeleteCustomDashboard(ctx context.Context, id string) error
	ListCustomDashboardShares(ctx context.Context, id string) ([]entity.CustomDashboardShare, error)
	AddCustomDashboardShare(ctx context.Context, id, userID string) ([]entity.CustomDashboardShare, error)
	RemoveCustomDashboardShare(ctx context.Context, id, userID string) error
	ListSharedCustomDashboards(ctx context.Context) ([]entity.SharedCustomDashboardSummary, error)
	GetSharedCustomDashboard(ctx context.Context, id string) (entity.SharedCustomDashboard, error)
	SaveCustomDashboard(ctx context.Context, id, name, definition string) (entity.CustomDashboard, error)
}

// customDashboardSummary is one entry of GET /dashboards/custom: enough for
// the My Dashboards dropdown, without the definition.
type customDashboardSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedOn string `json:"createdOn"`
	UpdatedOn string `json:"updatedOn"`
}

// customDashboardShareListResponse is the body of GET and PUT
// /dashboards/custom/{id}/shares[/{userId}]: who the dashboard is shared with,
// in entity-service's shape (each person's id, email and name, and when).
type customDashboardShareListResponse struct {
	Shares []entity.CustomDashboardShare `json:"shares"`
}

// customDashboardListResponse is the body of GET /dashboards/custom.
type customDashboardListResponse struct {
	Dashboards []customDashboardSummary `json:"dashboards"`
}

// saveCustomDashboardRequest is the body of PUT /dashboards/custom/{id}:
// the generate page's draft, in the GET /dashboards/{dashboardId} shape the
// generate endpoint returned it in.
type saveCustomDashboardRequest struct {
	Dashboard json.RawMessage `json:"dashboard"`
}

// customDashboardResponse is a saved dashboard: the dashboard in the
// GET /dashboards/{dashboardId} shape, so the webapp draws it with the same
// components, and when it was saved.
type customDashboardResponse struct {
	Dashboard dashboardDetailView `json:"dashboard"`
	CreatedOn string              `json:"createdOn"`
	UpdatedOn string              `json:"updatedOn"`
}

// newCustomDashboardResponse is the response for a saved dashboard: d is its
// parsed definition, and the id and times come from the stored row.
func newCustomDashboardResponse(d dashboard.Dashboard, saved entity.CustomDashboard) customDashboardResponse {
	return customDashboardResponse{
		Dashboard: storedDashboardView(d, saved.ID),
		CreatedOn: saved.CreatedOn,
		UpdatedOn: saved.UpdatedOn,
	}
}

// storedDashboardView is a parsed stored definition in the
// GET /dashboards/{dashboardId} shape, under the stored row's id.
func storedDashboardView(d dashboard.Dashboard, id string) dashboardDetailView {
	return dashboardDetailView{
		ID:          id,
		DisplayName: d.DisplayName,
		Type:        d.Type,
		IsDefault:   d.IsDefault,
		TargetTeam:  d.TargetTeam,
		IsTeamBased: d.IsTeamBased,
		Widgets:     widgetViews(d.Widgets),
	}
}

// sharedDashboardListResponse is the body of GET /dashboards/shared: the
// dashboards other users shared with the caller, each with its owner, no
// definitions (entity-service's shape).
type sharedDashboardListResponse struct {
	Dashboards []entity.SharedCustomDashboardSummary `json:"dashboards"`
}

// sharedDashboardResponse is the body of GET /dashboards/shared/{id}: the
// dashboard in the GET /dashboards/{dashboardId} shape, who shared it, when,
// and when its owner last changed it.
type sharedDashboardResponse struct {
	Dashboard dashboardDetailView          `json:"dashboard"`
	Owner     entity.CustomDashboardPerson `json:"owner"`
	SharedOn  string                       `json:"sharedOn"`
	UpdatedOn string                       `json:"updatedOn"`
}

// CustomDashboardHandler serves the caller's own saved dashboards (My
// Dashboards). Storage is entity-service's /users/me/dashboards, which keys
// every row to the caller of the forwarded x-user-id-token; this handler
// owns what a definition may contain. A definition is checked with
// dashboard.ParseGenerated, the same check a generated one passes, before it
// is stored.
type CustomDashboardHandler struct {
	entity entityCustomDashboardClient
}

// NewCustomDashboardHandler returns the handler using client.
func NewCustomDashboardHandler(client entityCustomDashboardClient) *CustomDashboardHandler {
	return &CustomDashboardHandler{entity: client}
}

// ListCustomDashboards handles GET /dashboards/custom: the caller's saved
// dashboards, ordered by name (entity-service's order), never null.
func (h *CustomDashboardHandler) ListCustomDashboards(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	list, err := h.entity.ListCustomDashboards(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListCustomDashboards failed", "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgCustomDashboardListFailed)
		return
	}
	resp := customDashboardListResponse{Dashboards: make([]customDashboardSummary, 0, len(list))}
	for _, d := range list {
		resp.Dashboards = append(resp.Dashboards, customDashboardSummary{
			ID: d.ID, Name: d.Name, CreatedOn: d.CreatedOn, UpdatedOn: d.UpdatedOn,
		})
	}
	writeJSONValue(w, http.StatusOK, resp)
}

// GetCustomDashboard handles GET /dashboards/custom/{id}: one of the caller's
// saved dashboards in the GET /dashboards/{dashboardId} shape, so the webapp
// draws it with the same components. Another user's dashboard is a 404, the
// same as a missing one.
//
// The stored definition is checked again with dashboard.ParseGenerated, the
// check it passed when it was saved, and that also adds the implied "type"
// filters the served shape carries. A definition that no longer passes (the
// rules changed since it was saved) is a 422 asking the user to generate it
// again: drawing it anyway could run queries the current rules refuse.
func (h *CustomDashboardHandler) GetCustomDashboard(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	id = strings.ToLower(id)

	saved, err := h.entity.GetCustomDashboard(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCustomDashboard failed", "dashboardId", id, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgCustomDashboardLoadFailed)
		return
	}
	d, err := dashboard.ParseGenerated([]byte(saved.Definition))
	if err != nil {
		slog.ErrorContext(r.Context(), "custom dashboard get: stored definition no longer valid", "dashboardId", id, "err", err)
		writeError(w, http.StatusUnprocessableEntity, errMsgCustomDashboardOutdated)
		return
	}

	writeJSONValue(w, http.StatusOK, newCustomDashboardResponse(d, saved))
}

// DeleteCustomDashboard handles DELETE /dashboards/custom/{id}: removes one
// of the caller's saved dashboards (204). Another user's dashboard is a 404,
// the same as a missing one, and is left as it is.
func (h *CustomDashboardHandler) DeleteCustomDashboard(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	id = strings.ToLower(id)

	if err := h.entity.DeleteCustomDashboard(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteCustomDashboard failed", "dashboardId", id, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgCustomDashboardDeleteFailed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// customDashboardAndUserIDs reads and checks the {id} and {userId} path
// values; on a bad one it has written the 400 and returns false.
func customDashboardAndUserIDs(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	id, userID := r.PathValue("id"), r.PathValue("userId")
	if !uuidRe.MatchString(id) || !uuidRe.MatchString(userID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return "", "", false
	}
	return strings.ToLower(id), strings.ToLower(userID), true
}

// ListCustomDashboardShares handles GET /dashboards/custom/{id}/shares: who
// the caller's dashboard is shared with. Owner only: another user's
// dashboard is a 404, the same as a missing one.
func (h *CustomDashboardHandler) ListCustomDashboardShares(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	shares, err := h.entity.ListCustomDashboardShares(r.Context(), strings.ToLower(id))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListCustomDashboardShares failed", "dashboardId", id, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgCustomDashboardSharesFailed)
		return
	}
	writeJSONValue(w, http.StatusOK, customDashboardShareListResponse{Shares: shares})
}

// AddCustomDashboardShare handles PUT /dashboards/custom/{id}/shares/{userId}
// (no body): shares the caller's dashboard with a user, view only, and answers
// the dashboard's shares after the change; sharing again is a 200 too. The
// refusals are passed on with mapUpstreamError, as for Save, because the user
// can act on them: the user is the caller, or not internal staff (400).
// A missing dashboard or user is a 404.
func (h *CustomDashboardHandler) AddCustomDashboardShare(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, userID, ok := customDashboardAndUserIDs(w, r)
	if !ok {
		return
	}
	shares, err := h.entity.AddCustomDashboardShare(r.Context(), id, userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AddCustomDashboardShare failed", "dashboardId", id, "userId", userID, "err", err)
		mapUpstreamError(w, err, errMsgCustomDashboardShareFailed)
		return
	}
	writeJSONValue(w, http.StatusOK, customDashboardShareListResponse{Shares: shares})
}

// RemoveCustomDashboardShare handles DELETE /dashboards/custom/{id}/shares/{userId}
// (204): stops sharing the caller's dashboard with that user. Not shared with
// them, or not the caller's dashboard, is a 404.
func (h *CustomDashboardHandler) RemoveCustomDashboardShare(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, userID, ok := customDashboardAndUserIDs(w, r)
	if !ok {
		return
	}
	if err := h.entity.RemoveCustomDashboardShare(r.Context(), id, userID); err != nil {
		slog.ErrorContext(r.Context(), "entity RemoveCustomDashboardShare failed", "dashboardId", id, "userId", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgCustomDashboardUnshareFailed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListSharedDashboards handles GET /dashboards/shared: the dashboards other
// users shared with the caller, view only, ordered by name, never null.
func (h *CustomDashboardHandler) ListSharedDashboards(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	list, err := h.entity.ListSharedCustomDashboards(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListSharedCustomDashboards failed", "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgSharedDashboardListFailed)
		return
	}
	if list == nil {
		list = []entity.SharedCustomDashboardSummary{}
	}
	writeJSONValue(w, http.StatusOK, sharedDashboardListResponse{Dashboards: list})
}

// GetSharedDashboard handles GET /dashboards/shared/{id}: one dashboard shared
// with the caller, the owner's current version, view only. Not shared with
// the caller (including the caller's own) is a 404.
//
// Like GetCustomDashboard, the stored definition is checked again with
// dashboard.ParseGenerated; one that no longer passes is a 422 whose message
// asks the recipient to have the owner update it (only the owner can).
func (h *CustomDashboardHandler) GetSharedDashboard(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	id = strings.ToLower(id)

	shared, err := h.entity.GetSharedCustomDashboard(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetSharedCustomDashboard failed", "dashboardId", id, "err", err)
		mapUpstreamErrorGeneric(w, err, errMsgCustomDashboardLoadFailed)
		return
	}
	d, err := dashboard.ParseGenerated([]byte(shared.Definition))
	if err != nil {
		slog.ErrorContext(r.Context(), "shared dashboard get: stored definition no longer valid", "dashboardId", id, "err", err)
		writeError(w, http.StatusUnprocessableEntity, errMsgSharedDashboardOutdated)
		return
	}
	writeJSONValue(w, http.StatusOK, sharedDashboardResponse{
		Dashboard: storedDashboardView(d, shared.ID),
		Owner:     shared.Owner,
		SharedOn:  shared.SharedOn,
		UpdatedOn: shared.UpdatedOn,
	})
}

// SaveCustomDashboard handles PUT /dashboards/custom/{id}: creates the
// dashboard, or replaces it when the caller already saved one with this id.
//
// The draft is turned back into the file format (draftToDefinition) and
// checked with dashboard.ParseGenerated; only a definition that passes is
// stored, and its name is the definition's displayName. Upstream refusals
// are passed on with mapUpstreamError, like the PATCH handlers, because they
// are about what the user just submitted and the user can act on them: a
// name they already use (409), the 50-dashboard limit (400). Another user's
// id is a 404, the same as a missing dashboard.
func (h *CustomDashboardHandler) SaveCustomDashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	id = strings.ToLower(id)

	var req saveCustomDashboardRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	definition, err := draftToDefinition(req.Dashboard)
	if err != nil {
		slog.WarnContext(r.Context(), "custom dashboard save: unreadable draft", "dashboardId", id, "err", err)
		writeError(w, http.StatusBadRequest, errMsgCustomDashboardInvalid)
		return
	}
	if definition == nil {
		writeError(w, http.StatusBadRequest, errMsgCustomDashboardMissing)
		return
	}
	var draft struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Dashboard, &draft); err == nil && draft.ID != "" && !strings.EqualFold(draft.ID, id) {
		writeError(w, http.StatusBadRequest, errMsgCustomDashboardIDMismatch)
		return
	}

	d, err := dashboard.ParseGenerated(definition)
	if err != nil {
		slog.WarnContext(r.Context(), "custom dashboard save: invalid definition", "dashboardId", id, "err", err)
		writeError(w, http.StatusBadRequest, errMsgCustomDashboardInvalid)
		return
	}
	name := strings.TrimSpace(d.DisplayName)
	if name == "" {
		writeError(w, http.StatusBadRequest, errMsgCustomDashboardNameEmpty)
		return
	}
	if utf8.RuneCountInString(name) > customDashboardNameMaxLength {
		writeError(w, http.StatusBadRequest, errMsgCustomDashboardNameTooLong)
		return
	}
	if len(definition) > customDashboardDefinitionMaxBytes {
		writeError(w, http.StatusBadRequest, errMsgCustomDashboardTooLarge)
		return
	}

	saved, err := h.entity.SaveCustomDashboard(r.Context(), id, name, string(definition))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SaveCustomDashboard failed", "dashboardId", id, "err", err)
		mapUpstreamError(w, err, errMsgCustomDashboardSaveFailed)
		return
	}

	writeJSONValue(w, http.StatusOK, newCustomDashboardResponse(d, saved))
}
