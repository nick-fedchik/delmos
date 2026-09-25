package project_test

import (
	"context"
	"errors"
	"testing"

	"delmos/internal/project"
)

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
	if len(plan.Manifest.Objectives) != 1 || plan.Manifest.Objectives[0].Key != "OBJ-001" {
		t.Fatalf("очікувався початковий objective шаблону, отримано %+v", plan.Manifest.Objectives)
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
	manifest := project.DefaultGenericPlanManifest("Project")
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
