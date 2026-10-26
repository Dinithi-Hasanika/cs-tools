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

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

const testCustomDashboardID = "5e636a12-0000-4000-8000-000000000001"

type recordedRequest struct {
	method, path, rawPath, userToken, body string
}

// newCustomDashboardTestClient serves status and body for every call and
// records the last request.
func newCustomDashboardTestClient(t *testing.T, status int, body string) (*CustomerEntityClient, *recordedRequest) {
	t.Helper()
	got := &recordedRequest{}
	client := newHealthDetailTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*got = recordedRequest{r.Method, r.URL.Path, r.URL.EscapedPath(), r.Header.Get("x-user-id-token"), string(b)}
		if body != "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	return client, got
}

func userContext() context.Context {
	return WithUserIDToken(context.Background(), "user-token")
}

func TestListCustomDashboards(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusOK,
		`{"dashboards":[{"id":"`+testCustomDashboardID+`","name":"My Cases","createdOn":"2026-10-26T09:21:45Z","updatedOn":"2026-10-26T09:22:00Z"}]}`)
	list, err := client.ListCustomDashboards(userContext())
	if err != nil {
		t.Fatalf("ListCustomDashboards: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/users/me/dashboards" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.userToken != "user-token" {
		t.Errorf("x-user-id-token = %q, want the caller's token forwarded", got.userToken)
	}
	if len(list) != 1 || list[0].Name != "My Cases" || list[0].UpdatedOn != "2026-10-26T09:22:00Z" {
		t.Fatalf("list = %+v", list)
	}
}

func TestListCustomDashboards_EmptyIsNotNil(t *testing.T) {
	client, _ := newCustomDashboardTestClient(t, http.StatusOK, `{"dashboards":null}`)
	list, err := client.ListCustomDashboards(userContext())
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("list = %#v, %v; want an empty, non-nil slice", list, err)
	}
}

func TestGetCustomDashboard(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusOK,
		`{"id":"`+testCustomDashboardID+`","name":"My Cases","definition":"{\"widgets\":[]}","createdOn":"x","updatedOn":"y"}`)
	d, err := client.GetCustomDashboard(userContext(), testCustomDashboardID)
	if err != nil {
		t.Fatalf("GetCustomDashboard: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/users/me/dashboards/"+testCustomDashboardID {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if d.Definition != `{"widgets":[]}` || d.Name != "My Cases" {
		t.Fatalf("dashboard = %+v", d)
	}
}

func TestGetCustomDashboard_EscapesID(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusBadRequest, `{"code":400,"message":"not a UUID"}`)
	_, _ = client.GetCustomDashboard(userContext(), "../users/x")
	if got.rawPath != "/users/me/dashboards/..%2Fusers%2Fx" {
		t.Errorf("escaped path = %q, want the id kept as one segment", got.rawPath)
	}
}

func TestSaveCustomDashboard(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusOK,
		`{"id":"`+testCustomDashboardID+`","name":"My Cases","definition":"{\"widgets\":[]}","createdOn":"x","updatedOn":"y"}`)
	d, err := client.SaveCustomDashboard(userContext(), testCustomDashboardID, "My Cases", `{"widgets":[]}`)
	if err != nil {
		t.Fatalf("SaveCustomDashboard: %v", err)
	}
	if got.method != http.MethodPut || got.path != "/users/me/dashboards/"+testCustomDashboardID {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatalf("request body %q: %v", got.body, err)
	}
	if len(body) != 2 || body["name"] != "My Cases" || body["definition"] != `{"widgets":[]}` {
		t.Errorf("request body = %v, want exactly name and definition (as a string)", body)
	}
	if d.ID != testCustomDashboardID {
		t.Errorf("dashboard = %+v", d)
	}
}

func TestDeleteCustomDashboard(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusNoContent, "")
	if err := client.DeleteCustomDashboard(userContext(), testCustomDashboardID); err != nil {
		t.Fatalf("DeleteCustomDashboard: %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/users/me/dashboards/"+testCustomDashboardID {
		t.Errorf("request = %s %s", got.method, got.path)
	}
}

func TestCustomDashboardClient_PassesStatusErrorsThrough(t *testing.T) {
	cases := []struct {
		name   string
		status int
		call   func(*CustomerEntityClient) error
	}{
		{"get missing", http.StatusNotFound, func(c *CustomerEntityClient) error {
			_, err := c.GetCustomDashboard(userContext(), testCustomDashboardID)
			return err
		}},
		{"save name taken", http.StatusConflict, func(c *CustomerEntityClient) error {
			_, err := c.SaveCustomDashboard(userContext(), testCustomDashboardID, "X", "{}")
			return err
		}},
		{"delete missing", http.StatusNotFound, func(c *CustomerEntityClient) error {
			return c.DeleteCustomDashboard(userContext(), testCustomDashboardID)
		}},
		{"list unauthorized", http.StatusUnauthorized, func(c *CustomerEntityClient) error {
			_, err := c.ListCustomDashboards(userContext())
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := newCustomDashboardTestClient(t, tc.status, `{"code":0,"message":"from entity-service"}`)
			err := tc.call(client)
			var apiErr *apierror.Error
			if !errors.As(err, &apiErr) || apiErr.StatusCode != tc.status {
				t.Fatalf("err = %v, want *apierror.Error with status %d", err, tc.status)
			}
		})
	}
}
