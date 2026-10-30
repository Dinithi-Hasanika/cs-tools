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

package service

import "testing"

// These three raw condition strings are the real "Cloud Incident - High
// Priority" sla_policy row (id 5b117191-87ec-fd50-dcc8-c998cebb354d),
// fetched directly from the table, not invented -- pinning the exact
// behavior of translateIncidentSLACondition against real production data,
// the same way sla_policy_resolver_test.go/sla_engine_service_test.go pin
// against real case-policy condition text.
const (
	realCloudIncidentStartCondition  = "business_service=b9c999f81b86a01000ae86acdd4bcb61^ORbusiness_service=97ed1b8b1ba26c1000ae86acdd4bcbd3^priorityIN1,2^category=service_interruption^EQ"
	realCloudIncidentStopCondition   = "stateIN3,6,7^EQ"
	realCloudIncidentCancelCondition = "business_service!=b9c999f81b86a01000ae86acdd4bcb61^business_service!=97ed1b8b1ba26c1000ae86acdd4bcbd3^NQstate=8^NQcategory!=service_interruption^NQpriorityNOT IN1,2^EQ"
)

func TestTranslateIncidentSLACondition_RealCloudIncidentStartCondition(t *testing.T) {
	got := translateIncidentSLACondition(realCloudIncidentStartCondition)
	want := "business_service=b9c999f81b86a01000ae86acdd4bcb61^ORbusiness_service=97ed1b8b1ba26c1000ae86acdd4bcbd3^priorityINCRITICAL,HIGH^category=service_interruption"
	if got != want {
		t.Errorf("translateIncidentSLACondition(start) =\n  %q\nwant\n  %q", got, want)
	}
}

func TestTranslateIncidentSLACondition_RealCloudIncidentStopCondition(t *testing.T) {
	got := translateIncidentSLACondition(realCloudIncidentStopCondition)
	want := "stateINON_HOLD,RESOLVED,CLOSED"
	if got != want {
		t.Errorf("translateIncidentSLACondition(stop) = %q, want %q", got, want)
	}
}

func TestTranslateIncidentSLACondition_RealCloudIncidentCancelCondition(t *testing.T) {
	got := translateIncidentSLACondition(realCloudIncidentCancelCondition)
	want := "business_service!=b9c999f81b86a01000ae86acdd4bcb61^business_service!=97ed1b8b1ba26c1000ae86acdd4bcbd3^NQstate=CANCELED^NQcategory!=service_interruption^NQpriorityNOT INCRITICAL,HIGH"
	if got != want {
		t.Errorf("translateIncidentSLACondition(cancel) =\n  %q\nwant\n  %q", got, want)
	}
}

// TestTranslateIncidentSLACondition_StateCodesDifferFromCaseTranslator is the
// regression test for the exact collision this file's own doc comment warns
// about: running the SAME raw incident stop_condition through the
// case-specific translateSLACondition (sn_sla_condition_translate.go) must
// NEVER produce the same result translateIncidentSLACondition does --
// if it ever does, something has silently merged the two vocabularies back
// together.
func TestTranslateIncidentSLACondition_StateCodesDifferFromCaseTranslator(t *testing.T) {
	incidentResult := translateIncidentSLACondition(realCloudIncidentStopCondition)
	caseResult := translateSLACondition(realCloudIncidentStopCondition)
	if incidentResult == caseResult {
		t.Fatalf("translateIncidentSLACondition and translateSLACondition agree on %q -- they must use different vocabularies for \"state\" codes",
			realCloudIncidentStopCondition)
	}
	if incidentResult != "stateINON_HOLD,RESOLVED,CLOSED" {
		t.Errorf("translateIncidentSLACondition(stop) = %q, want stateINON_HOLD,RESOLVED,CLOSED", incidentResult)
	}
}
