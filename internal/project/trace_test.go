package project_test

import (
	"context"
	"testing"
)

// TestTraverseTraceabilityDetectsCycle перевіряє GRP-01/GRP-02: рекурсивний
// обхід графа повертає повний ланцюжок довільної глибини й коректно
// завершується на циклічному ребрі, позначаючи його is_cycle без зависання.
func TestTraverseTraceabilityDetectsCycle(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "trace-author")
	proj, err := store.CreateWithPlan(ctx, userID, "TRACE-001", "Trace project", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	a, _, err := store.CreateWorkProduct(ctx, userID, proj.Project.ID, "A-001", "requirement", "A", "body", nil)
	if err != nil {
		t.Fatalf("створення A: %v", err)
	}
	b, _, err := store.CreateWorkProduct(ctx, userID, proj.Project.ID, "B-001", "requirement", "B", "body", nil)
	if err != nil {
		t.Fatalf("створення B: %v", err)
	}
	c, _, err := store.CreateWorkProduct(ctx, userID, proj.Project.ID, "C-001", "requirement", "C", "body", nil)
	if err != nil {
		t.Fatalf("створення C: %v", err)
	}

	if _, err := store.CreateTraceLink(ctx, userID, proj.Project.ID, a.ID, b.ID, nil, nil, "derives_from"); err != nil {
		t.Fatalf("зв'язок A->B: %v", err)
	}
	if _, err := store.CreateTraceLink(ctx, userID, proj.Project.ID, b.ID, c.ID, nil, nil, "derives_from"); err != nil {
		t.Fatalf("зв'язок B->C: %v", err)
	}
	if _, err := store.CreateTraceLink(ctx, userID, proj.Project.ID, c.ID, a.ID, nil, nil, "derives_from"); err != nil {
		t.Fatalf("зв'язок C->A (замикає цикл): %v", err)
	}

	entries, err := store.TraverseTraceability(ctx, proj.Project.ID, a.ID)
	if err != nil {
		t.Fatalf("обхід графа: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("очікувалося 3 ребра (A->B, B->C, C->A), отримано %d: %+v", len(entries), entries)
	}
	last := entries[len(entries)-1]
	if !last.IsCycle || last.TargetCode != "A-001" {
		t.Fatalf("останнє ребро мало бути позначене is_cycle=true на поверненні до A-001, отримано %+v", last)
	}
	for _, entry := range entries[:len(entries)-1] {
		if entry.IsCycle {
			t.Fatalf("нециклічне ребро помилково позначене is_cycle: %+v", entry)
		}
	}
}

// TestReviseWorkProductPropagatesSuspectFlag перевіряє GRP-03 (Change Impact
// Analysis): нова ревізія джерела каскадно позначає is_suspect=true на всіх
// прямих і непрямих залежних ребрах; явне підтвердження знімає прапорець.
func TestReviseWorkProductPropagatesSuspectFlag(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "impact-author")
	proj, err := store.CreateWithPlan(ctx, userID, "IMPACT-001", "Impact project", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	req, _, err := store.CreateWorkProduct(ctx, userID, proj.Project.ID, "REQ-001", "requirement", "Req", "body", nil)
	if err != nil {
		t.Fatalf("створення вимоги: %v", err)
	}
	test, _, err := store.CreateWorkProduct(ctx, userID, proj.Project.ID, "TC-001", "test_spec", "Test", "body", nil)
	if err != nil {
		t.Fatalf("створення тесту: %v", err)
	}
	report, _, err := store.CreateWorkProduct(ctx, userID, proj.Project.ID, "REP-001", "report", "Report", "body", nil)
	if err != nil {
		t.Fatalf("створення звіту: %v", err)
	}

	// TC-001 verifies REQ-001 (source=TC-001, target=REQ-001):
	// тест спирається на вимогу.
	verifiesLink, err := store.CreateTraceLink(ctx, userID, proj.Project.ID, test.ID, req.ID, nil, nil, "verifies")
	if err != nil {
		t.Fatalf("зв'язок TC->REQ: %v", err)
	}
	// REP-001 satisfies TC-001 (source=REP-001, target=TC-001):
	// звіт спирається на результат тесту — другий рівень залежності.
	satisfiesLink, err := store.CreateTraceLink(ctx, userID, proj.Project.ID, report.ID, test.ID, nil, nil, "satisfies")
	if err != nil {
		t.Fatalf("зв'язок REP->TC: %v", err)
	}

	if _, err := store.ReviseWorkProduct(ctx, userID, proj.Project.ID, req.ID, 1, "оновлений текст вимоги", nil); err != nil {
		t.Fatalf("ревізія вимоги: %v", err)
	}

	links, err := store.ListTraceLinks(ctx, proj.Project.ID, false)
	if err != nil {
		t.Fatalf("читання зв'язків: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("очікувалося 2 зв'язки, отримано %d", len(links))
	}
	for _, link := range links {
		if !link.IsSuspect {
			t.Fatalf("зв'язок %s мав отримати is_suspect=true після ревізії REQ-001: %+v", link.ID, link)
		}
	}

	suspectOnly, err := store.ListTraceLinks(ctx, proj.Project.ID, true)
	if err != nil {
		t.Fatalf("читання підозрілих зв'язків: %v", err)
	}
	if len(suspectOnly) != 2 {
		t.Fatalf("очікувалося 2 підозрілі зв'язки, отримано %d", len(suspectOnly))
	}

	if err := store.AcknowledgeTraceLink(ctx, userID, proj.Project.ID, verifiesLink.ID); err != nil {
		t.Fatalf("зняття підозрілості з TC->REQ: %v", err)
	}
	after, err := store.ListTraceLinks(ctx, proj.Project.ID, true)
	if err != nil {
		t.Fatalf("читання підозрілих зв'язків після підтвердження: %v", err)
	}
	if len(after) != 1 || after[0].ID != satisfiesLink.ID {
		t.Fatalf("після підтвердження мав лишитися лише REP->TC підозрілим, отримано %+v", after)
	}
}
