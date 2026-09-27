package project

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"delmos/internal/automation"
)

var (
	ErrMilestoneNotFound         = errors.New("віху не знайдено")
	ErrMilestoneNotReady         = errors.New("обов'язкові результати віхи не погоджено")
	ErrSelfGateDecision          = errors.New("автор результату або плану не може прийняти власну віху")
	ErrUnsupportedAcceptanceRule = errors.New("правило приймання віхи ще не підтримується")
)

type GateDecision struct {
	ID               uuid.UUID `json:"id"`
	MilestoneKey     string    `json:"milestone_key"`
	Outcome          string    `json:"outcome"`
	ConfigGeneration int64     `json:"config_generation"`
}

func (s *Store) AcceptMilestone(ctx context.Context, actorID, projectID uuid.UUID, key string) (GateDecision, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return GateDecision{}, fmt.Errorf("початок приймання віхи: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var milestoneID uuid.UUID
	var status string
	var generation int64
	var phaseKey string
	var deliverableKeys, ruleKeys []string
	err = tx.QueryRow(ctx, `SELECT id, phase_key, status, config_generation, deliverable_keys, acceptance_rule_keys
		FROM core.project_milestones WHERE project_id = $1 AND milestone_key = $2 FOR UPDATE`, projectID, key).
		Scan(&milestoneID, &phaseKey, &status, &generation, &deliverableKeys, &ruleKeys)
	if errors.Is(err, pgx.ErrNoRows) {
		return GateDecision{}, ErrMilestoneNotFound
	}
	if err != nil {
		return GateDecision{}, fmt.Errorf("читання віхи: %w", err)
	}

	var revisionID uuid.UUID
	var currentGeneration int64
	var planAuthor uuid.UUID
	var manifest GenericPlanManifest
	err = tx.QueryRow(ctx, `SELECT b.effective_plan_revision_id, b.config_generation, r.created_by, pm.manifest
		FROM core.project_plan_bindings b
		JOIN core.work_product_revisions r ON r.id = b.effective_plan_revision_id
		JOIN core.project_plan_manifests pm ON pm.revision_id = r.id
		WHERE b.project_id = $1`, projectID).
		Scan(&revisionID, &currentGeneration, &planAuthor, &manifest)
	if errors.Is(err, pgx.ErrNoRows) {
		return GateDecision{}, ErrPlanNotApproved
	}
	if err != nil {
		return GateDecision{}, fmt.Errorf("читання чинного плану: %w", err)
	}
	if generation != currentGeneration {
		return GateDecision{}, ErrPlanNotApproved
	}
	if actorID == planAuthor {
		return GateDecision{}, ErrSelfGateDecision
	}

	if status == "passed" {
		var decision GateDecision
		err = tx.QueryRow(ctx, `SELECT id, outcome, config_generation FROM core.gate_decisions
			WHERE milestone_id = $1 AND config_generation = $2 AND outcome = 'passed'
			ORDER BY decided_at DESC, id DESC LIMIT 1`, milestoneID, generation).
			Scan(&decision.ID, &decision.Outcome, &decision.ConfigGeneration)
		if err == nil {
			var invalid int
			err = tx.QueryRow(ctx, `SELECT count(*) FROM core.gate_decision_evidence e
				JOIN core.work_products wp ON wp.id = e.work_product_id
				WHERE e.gate_decision_id = $1 AND (wp.status <> 'approved' OR
				 e.revision_id <> (SELECT id FROM core.work_product_revisions
				 WHERE work_product_id = wp.id ORDER BY revision_number DESC LIMIT 1))`, decision.ID).Scan(&invalid)
			if err != nil {
				return GateDecision{}, err
			}
			var evidenceCount int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM core.gate_decision_evidence WHERE gate_decision_id = $1`, decision.ID).Scan(&evidenceCount); err != nil {
				return GateDecision{}, err
			}
			if invalid == 0 && evidenceCount == len(deliverableKeys) {
				decision.MilestoneKey = key
				return decision, tx.Commit(ctx)
			}
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			if err != nil {
				return GateDecision{}, err
			}
		} else {
			return GateDecision{}, ErrMilestoneNotReady
		}
	}
	if status != "pending" && status != "passed" {
		return GateDecision{}, ErrMilestoneNotReady
	}
	if len(ruleKeys) > 0 {
		return GateDecision{}, ErrUnsupportedAcceptanceRule
	}

	var milestone *PlanMilestone
	for index := range manifest.Milestones {
		if manifest.Milestones[index].Key == key {
			milestone = &manifest.Milestones[index]
			break
		}
	}
	if milestone == nil || milestone.PhaseKey != phaseKey || !sameKeys(milestone.DeliverableKeys, deliverableKeys) || !sameKeys(milestone.AcceptanceRuleKeys, ruleKeys) {
		return GateDecision{}, ErrMilestoneNotReady
	}

	correlationID := uuid.New()
	if err := automation.EnforceRules(ctx, tx, "trigger.core.before_milestone_accept",
		automation.EvalContext{Fields: map[string]any{
			"action": "milestone.accept", "milestone_key": key, "phase_key": phaseKey,
			"status": status, "target_status": "passed", "config_generation": generation,
		}}, actorID, projectID, correlationID); err != nil {
		return GateDecision{}, err
	}

	decision := GateDecision{ID: uuid.New(), MilestoneKey: key, Outcome: "passed", ConfigGeneration: generation}
	if _, err := tx.Exec(ctx, `INSERT INTO core.gate_decisions
		(id, project_id, milestone_id, plan_revision_id, config_generation, outcome, decided_by)
		VALUES ($1, $2, $3, $4, $5, 'passed', $6)`,
		decision.ID, projectID, milestoneID, revisionID, generation, actorID); err != nil {
		return GateDecision{}, fmt.Errorf("запис рішення: %w", err)
	}

	for _, deliverableKey := range deliverableKeys {
		var deliverable *Deliverable
		for index := range manifest.Deliverables {
			if manifest.Deliverables[index].Key == deliverableKey {
				deliverable = &manifest.Deliverables[index]
				break
			}
		}
		if deliverable == nil || deliverable.WorkProductID == "" {
			return GateDecision{}, ErrMilestoneNotReady
		}
		workProductID, err := uuid.Parse(deliverable.WorkProductID)
		if err != nil {
			return GateDecision{}, ErrMilestoneNotReady
		}
		var revision uuid.UUID
		var author uuid.UUID
		var hash []byte
		var workStatus string
		err = tx.QueryRow(ctx, `SELECT r.id, r.created_by, r.payload_hash, wp.status
			FROM core.work_products wp
			JOIN LATERAL (SELECT id, created_by, payload_hash FROM core.work_product_revisions
				WHERE work_product_id = wp.id ORDER BY revision_number DESC LIMIT 1) r ON true
			WHERE wp.id = $1 AND wp.project_id = $2 FOR SHARE OF wp`, workProductID, projectID).
			Scan(&revision, &author, &hash, &workStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return GateDecision{}, ErrMilestoneNotReady
		}
		if err != nil {
			return GateDecision{}, fmt.Errorf("читання результату: %w", err)
		}
		if actorID == author {
			return GateDecision{}, ErrSelfGateDecision
		}
		if workStatus != deliverable.RequiredStatus || workStatus != "approved" {
			return GateDecision{}, ErrMilestoneNotReady
		}
		if _, err := tx.Exec(ctx, `INSERT INTO core.gate_decision_evidence
			(gate_decision_id, work_product_id, revision_id, payload_hash) VALUES ($1, $2, $3, $4)`,
			decision.ID, workProductID, revision, hash); err != nil {
			return GateDecision{}, fmt.Errorf("запис доказу результату: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE core.project_milestones SET status = 'passed' WHERE id = $1`, milestoneID); err != nil {
		return GateDecision{}, fmt.Errorf("оновлення статусу віхи: %w", err)
	}
	if err := automation.EmitEvent(ctx, tx, "milestone.accepted", &projectID, &actorID, correlationID,
		map[string]any{"milestone_key": key, "decision_id": decision.ID.String()}); err != nil {
		return GateDecision{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO core.audit_events
		(actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		VALUES ($1, 'milestone.accepted', 'project', $2, 'success', $3, $4)`,
		actorID, projectID, map[string]any{"decision_id": decision.ID.String(), "milestone_key": key}, correlationID); err != nil {
		return GateDecision{}, err
	}
	return decision, tx.Commit(ctx)
}

func sameKeys(first, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}
