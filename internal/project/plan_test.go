package project_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"delmos/internal/project"
)

func TestDefaultGenericPlanManifestMatchesTemplate(t *testing.T) {
	data, err := os.ReadFile("../../docs/templates/GENERIC-PROJECT-PLAN-TEMPLATE-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var template struct {
		Manifest project.GenericPlanManifest `json:"manifest"`
	}
	if err := json.Unmarshal(data, &template); err != nil {
		t.Fatal(err)
	}
	actual := project.DefaultGenericPlanManifest()
	if !reflect.DeepEqual(actual, template.Manifest) {
		t.Fatalf("початковий план не збігається з шаблоном: отримано %+v, шаблон %+v", actual, template.Manifest)
	}
	if len(actual.Objectives) != 0 {
		t.Fatal("новий план не повинен вигадувати цілі за проєктного менеджера")
	}
}

func TestProjectCreatesGenericPlanTemplateAndRevision(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "creator")
	created, err := store.CreateWithPlan(ctx, userID, "PLAN-001", "Plan Project", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	plan, err := store.GetPlan(ctx, created.Project.ID)
	if err != nil {
		t.Fatalf("читання Generic Plan: %v", err)
	}
	if plan.TemplateKey != "generic-project-plan" || plan.TemplateVersion != 1 || plan.Revision.RevisionNumber != 1 {
		t.Fatalf("очікувався generic-project-plan@1/r1, отримано %+v", plan)
	}
	if len(plan.Manifest.Objectives) != 0 {
		t.Fatalf("новий план не повинен вигадувати ціль за ПМ, отримано %+v", plan.Manifest.Objectives)
	}

	plan.Manifest.Phases = []project.PlanPhase{{Key: "PH-001", Name: "Delivery", PlannedStart: "2026-10-01", PlannedFinish: "2026-10-31"}}
	revised, err := store.RevisePlan(ctx, userID, created.Project.ID, plan.WorkProduct.RowVersion, "# Project plan\n", plan.Manifest)
	if err != nil {
		t.Fatalf("створення ревізії плану: %v", err)
	}
	if revised.Revision.RevisionNumber != 2 || revised.WorkProduct.RowVersion != 2 {
		t.Fatalf("очікувалася immutable r2 та row_version=2, отримано %+v", revised)
	}
}

func TestGenericPlanRejectsCyclicPhaseGraph(t *testing.T) {
	manifest := project.DefaultGenericPlanManifest()
	manifest.Phases = []project.PlanPhase{
		{Key: "PH-001", Name: "First", PlannedStart: "2026-10-01", PlannedFinish: "2026-10-02", DependsOn: []string{"PH-002"}},
		{Key: "PH-002", Name: "Second", PlannedStart: "2026-10-03", PlannedFinish: "2026-10-04", DependsOn: []string{"PH-001"}},
	}
	if err := project.ValidateGenericPlanManifest(manifest); !errors.Is(err, project.ErrInvalidPlanManifest) {
		t.Errorf("очікувалася помилка циклу фаз, отримано %v", err)
	}
}

func TestEntityRegistryContainsGenericPlanDomain(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	definitions, err := store.ListEntityDefinitions(ctx)
	if err != nil {
		t.Fatalf("читання реєстру сутностей: %v", err)
	}
	found := map[string]bool{}
	for _, definition := range definitions {
		found[definition.Key] = true
	}
	for _, key := range []string{"project_plan", "phase", "milestone", "risk", "issue", "change_request", "decision", "gate_decision"} {
		if !found[key] {
			t.Errorf("реєстр не містить базову сутність %s", key)
		}
	}
}
