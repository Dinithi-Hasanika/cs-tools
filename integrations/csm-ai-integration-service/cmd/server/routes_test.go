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

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/handler"
)

// status sends method path through the service's routes. The handler has no
// generators (no key), so a registered AI route answers 503 not_configured
// and an unregistered one 404.
func status(t *testing.T, codeGeneration bool, method, path string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	newMux(handler.NewDashboardHandler(nil, nil), codeGeneration).
		ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(`{"prompt":"p"}`)))
	return rec.Code
}

func TestTheCodeRouteExistsOnlyWhenTurnedOn(t *testing.T) {
	if got := status(t, false, http.MethodPost, "/ai/dashboards/generate-code"); got != http.StatusNotFound {
		t.Errorf("off: generate-code = %d, want 404 (not registered)", got)
	}
	if got := status(t, true, http.MethodPost, "/ai/dashboards/generate-code"); got != http.StatusServiceUnavailable {
		t.Errorf("on: generate-code = %d, want 503 from the handler (registered, no key)", got)
	}
}

func TestTheOtherRoutesDoNotDependOnTheFlag(t *testing.T) {
	for _, on := range []bool{false, true} {
		if got := status(t, on, http.MethodGet, "/health"); got != http.StatusOK {
			t.Errorf("code generation %v: /health = %d, want 200", on, got)
		}
		if got := status(t, on, http.MethodPost, "/ai/dashboards/generate"); got != http.StatusServiceUnavailable {
			t.Errorf("code generation %v: generate = %d, want 503 from the handler", on, got)
		}
	}
}
