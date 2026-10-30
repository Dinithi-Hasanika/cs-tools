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

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// cloudIncidentPolicyCandidates mirrors the real "Cloud Incident - High
// Priority" sla_policy row exactly (see
// incident_sla_condition_translate_test.go's own real-data constants) --
// the one real policy confirmed to exist for incidents today.
func cloudIncidentPolicyCandidates() map[string][]repository.SLAPolicyRef {
	return map[string][]repository.SLAPolicyRef{
		"RESOLUTION": {
			{
				ID:              "cloud-incident-high-priority",
				Name:            "Cloud Incident - High Priority",
				Target:          "RESOLUTION",
				Source:          "SERVICENOW",
				StartCondition:  realCloudIncidentStartCondition,
				StopCondition:   realCloudIncidentStopCondition,
				CancelCondition: realCloudIncidentCancelCondition,
				WhenToCancel:    "CANCEL_CONDITIONS_ARE_MET",
				WhenToResume:    "RESUME_CONDITIONS_ARE_MET",
			},
		},
	}
}

// choreoServiceID/asgardeoServiceID are the two business_service rows the
// real policy's conditions name, confirmed on staging (sysidToUUID of
// b9c999f81b86a01000ae86acdd4bcb61 / 97ed1b8b1ba26c1000ae86acdd4bcbd3).
const (
	choreoServiceID   = "b9c999f8-1b86-a010-00ae-86acdd4bcb61"
	asgardeoServiceID = "97ed1b8b-1ba2-6c10-00ae-86acdd4bcbd3"
)

func TestSLAEngineService_RegisterIncidentClocks_CriticalChoreoServiceInterruptionMatches(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.factsByIncident["incident-1"] = repository.IncidentFacts{
		State:     "NEW",
		Category:  "SERVICE_INTERRUPTION",
		ServiceID: choreoServiceID,
	}
	svc := NewSLAEngineService(repo)

	priority := domain.IncidentPriorityCritical
	svc.RegisterIncidentClocks(context.Background(), "incident-1", &priority)

	want := []string{"incident-1|cloud-incident-high-priority"}
	if len(repo.registered) != len(want) || repo.registered[0] != want[0] {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
}

func TestSLAEngineService_RegisterIncidentClocks_HighAsgardeoServiceInterruptionMatches(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.factsByIncident["incident-2"] = repository.IncidentFacts{
		State:     "NEW",
		Category:  "SERVICE_INTERRUPTION",
		ServiceID: asgardeoServiceID,
	}
	svc := NewSLAEngineService(repo)

	priority := domain.IncidentPriorityHigh
	svc.RegisterIncidentClocks(context.Background(), "incident-2", &priority)

	want := []string{"incident-2|cloud-incident-high-priority"}
	if len(repo.registered) != len(want) || repo.registered[0] != want[0] {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
}

func TestSLAEngineService_RegisterIncidentClocks_ModeratePriorityDoesNotMatch(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.factsByIncident["incident-3"] = repository.IncidentFacts{
		State:     "NEW",
		Category:  "SERVICE_INTERRUPTION",
		ServiceID: choreoServiceID,
	}
	svc := NewSLAEngineService(repo)

	priority := domain.IncidentPriorityModerate
	svc.RegisterIncidentClocks(context.Background(), "incident-3", &priority)

	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none: Moderate priority is outside priorityIN1,2 (Critical/High)", repo.registered)
	}
}

func TestSLAEngineService_RegisterIncidentClocks_OtherServiceDoesNotMatch(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.factsByIncident["incident-4"] = repository.IncidentFacts{
		State:     "NEW",
		Category:  "SERVICE_INTERRUPTION",
		ServiceID: "11111111-2222-3333-4444-555555555555", // not Choreo or Asgardeo
	}
	svc := NewSLAEngineService(repo)

	priority := domain.IncidentPriorityCritical
	svc.RegisterIncidentClocks(context.Background(), "incident-4", &priority)

	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none: the policy is scoped to Choreo/Asgardeo only", repo.registered)
	}
}

func TestSLAEngineService_RegisterIncidentClocks_NonServiceInterruptionCategoryDoesNotMatch(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.factsByIncident["incident-5"] = repository.IncidentFacts{
		State:     "NEW",
		Category:  "INQUIRY",
		ServiceID: choreoServiceID,
	}
	svc := NewSLAEngineService(repo)

	priority := domain.IncidentPriorityCritical
	svc.RegisterIncidentClocks(context.Background(), "incident-5", &priority)

	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none: category=service_interruption is required", repo.registered)
	}
}

func TestSLAEngineService_ApplyIncidentStateEffects_CompletesOnResolved(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.activeByCaseTarget["incident-6|RESOLUTION"] = cloudIncidentPolicyCandidates()["RESOLUTION"][0]
	repo.factsByIncident["incident-6"] = repository.IncidentFacts{
		State:     "RESOLVED",
		Category:  "SERVICE_INTERRUPTION",
		ServiceID: choreoServiceID,
	}
	svc := NewSLAEngineService(repo)

	svc.ApplyIncidentStateEffects(context.Background(), "incident-6", domain.IncidentStateResolved)

	want := "incident-6|RESOLUTION"
	if len(repo.completed) != 1 || repo.completed[0] != want {
		t.Fatalf("completed = %v, want [%s]: RESOLVED is one of the policy's own stop_condition states (stateIN3,6,7 -> ON_HOLD,RESOLVED,CLOSED)", repo.completed, want)
	}
}

func TestSLAEngineService_ApplyIncidentStateEffects_StateDBLabelNormalizesCancelledTypo(t *testing.T) {
	// Regression test for the exact one-letter mismatch this package's own
	// doc comments warn about: domain.IncidentStateCancelled is spelled
	// "CANCELLED" (two L's) but the real stored incident_state_enum value,
	// and this translator's own output, is "CANCELED" (one L). Passing the
	// domain constant as ApplyIncidentStateEffects' stateOverride must still
	// correctly cancel the clock -- not silently fail to match by one
	// letter.
	repo := newFakeFullSLAEngineRepo()
	repo.candidatesByTarget = cloudIncidentPolicyCandidates()
	repo.activeByCaseTarget["incident-7|RESOLUTION"] = cloudIncidentPolicyCandidates()["RESOLUTION"][0]
	repo.factsByIncident["incident-7"] = repository.IncidentFacts{
		State:     "IN_PROGRESS", // stale fresh-read value; override below takes precedence
		Category:  "SERVICE_INTERRUPTION",
		Priority:  "CRITICAL", // isolates the cancel_condition's OR'd "^NQ" groups to the state one
		ServiceID: choreoServiceID,
	}
	svc := NewSLAEngineService(repo)

	svc.ApplyIncidentStateEffects(context.Background(), "incident-7", domain.IncidentStateCancelled)

	want := "incident-7|RESOLUTION"
	if len(repo.clockCancelled) != 1 || repo.clockCancelled[0] != want {
		t.Fatalf("clockCancelled = %v, want [%s]: domain.IncidentStateCancelled must normalize to the real \"CANCELED\" (one L) enum label",
			repo.clockCancelled, want)
	}
}
