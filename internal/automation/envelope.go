// Package automation реалізує рушій подій, тригерів, правил, FSM-переходів та
// планувальника (EVENTS_TRIGGERS_RULES.md, ADR-007, SPEC-04). Пакет свідомо не
// імпортує internal/project: обробники прив'язуються ззовні (RegisterHandler),
// щоб уникнути циклічного імпорту та тримати рушій незалежним від доменів.
package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Envelope — конверт події (SPEC-04 §1): незмінний факт зміни стану,
// записаний у тій самій транзакції, що й бізнес-зміна.
type Envelope struct {
	EventID       uuid.UUID      `json:"event_id"`
	EventKey      string         `json:"event_key"`
	SchemaVersion string         `json:"schema_version"`
	Source        string         `json:"source"`
	OccurredAt    time.Time      `json:"occurred_at"`
	ScopeType     string         `json:"scope_type"`
	ScopeID       *uuid.UUID     `json:"scope_id,omitempty"`
	ActorUserID   *uuid.UUID     `json:"actor_user_id,omitempty"`
	CorrelationID uuid.UUID      `json:"correlation_id"`
	CausationID   uuid.UUID      `json:"causation_id"`
	Payload       map[string]any `json:"payload"`
}

// EmitEvent записує конверт події в core.event_outbox у переданій транзакції
// (Transactional Outbox — ADR-007): подія стає видимою іншим лише після
// commit тієї самої транзакції, що й бізнес-зміна.
func EmitEvent(ctx context.Context, tx pgx.Tx, eventKey string, projectID *uuid.UUID, actorUserID *uuid.UUID, correlationID uuid.UUID, payload map[string]any) error {
	envelope := Envelope{
		EventID:       uuid.New(),
		EventKey:      eventKey,
		SchemaVersion: "1.0.0",
		Source:        "delmos.core",
		OccurredAt:    time.Now().UTC(),
		ScopeType:     "project",
		ScopeID:       projectID,
		ActorUserID:   actorUserID,
		CorrelationID: correlationID,
		CausationID:   correlationID,
		Payload:       payload,
	}
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("серіалізація конверта події %s: %w", eventKey, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO core.event_outbox (event_key, project_id, envelope) VALUES ($1, $2, $3)`,
		eventKey, projectID, envelopeJSON); err != nil {
		return fmt.Errorf("запис події %s у outbox: %w", eventKey, err)
	}
	return nil
}
