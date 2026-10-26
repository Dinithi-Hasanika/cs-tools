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
	"fmt"
	"net/http"
	"net/url"
)

// CustomDashboard is a dashboard the caller saved (entity-service
// /users/me/dashboards). Definition is the dashboard JSON as text; this
// backend validates it, entity-service stores it without parsing it. The
// owner is always the caller of the x-user-id-token, so it is not here.
type CustomDashboard struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Definition string `json:"definition"`
	CreatedOn  string `json:"createdOn"`
	UpdatedOn  string `json:"updatedOn"`
}

// CustomDashboardSummary is one entry of the caller's list: no definition.
type CustomDashboardSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedOn string `json:"createdOn"`
	UpdatedOn string `json:"updatedOn"`
}

type customDashboardListResponse struct {
	Dashboards []CustomDashboardSummary `json:"dashboards"`
}

type saveCustomDashboardRequest struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

func customDashboardPath(id string) string {
	return "/users/me/dashboards/" + url.PathEscape(id)
}

// ListCustomDashboards calls GET /users/me/dashboards. Never nil on success.
func (c *CustomerEntityClient) ListCustomDashboards(ctx context.Context) ([]CustomDashboardSummary, error) {
	raw, err := c.do(ctx, http.MethodGet, "/users/me/dashboards", nil)
	if err != nil {
		return nil, err
	}
	var resp customDashboardListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("entity: decode custom dashboard list: %w", err)
	}
	if resp.Dashboards == nil {
		resp.Dashboards = []CustomDashboardSummary{}
	}
	return resp.Dashboards, nil
}

// GetCustomDashboard calls GET /users/me/dashboards/{id}. Another user's
// dashboard is a 404 *apierror.Error, the same as a missing one.
func (c *CustomerEntityClient) GetCustomDashboard(ctx context.Context, id string) (CustomDashboard, error) {
	raw, err := c.do(ctx, http.MethodGet, customDashboardPath(id), nil)
	if err != nil {
		return CustomDashboard{}, err
	}
	var d CustomDashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		return CustomDashboard{}, fmt.Errorf("entity: decode custom dashboard: %w", err)
	}
	return d, nil
}

// SaveCustomDashboard calls PUT /users/me/dashboards/{id}: creates the
// dashboard, or replaces it when the caller already has one with this id.
// A taken name is a 409 *apierror.Error; another user's id is a 404.
func (c *CustomerEntityClient) SaveCustomDashboard(ctx context.Context, id, name, definition string) (CustomDashboard, error) {
	body, err := json.Marshal(saveCustomDashboardRequest{Name: name, Definition: definition})
	if err != nil {
		return CustomDashboard{}, fmt.Errorf("entity: encode custom dashboard: %w", err)
	}
	raw, err := c.do(ctx, http.MethodPut, customDashboardPath(id), body)
	if err != nil {
		return CustomDashboard{}, err
	}
	var d CustomDashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		return CustomDashboard{}, fmt.Errorf("entity: decode saved custom dashboard: %w", err)
	}
	return d, nil
}

// DeleteCustomDashboard calls DELETE /users/me/dashboards/{id} (204).
func (c *CustomerEntityClient) DeleteCustomDashboard(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, customDashboardPath(id), nil)
	return err
}
