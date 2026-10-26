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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

const testCustomDashboardID = "5e636a12-0000-4000-8000-000000000001"

type fakeCustomDashboardClient struct {
	called         bool
	gotID, gotName string
	gotDefinition  string
	list           []entity.CustomDashboardSummary
	stored         entity.CustomDashboard
	err            error
}

func (f *fakeCustomDashboardClient) GetCustomDashboard(_ context.Context, id string) (entity.CustomDashboard, error) {
	f.called = true
	f.gotID = id
	if f.err != nil {
		return entity.CustomDashboard{}, f.err
	}
	return f.stored, nil
}

func (f *fakeCustomDashboardClient) DeleteCustomDashboard(_ context.Context, id string) error {
	f.called = true
	f.gotID = id
	return f.err
}

func (f *fakeCustomDashboardClient) ListCustomDashboards(context.Context) ([]entity.CustomDashboardSummary, error) {
	f.called = true
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

func (f *fakeCustomDashboardClient) SaveCustomDashboard(_ context.Context, id, name, definition string) (entity.CustomDashboard, error) {
	f.called = true
	f.gotID, f.gotName, f.gotDefinition = id, name, definition
	if f.err != nil {
		return entity.CustomDashboard{}, f.err
	}
	return entity.CustomDashboard{ID: id, Name: name, Definition: definition, CreatedOn: "2026-10-26T10:00:00Z", UpdatedOn: "2026-10-26T10:05:00Z"}, nil
}

// customDraft is authoredDraftDashboard with id, as the generate page holds
// it: in the served form.
func customDraft(t *testing.T, id string) (dashboard.Dashboard, json.RawMessage) {
	t.Helper()
	return servedDraft(t, strings.Replace(authoredDraftDashboard, `"ai-draft-round-trip"`, `"`+id+`"`, 1))
}

func putCustomDashboard(t *testing.T, client *fakeCustomDashboardClient, pathID string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /dashboards/custom/{id}", NewCustomDashboardHandler(client).SaveCustomDashboard)
	req := withUser(httptest.NewRequest(http.MethodPut, "/dashboards/custom/"+pathID, bytes.NewReader(body)))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func saveBody(t *testing.T, draft json.RawMessage) []byte {
	t.Helper()
	b, err := json.Marshal(saveCustomDashboardRequest{Dashboard: draft})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSaveCustomDashboard_StoresCheckedDefinition(t *testing.T) {
	_, draft := customDraft(t, testCustomDashboardID)
	client := &fakeCustomDashboardClient{}

	w := putCustomDashboard(t, client, testCustomDashboardID, saveBody(t, draft))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if client.gotID != testCustomDashboardID || client.gotName != "Draft Round Trip" {
		t.Errorf("saved (id %q, name %q)", client.gotID, client.gotName)
	}
	// What is stored is the file format: it loads back through the strict
	// check, and carries "id" (not "widgetId") on its widgets.
	stored, err := dashboard.ParseGenerated([]byte(client.gotDefinition))
	if err != nil {
		t.Fatalf("stored definition does not parse: %v\n%s", err, client.gotDefinition)
	}
	if len(stored.Widgets) != 7 || strings.Contains(client.gotDefinition, "widgetId") {
		t.Errorf("stored definition = %s", client.gotDefinition)
	}

	got := decodeJSON[customDashboardResponse](t, w)
	if got.Dashboard.ID != testCustomDashboardID || got.CreatedOn == "" || got.UpdatedOn == "" {
		t.Errorf("response = %+v", got)
	}
	// The saved dashboard comes back exactly as the draft was served.
	var sent dashboardDetailView
	if err := json.Unmarshal(draft, &sent); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Dashboard, sent) {
		t.Errorf("response dashboard differs from the draft\n got: %+v\nwant: %+v", got.Dashboard, sent)
	}
}

func TestSaveCustomDashboard_LowercasesID(t *testing.T) {
	upper := strings.ToUpper(testCustomDashboardID)
	_, draft := customDraft(t, upper)
	client := &fakeCustomDashboardClient{}

	w := putCustomDashboard(t, client, upper, saveBody(t, draft))

	if w.Code != http.StatusOK || client.gotID != testCustomDashboardID {
		t.Fatalf("status %d, saved id %q; want 200 and the lower-case id", w.Code, client.gotID)
	}
}

func TestSaveCustomDashboard_RequiresUser(t *testing.T) {
	_, draft := customDraft(t, testCustomDashboardID)
	client := &fakeCustomDashboardClient{}
	req := httptest.NewRequest(http.MethodPut, "/dashboards/custom/"+testCustomDashboardID, bytes.NewReader(saveBody(t, draft)))
	req.SetPathValue("id", testCustomDashboardID)
	w := httptest.NewRecorder()

	NewCustomDashboardHandler(client).SaveCustomDashboard(w, req)

	if w.Code != http.StatusUnauthorized || client.called {
		t.Fatalf("status %d, called %v; want 401 and no upstream call", w.Code, client.called)
	}
}

func TestSaveCustomDashboard_RejectsBeforeCallingEntity(t *testing.T) {
	_, draft := customDraft(t, testCustomDashboardID)
	_, otherDraft := customDraft(t, "5e636a12-0000-4000-8000-0000000000ff")

	withField := func(field, value string) json.RawMessage {
		var m map[string]any
		if err := json.Unmarshal(draft, &m); err != nil {
			t.Fatal(err)
		}
		var v any
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			t.Fatal(err)
		}
		m[field] = v
		b, _ := json.Marshal(m)
		return b
	}
	bigWidgets := `[{"widgetId":"big","displayName":"Big","description":"` + strings.Repeat("x", customDashboardDefinitionMaxBytes) +
		`","resourceType":"case","shape":"count","gridWidth":3,"query":{}}]`

	cases := []struct {
		name   string
		pathID string
		body   []byte
		want   string
	}{
		{"non-UUID id", "abc", saveBody(t, draft), ErrMsgInvalidUUID},
		{"not JSON", testCustomDashboardID, []byte(`dashboard`), ErrMsgBadRequest},
		{"no dashboard", testCustomDashboardID, []byte(`{}`), errMsgCustomDashboardMissing},
		{"null dashboard", testCustomDashboardID, []byte(`{"dashboard":null}`), errMsgCustomDashboardMissing},
		{"another dashboard's id", testCustomDashboardID, saveBody(t, otherDraft), errMsgCustomDashboardIDMismatch},
		{"field the served form lacks", testCustomDashboardID, saveBody(t, withField("restricted", "true")), errMsgCustomDashboardInvalid},
		{"isDefault", testCustomDashboardID, saveBody(t, withField("isDefault", "true")), errMsgCustomDashboardInvalid},
		{"not a case-family widget", testCustomDashboardID, saveBody(t, withField("widgets",
			`[{"widgetId":"inc","displayName":"Incidents","resourceType":"incident","shape":"count","gridWidth":3,"query":{}}]`)), errMsgCustomDashboardInvalid},
		{"blank name", testCustomDashboardID, saveBody(t, withField("displayName", `"   "`)), ""},
		{"name too long", testCustomDashboardID, saveBody(t, withField("displayName", `"`+strings.Repeat("é", customDashboardNameMaxLength+1)+`"`)), errMsgCustomDashboardNameTooLong},
		{"definition too large", testCustomDashboardID, saveBody(t, withField("widgets", bigWidgets)), errMsgCustomDashboardTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCustomDashboardClient{}
			w := putCustomDashboard(t, client, tc.pathID, tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
			}
			if client.called {
				t.Error("entity-service was called")
			}
			if tc.want != "" {
				if got := decodeJSON[errorBody](t, w); got.Message != tc.want {
					t.Errorf("message = %q, want %q", got.Message, tc.want)
				}
			}
		})
	}
}

func TestSaveCustomDashboard_UpstreamErrors(t *testing.T) {
	_, draft := customDraft(t, testCustomDashboardID)
	cases := []struct {
		name     string
		status   int
		body     string
		want     int
		wantText string
	}{
		{"name taken", http.StatusConflict, `{"code":409,"message":"you already have a dashboard named: Draft Round Trip"}`,
			http.StatusConflict, "you already have a dashboard named: Draft Round Trip"},
		{"limit reached", http.StatusBadRequest, `{"code":400,"message":"at most 50 saved dashboards are allowed"}`,
			http.StatusBadRequest, "at most 50 saved dashboards are allowed"},
		{"another user's id", http.StatusNotFound, `{"code":404,"message":"dashboard not found"}`,
			http.StatusNotFound, ErrMsgNotFound},
		{"entity-service failed", http.StatusInternalServerError, `{"code":500,"message":"pq: connection refused"}`,
			http.StatusInternalServerError, errMsgCustomDashboardSaveFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCustomDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: tc.body}}
			w := putCustomDashboard(t, client, testCustomDashboardID, saveBody(t, draft))
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.wantText {
				t.Errorf("message = %q, want %q", got.Message, tc.wantText)
			}
		})
	}
}

func getCustomDashboards(client *fakeCustomDashboardClient) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	NewCustomDashboardHandler(client).ListCustomDashboards(w, withUser(httptest.NewRequest(http.MethodGet, "/dashboards/custom", nil)))
	return w
}

func TestListCustomDashboards(t *testing.T) {
	client := &fakeCustomDashboardClient{list: []entity.CustomDashboardSummary{
		{ID: testCustomDashboardID, Name: "My Cases", CreatedOn: "2026-10-26T10:00:00Z", UpdatedOn: "2026-10-26T10:05:00Z"},
	}}
	w := getCustomDashboards(client)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[customDashboardListResponse](t, w)
	want := []customDashboardSummary{{ID: testCustomDashboardID, Name: "My Cases", CreatedOn: "2026-10-26T10:00:00Z", UpdatedOn: "2026-10-26T10:05:00Z"}}
	if !reflect.DeepEqual(got.Dashboards, want) {
		t.Errorf("dashboards = %+v, want %+v", got.Dashboards, want)
	}
}

func TestListCustomDashboards_EmptyIsAnEmptyArray(t *testing.T) {
	w := getCustomDashboards(&fakeCustomDashboardClient{})
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"dashboards":[]}` {
		t.Fatalf("status %d, body %s; want 200 and an empty array", w.Code, w.Body.String())
	}
}

func TestListCustomDashboards_RequiresUser(t *testing.T) {
	client := &fakeCustomDashboardClient{}
	w := httptest.NewRecorder()
	NewCustomDashboardHandler(client).ListCustomDashboards(w, httptest.NewRequest(http.MethodGet, "/dashboards/custom", nil))
	if w.Code != http.StatusUnauthorized || client.called {
		t.Fatalf("status %d, called %v; want 401 and no upstream call", w.Code, client.called)
	}
}

func TestListCustomDashboards_UpstreamErrorsAreGeneric(t *testing.T) {
	cases := []struct {
		status int
		want   int
		msg    string
	}{
		{http.StatusUnauthorized, http.StatusUnauthorized, ErrMsgUnauthorized},
		{http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{http.StatusBadRequest, http.StatusBadRequest, errMsgCustomDashboardListFailed},
		{http.StatusInternalServerError, http.StatusInternalServerError, errMsgCustomDashboardListFailed},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			client := &fakeCustomDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: `{"code":0,"message":"upstream detail"}`}}
			w := getCustomDashboards(client)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.msg {
				t.Errorf("message = %q, want %q", got.Message, tc.msg)
			}
		})
	}
}

func getCustomDashboard(client *fakeCustomDashboardClient, pathID string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dashboards/custom/{id}", NewCustomDashboardHandler(client).GetCustomDashboard)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, withUser(httptest.NewRequest(http.MethodGet, "/dashboards/custom/"+pathID, nil)))
	return w
}

// storedDefinition is what a save of the served draft stores.
func storedDefinition(t *testing.T, draft json.RawMessage) string {
	t.Helper()
	def, err := draftToDefinition(draft)
	if err != nil {
		t.Fatal(err)
	}
	return string(def)
}

func TestGetCustomDashboard_ServesWhatWasSaved(t *testing.T) {
	_, draft := customDraft(t, testCustomDashboardID)
	client := &fakeCustomDashboardClient{stored: entity.CustomDashboard{
		ID: testCustomDashboardID, Name: "Draft Round Trip", Definition: storedDefinition(t, draft),
		CreatedOn: "2026-10-26T10:00:00Z", UpdatedOn: "2026-10-26T10:05:00Z",
	}}

	w := getCustomDashboard(client, testCustomDashboardID)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[customDashboardResponse](t, w)
	var sent dashboardDetailView
	if err := json.Unmarshal(draft, &sent); err != nil {
		t.Fatal(err)
	}
	// Saved, then read back: the same dashboard the draft was, implied type
	// filters included.
	if !reflect.DeepEqual(got.Dashboard, sent) {
		t.Errorf("dashboard differs from the saved draft\n got: %+v\nwant: %+v", got.Dashboard, sent)
	}
	if got.CreatedOn != "2026-10-26T10:00:00Z" || got.UpdatedOn != "2026-10-26T10:05:00Z" {
		t.Errorf("times = %q, %q", got.CreatedOn, got.UpdatedOn)
	}
}

func TestGetCustomDashboard_UsesTheRowsID(t *testing.T) {
	// The stored definition carries some other id; the row's id wins.
	_, draft := customDraft(t, "5e636a12-0000-4000-8000-0000000000ff")
	client := &fakeCustomDashboardClient{stored: entity.CustomDashboard{ID: testCustomDashboardID, Definition: storedDefinition(t, draft)}}

	w := getCustomDashboard(client, strings.ToUpper(testCustomDashboardID))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if client.gotID != testCustomDashboardID {
		t.Errorf("asked entity-service for %q, want the lower-case id", client.gotID)
	}
	if got := decodeJSON[customDashboardResponse](t, w); got.Dashboard.ID != testCustomDashboardID {
		t.Errorf("dashboard id = %q, want the row's", got.Dashboard.ID)
	}
}

func TestGetCustomDashboard_RejectsBadIDAndNoUser(t *testing.T) {
	client := &fakeCustomDashboardClient{}
	if w := getCustomDashboard(client, "abc"); w.Code != http.StatusBadRequest || client.called {
		t.Errorf("non-UUID: status %d, called %v; want 400 and no upstream call", w.Code, client.called)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboards/custom/"+testCustomDashboardID, nil)
	req.SetPathValue("id", testCustomDashboardID)
	NewCustomDashboardHandler(client).GetCustomDashboard(w, req)
	if w.Code != http.StatusUnauthorized || client.called {
		t.Errorf("no user: status %d, called %v; want 401 and no upstream call", w.Code, client.called)
	}
}

func TestGetCustomDashboard_OutdatedDefinitionIs422(t *testing.T) {
	stored := `{"id":"x","displayName":"Old","type":"cs","widgets":[{"id":"inc","displayName":"Incidents","resourceType":"incident","shape":"count","gridWidth":3,"query":{}}]}`
	client := &fakeCustomDashboardClient{stored: entity.CustomDashboard{ID: testCustomDashboardID, Definition: stored}}

	w := getCustomDashboard(client, testCustomDashboardID)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", w.Code, w.Body.String())
	}
	if got := decodeJSON[errorBody](t, w); got.Message != errMsgCustomDashboardOutdated {
		t.Errorf("message = %q, want the fixed message (no validation detail)", got.Message)
	}
}

func TestGetCustomDashboard_UpstreamErrorsAreGeneric(t *testing.T) {
	cases := []struct {
		status int
		want   int
		msg    string
	}{
		{http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{http.StatusBadRequest, http.StatusBadRequest, errMsgCustomDashboardLoadFailed},
		{http.StatusInternalServerError, http.StatusInternalServerError, errMsgCustomDashboardLoadFailed},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			client := &fakeCustomDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: `{"code":0,"message":"upstream detail"}`}}
			w := getCustomDashboard(client, testCustomDashboardID)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.msg {
				t.Errorf("message = %q, want %q", got.Message, tc.msg)
			}
		})
	}
}

func deleteCustomDashboard(client *fakeCustomDashboardClient, pathID string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /dashboards/custom/{id}", NewCustomDashboardHandler(client).DeleteCustomDashboard)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, withUser(httptest.NewRequest(http.MethodDelete, "/dashboards/custom/"+pathID, nil)))
	return w
}

func TestDeleteCustomDashboard(t *testing.T) {
	client := &fakeCustomDashboardClient{}
	w := deleteCustomDashboard(client, strings.ToUpper(testCustomDashboardID))
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("status %d, body %q; want 204 and no body", w.Code, w.Body.String())
	}
	if client.gotID != testCustomDashboardID {
		t.Errorf("deleted %q, want the lower-case id", client.gotID)
	}
}

func TestDeleteCustomDashboard_RejectsBadIDAndNoUser(t *testing.T) {
	client := &fakeCustomDashboardClient{}
	if w := deleteCustomDashboard(client, "abc"); w.Code != http.StatusBadRequest || client.called {
		t.Errorf("non-UUID: status %d, called %v; want 400 and no upstream call", w.Code, client.called)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/dashboards/custom/"+testCustomDashboardID, nil)
	req.SetPathValue("id", testCustomDashboardID)
	NewCustomDashboardHandler(client).DeleteCustomDashboard(w, req)
	if w.Code != http.StatusUnauthorized || client.called {
		t.Errorf("no user: status %d, called %v; want 401 and no upstream call", w.Code, client.called)
	}
}

func TestDeleteCustomDashboard_UpstreamErrorsAreGeneric(t *testing.T) {
	cases := []struct {
		status int
		want   int
		msg    string
	}{
		{http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{http.StatusBadRequest, http.StatusBadRequest, errMsgCustomDashboardDeleteFailed},
		{http.StatusInternalServerError, http.StatusInternalServerError, errMsgCustomDashboardDeleteFailed},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			client := &fakeCustomDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: `{"code":0,"message":"upstream detail"}`}}
			w := deleteCustomDashboard(client, testCustomDashboardID)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.msg {
				t.Errorf("message = %q, want %q", got.Message, tc.msg)
			}
		})
	}
}
