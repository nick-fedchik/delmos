package project_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"delmos/internal/project"

	"github.com/google/uuid"
)

func TestCreateSpecificationStoresImmutableManifest(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "spec-author")
	first, _ := store.CreateWithPlan(ctx, userID, "SPEC-PROJECT", "Specification project", "")
	target, revision, err := store.CreateWorkProduct(ctx, userID, first.Project.ID, "REQ-001", "requirement", "Requirement", "body", nil)
	if err != nil {
		t.Fatalf("створення вимоги: %v", err)
	}

	manifest := project.CompositionManifest{
		ManifestVersion: "1.0.0",
		Sections:        []project.CompositionSection{{SectionID: "root", Title: "Requirements", Order: 1}},
		Occurrences: []project.CompositionOccurrence{{
			OccurrenceID:        uuid.New(),
			SectionID:           "root",
			TargetWorkProductID: target.ID,
			TargetRevisionID:    revision.ID,
			TargetPayloadHash:   fmt.Sprintf("%x", revision.PayloadHash),
			Order:               1,
		}},
	}
	manifest.ElementsHash = project.CalculateElementsHash([]project.CompositionElement{{
		TargetWorkProductID: target.ID.String(),
		TargetRevisionID:    revision.ID.String(),
		TargetPayloadHash:   fmt.Sprintf("%x", revision.PayloadHash),
	}})

	workProduct, specRevision, specification, err := store.CreateSpecification(ctx, userID, first.Project.ID, "SPEC-001", "System specification", "body", nil, manifest)
	if err != nil {
		t.Fatalf("створення специфікації: %v", err)
	}
	if workProduct.Profile != "core:specification" || specRevision.RevisionNumber != 1 || specification.ID == uuid.Nil {
		t.Fatalf("некоректний результат створення специфікації: %+v %+v %+v", workProduct, specRevision, specification)
	}
}

func TestCalculateElementsHashIsIndependentOfOccurrenceOrder(t *testing.T) {
	first := []project.CompositionElement{
		{TargetWorkProductID: "wp-b", TargetRevisionID: "rev-2", TargetPayloadHash: "hash-b"},
		{TargetWorkProductID: "wp-a", TargetRevisionID: "rev-1", TargetPayloadHash: "hash-a"},
	}
	second := []project.CompositionElement{first[1], first[0]}

	if !bytes.Equal(project.CalculateElementsHash(first), project.CalculateElementsHash(second)) {
		t.Fatal("перестановка входжень не повинна змінювати elements_hash")
	}
}

func TestCompositionManifestValidatesStructureAndElementsHash(t *testing.T) {
	parent := "root"
	workProductID := uuid.New()
	revisionID := uuid.New()
	payloadHash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	manifest := project.CompositionManifest{
		ManifestVersion: "1.0.0",
		Sections: []project.CompositionSection{
			{SectionID: "root", Title: "Root", Order: 1},
			{SectionID: "child", ParentSectionID: &parent, Title: "Child", Order: 2},
		},
		Occurrences: []project.CompositionOccurrence{{
			OccurrenceID:        uuid.New(),
			SectionID:           "child",
			TargetWorkProductID: workProductID,
			TargetRevisionID:    revisionID,
			TargetPayloadHash:   payloadHash,
			Order:               1,
		}},
	}
	manifest.ElementsHash = project.CalculateElementsHash([]project.CompositionElement{{
		TargetWorkProductID: workProductID.String(),
		TargetRevisionID:    revisionID.String(),
		TargetPayloadHash:   payloadHash,
	}})

	if err := manifest.Validate(); err != nil {
		t.Fatalf("валідний маніфест відхилено: %v", err)
	}
}

func TestCompositionManifestRejectsCyclesAndHashMismatch(t *testing.T) {
	firstParent := "second"
	secondParent := "first"
	manifest := project.CompositionManifest{
		ManifestVersion: "1.0.0",
		Sections: []project.CompositionSection{
			{SectionID: "first", ParentSectionID: &firstParent, Title: "First", Order: 1},
			{SectionID: "second", ParentSectionID: &secondParent, Title: "Second", Order: 2},
		},
		ElementsHash: make([]byte, 32),
	}

	if !errors.Is(manifest.Validate(), project.ErrCompositionCycle) {
		t.Fatal("цикл секцій має бути відхилений")
	}

	manifest.Sections = nil
	if !errors.Is(manifest.Validate(), project.ErrInvalidCompositionManifest) {
		t.Fatal("невідповідний elements_hash має бути відхилений")
	}
}

func TestCompositionManifestAllowsRepeatedRevisionOccurrences(t *testing.T) {
	workProductID := uuid.New()
	revisionID := uuid.New()
	payloadHash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	manifest := project.CompositionManifest{
		ManifestVersion: "1.0.0",
		Sections:        []project.CompositionSection{{SectionID: "root", Title: "Requirements", Order: 1}},
		Occurrences: []project.CompositionOccurrence{
			{OccurrenceID: uuid.New(), SectionID: "root", TargetWorkProductID: workProductID, TargetRevisionID: revisionID, TargetPayloadHash: payloadHash, Order: 1},
			{OccurrenceID: uuid.New(), SectionID: "root", TargetWorkProductID: workProductID, TargetRevisionID: revisionID, TargetPayloadHash: payloadHash, Order: 2},
		},
	}
	manifest.ElementsHash = project.CalculateElementsHash([]project.CompositionElement{
		{TargetWorkProductID: workProductID.String(), TargetRevisionID: revisionID.String(), TargetPayloadHash: payloadHash},
		{TargetWorkProductID: workProductID.String(), TargetRevisionID: revisionID.String(), TargetPayloadHash: payloadHash},
	})

	if err := manifest.Validate(); err != nil {
		t.Fatalf("повторне входження тієї самої ревізії має бути дозволене: %v", err)
	}
	if bytes.Equal(
		project.CalculateElementsHash([]project.CompositionElement{{TargetWorkProductID: workProductID.String(), TargetRevisionID: revisionID.String(), TargetPayloadHash: payloadHash}}),
		manifest.ElementsHash,
	) {
		t.Fatal("elements_hash має враховувати повторне входження")
	}
}

func TestCalculateElementsHashChangesWhenRevisionChanges(t *testing.T) {
	original := []project.CompositionElement{{
		TargetWorkProductID: "wp-a",
		TargetRevisionID:    "rev-1",
		TargetPayloadHash:   "hash-a",
	}}
	changed := []project.CompositionElement{{
		TargetWorkProductID: "wp-a",
		TargetRevisionID:    "rev-2",
		TargetPayloadHash:   "hash-a",
	}}

	if bytes.Equal(project.CalculateElementsHash(original), project.CalculateElementsHash(changed)) {
		t.Fatal("зміна цільової ревізії повинна змінювати elements_hash")
	}
}
