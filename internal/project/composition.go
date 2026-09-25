package project

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrInvalidCompositionManifest = errors.New("некоректний маніфест композиції")
	ErrCompositionCycle           = errors.New("маніфест композиції містить цикл")
)

type CompositionElement struct {
	TargetWorkProductID string `json:"target_wp_id"`
	TargetRevisionID    string `json:"target_revision_id"`
	TargetPayloadHash   string `json:"target_payload_hash"`
}

type CompositionSection struct {
	SectionID       string  `json:"section_id"`
	ParentSectionID *string `json:"parent_section_id"`
	Title           string  `json:"title"`
	Narrative       string  `json:"narrative,omitempty"`
	Order           int     `json:"order"`
}

type CompositionOccurrence struct {
	OccurrenceID        uuid.UUID `json:"occurrence_id"`
	SectionID           string    `json:"section_id"`
	TargetWorkProductID uuid.UUID `json:"target_wp_id"`
	TargetRevisionID    uuid.UUID `json:"target_revision_id"`
	TargetPayloadHash   string    `json:"target_payload_hash"`
	Order               int       `json:"order"`
}

type CompositionManifest struct {
	ManifestVersion string                  `json:"manifest_version"`
	Sections        []CompositionSection    `json:"sections"`
	Occurrences     []CompositionOccurrence `json:"occurrences"`
	ElementsHash    []byte                  `json:"elements_hash"`
}

func (manifest CompositionManifest) Validate() error {
	if manifest.ManifestVersion != "1.0.0" || len(manifest.ElementsHash) != sha256.Size {
		return ErrInvalidCompositionManifest
	}

	sections := make(map[string]CompositionSection, len(manifest.Sections))
	for _, section := range manifest.Sections {
		if section.SectionID == "" || section.Title == "" || section.Order < 1 {
			return ErrInvalidCompositionManifest
		}
		if _, exists := sections[section.SectionID]; exists {
			return fmt.Errorf("%w: дубльована секція %q", ErrInvalidCompositionManifest, section.SectionID)
		}
		sections[section.SectionID] = section
	}

	for _, section := range manifest.Sections {
		if section.ParentSectionID == nil {
			continue
		}
		if _, exists := sections[*section.ParentSectionID]; !exists {
			return fmt.Errorf("%w: батьківська секція %q не знайдена", ErrInvalidCompositionManifest, *section.ParentSectionID)
		}
		if hasSectionCycle(section.SectionID, sections) {
			return ErrCompositionCycle
		}
	}

	elements := make([]CompositionElement, 0, len(manifest.Occurrences))
	occurrences := make(map[uuid.UUID]struct{}, len(manifest.Occurrences))
	for _, occurrence := range manifest.Occurrences {
		if occurrence.OccurrenceID == uuid.Nil || occurrence.SectionID == "" || occurrence.Order < 1 ||
			occurrence.TargetWorkProductID == uuid.Nil || occurrence.TargetRevisionID == uuid.Nil ||
			!isSHA256Hex(occurrence.TargetPayloadHash) {
			return ErrInvalidCompositionManifest
		}
		if _, exists := sections[occurrence.SectionID]; !exists {
			return fmt.Errorf("%w: секція входження %q не знайдена", ErrInvalidCompositionManifest, occurrence.SectionID)
		}
		if _, exists := occurrences[occurrence.OccurrenceID]; exists {
			return fmt.Errorf("%w: дубльований occurrence_id %q", ErrInvalidCompositionManifest, occurrence.OccurrenceID)
		}
		occurrences[occurrence.OccurrenceID] = struct{}{}
		elements = append(elements, CompositionElement{
			TargetWorkProductID: occurrence.TargetWorkProductID.String(),
			TargetRevisionID:    occurrence.TargetRevisionID.String(),
			TargetPayloadHash:   occurrence.TargetPayloadHash,
		})
	}

	if !equalBytes(CalculateElementsHash(elements), manifest.ElementsHash) {
		return fmt.Errorf("%w: elements_hash не відповідає входженням", ErrInvalidCompositionManifest)
	}
	return nil
}

func CalculateElementsHash(elements []CompositionElement) []byte {
	canonicalElements := make([]string, 0, len(elements))
	for _, element := range elements {
		canonicalElements = append(canonicalElements, strings.Join([]string{
			element.TargetWorkProductID,
			element.TargetRevisionID,
			element.TargetPayloadHash,
		}, ":"))
	}
	sort.Strings(canonicalElements)

	sum := sha256.Sum256([]byte(strings.Join(canonicalElements, "\n")))
	return sum[:]
}

func hasSectionCycle(start string, sections map[string]CompositionSection) bool {
	visited := map[string]bool{}
	for current := start; current != ""; {
		if visited[current] {
			return true
		}
		visited[current] = true
		parent := sections[current].ParentSectionID
		if parent == nil {
			return false
		}
		current = *parent
	}
	return false
}

func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
