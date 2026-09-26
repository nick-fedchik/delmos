package project

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"delmos/internal/automation"
)

const (
	genericPlanTemplateKey     = "generic-project-plan"
	genericPlanTemplateVersion = 1
)

var (
	ErrPlanNotFound        = errors.New("план проєкту не знайдено")
	ErrPlanNotApproved     = errors.New("ревізію плану не погоджено")
	ErrInvalidPlanManifest = errors.New("некоректний маніфест Generic Project Plan")
	// ErrPlanPhaseHasActivity: фазу видалено з маніфесту, але в неї вже є записана
	// економічна активність (RESTRICT на work_records/expense_records) — тихе
	// видалення звело б частину історії без сліду.
	ErrPlanPhaseHasActivity = errors.New("фазу неможливо вилучити з плану: вона вже має записану активність")
)

var planKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

// GenericPlanManifest містить лише нейтральну конфігурацію ядра. Методології
// та галузеві правила додаються модулями до Extensions.
type GenericPlanManifest struct {
	Objectives                []ProjectObjective         `json:"objectives"`
	ScopeItems                []ScopeItem                `json:"scope_items"`
	Assumptions               []Assumption               `json:"assumptions"`
	Constraints               []Constraint               `json:"constraints"`
	ResponsibilityAssignments []ResponsibilityAssignment `json:"responsibility_assignments"`
	Deliverables              []Deliverable              `json:"deliverables"`
	Phases                    []PlanPhase                `json:"phases"`
	Milestones                []PlanMilestone            `json:"milestones"`
	AcceptanceRules           []AcceptanceRule           `json:"acceptance_rules"`
	Governance                PlanGovernance             `json:"governance"`
	Extensions                map[string]json.RawMessage `json:"extensions"`
}

type ProjectObjective struct {
	Key             string   `json:"key"`
	Statement       string   `json:"statement"`
	SuccessCriteria []string `json:"success_criteria"`
}

type ScopeItem struct {
	Key       string `json:"key"`
	Kind      string `json:"kind"`
	Statement string `json:"statement"`
	Rationale string `json:"rationale"`
}

type Assumption struct {
	Key            string `json:"key"`
	Statement      string `json:"statement"`
	OwnerReference string `json:"owner_reference"`
	ValidationDate string `json:"validation_date"`
	Status         string `json:"status"`
}

type Constraint struct {
	Key         string `json:"key"`
	Kind        string `json:"kind"`
	Statement   string `json:"statement"`
	SourceRef   string `json:"source_ref"`
	Enforcement string `json:"enforcement"`
}

type ResponsibilityAssignment struct {
	Key            string `json:"key"`
	RoleKey        string `json:"role_key"`
	Subject        string `json:"subject"`
	Responsibility string `json:"responsibility"`
}

type Deliverable struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	WorkProductID  string `json:"work_product_id"`
	RequiredStatus string `json:"required_status"`
}

type PlanPhase struct {
	Key           string   `json:"key"`
	Name          string   `json:"name"`
	PlannedStart  string   `json:"planned_start"`
	PlannedFinish string   `json:"planned_finish"`
	DependsOn     []string `json:"depends_on"`
}

type PlanMilestone struct {
	Key                string   `json:"key"`
	Name               string   `json:"name"`
	PhaseKey           string   `json:"phase_key"`
	TargetDate         string   `json:"target_date"`
	DeliverableKeys    []string `json:"deliverable_keys"`
	AcceptanceRuleKeys []string `json:"acceptance_rule_keys"`
}

type AcceptanceRule struct {
	Key          string         `json:"key"`
	PredicateKey string         `json:"predicate_key"`
	Parameters   map[string]any `json:"parameters"`
	Enforcement  string         `json:"enforcement"`
}

type PlanGovernance struct {
	ChangeControlRequired bool `json:"change_control_required"`
}

type ProjectPhase struct {
	PhaseKey      string   `json:"phase_key"`
	Name          string   `json:"name"`
	PlannedStart  string   `json:"planned_start"`
	PlannedFinish string   `json:"planned_finish"`
	Status        string   `json:"status"`
	DependsOn     []string `json:"depends_on"`
}

type ProjectMilestone struct {
	MilestoneKey       string   `json:"milestone_key"`
	PhaseKey           string   `json:"phase_key"`
	Name               string   `json:"name"`
	TargetDate         string   `json:"target_date"`
	Status             string   `json:"status"`
	DeliverableKeys    []string `json:"deliverable_keys"`
	AcceptanceRuleKeys []string `json:"acceptance_rule_keys"`
}

type PlanApplied struct {
	EffectivePlanRevisionID uuid.UUID `json:"effective_plan_revision_id"`
	ConfigGeneration        int64     `json:"config_generation"`
	RowVersion              int64     `json:"row_version"`
}

type PlanDetail struct {
	WorkProduct             WorkProduct
	Revision                WorkProductRevision
	Manifest                GenericPlanManifest
	TemplateKey             string
	TemplateVersion         int
	EffectivePlanRevisionID *uuid.UUID
	ConfigGeneration        int64
	Phases                  []ProjectPhase
	Milestones              []ProjectMilestone
}

type EntityDefinition struct {
	Key         string
	Name        string
	Category    string
	OwnerModule string
	Description string
}

func (s *Store) ListEntityDefinitions(ctx context.Context) ([]EntityDefinition, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT entity_key, name, category, owner_module, description
		 FROM core.entity_definitions ORDER BY category, entity_key`)
	if err != nil {
		return nil, fmt.Errorf("читання реєстру базових сутностей: %w", err)
	}
	defer rows.Close()

	definitions := []EntityDefinition{}
	for rows.Next() {
		var definition EntityDefinition
		if err := rows.Scan(&definition.Key, &definition.Name, &definition.Category, &definition.OwnerModule, &definition.Description); err != nil {
			return nil, fmt.Errorf("розбір базової сутності: %w", err)
		}
		definitions = append(definitions, definition)
	}
	return definitions, rows.Err()
}

func DefaultGenericPlanManifest(projectName string) GenericPlanManifest {
	return GenericPlanManifest{
		Objectives: []ProjectObjective{{
			Key: "OBJ-001", Statement: "Deliver " + projectName,
		}},
		ScopeItems: []ScopeItem{}, Assumptions: []Assumption{}, Constraints: []Constraint{},
		ResponsibilityAssignments: []ResponsibilityAssignment{}, Deliverables: []Deliverable{},
		Phases: []PlanPhase{}, Milestones: []PlanMilestone{}, AcceptanceRules: []AcceptanceRule{},
		Governance: PlanGovernance{ChangeControlRequired: true}, Extensions: map[string]json.RawMessage{},
	}
}

func ValidateGenericPlanManifest(manifest GenericPlanManifest) error {
	if manifest.Extensions == nil {
		return fmt.Errorf("%w: extensions обов'язковий", ErrInvalidPlanManifest)
	}
	keySets := []struct {
		name string
		keys []string
	}{
		{"objectives", objectiveKeys(manifest.Objectives)}, {"scope_items", scopeKeys(manifest.ScopeItems)},
		{"assumptions", assumptionKeys(manifest.Assumptions)}, {"constraints", constraintKeys(manifest.Constraints)},
		{"responsibility_assignments", responsibilityKeys(manifest.ResponsibilityAssignments)},
		{"deliverables", deliverableKeys(manifest.Deliverables)}, {"phases", phaseKeys(manifest.Phases)},
		{"milestones", milestoneKeys(manifest.Milestones)}, {"acceptance_rules", acceptanceRuleKeys(manifest.AcceptanceRules)},
	}
	for _, keySet := range keySets {
		if err := validateKeys(keySet.name, keySet.keys); err != nil {
			return err
		}
	}

	deliverables := keySet(deliverableKeys(manifest.Deliverables))
	for _, deliverable := range manifest.Deliverables {
		if deliverable.Name == "" || !validStatus(deliverable.RequiredStatus) {
			return fmt.Errorf("%w: deliverable %s має містити name та required_status", ErrInvalidPlanManifest, deliverable.Key)
		}
		if deliverable.WorkProductID != "" {
			if _, err := uuid.Parse(deliverable.WorkProductID); err != nil {
				return fmt.Errorf("%w: deliverable %s містить некоректний work_product_id", ErrInvalidPlanManifest, deliverable.Key)
			}
		}
	}

	phases := keySet(phaseKeys(manifest.Phases))
	for _, phase := range manifest.Phases {
		start, finish, err := dateRange(phase.PlannedStart, phase.PlannedFinish)
		if err != nil || finish.Before(start) {
			return fmt.Errorf("%w: фаза %s має некоректний діапазон дат", ErrInvalidPlanManifest, phase.Key)
		}
		seen := map[string]bool{}
		for _, dependency := range phase.DependsOn {
			if dependency == phase.Key || !phases[dependency] || seen[dependency] {
				return fmt.Errorf("%w: фаза %s має некоректну залежність %s", ErrInvalidPlanManifest, phase.Key, dependency)
			}
			seen[dependency] = true
		}
	}
	if hasPhaseCycle(manifest.Phases) {
		return fmt.Errorf("%w: граф залежностей фаз містить цикл", ErrInvalidPlanManifest)
	}

	rules := keySet(acceptanceRuleKeys(manifest.AcceptanceRules))
	for _, milestone := range manifest.Milestones {
		if !phases[milestone.PhaseKey] || milestone.Name == "" {
			return fmt.Errorf("%w: milestone %s посилається на відсутню фазу", ErrInvalidPlanManifest, milestone.Key)
		}
		if _, err := time.Parse(time.DateOnly, milestone.TargetDate); err != nil {
			return fmt.Errorf("%w: milestone %s має некоректну дату", ErrInvalidPlanManifest, milestone.Key)
		}
		for _, key := range milestone.DeliverableKeys {
			if !deliverables[key] {
				return fmt.Errorf("%w: milestone %s посилається на відсутній deliverable", ErrInvalidPlanManifest, milestone.Key)
			}
		}
		for _, key := range milestone.AcceptanceRuleKeys {
			if !rules[key] {
				return fmt.Errorf("%w: milestone %s посилається на відсутнє правило", ErrInvalidPlanManifest, milestone.Key)
			}
		}
	}
	for _, item := range manifest.ScopeItems {
		if (item.Kind != "in_scope" && item.Kind != "out_of_scope") || item.Statement == "" {
			return fmt.Errorf("%w: scope item %s некоректний", ErrInvalidPlanManifest, item.Key)
		}
	}
	return nil
}

func (s *Store) GetPlan(ctx context.Context, projectID uuid.UUID) (PlanDetail, error) {
	var detail PlanDetail
	err := s.pool.QueryRow(ctx,
		`SELECT wp.id, wp.project_id, wp.code, wp.type, wp.profile, wp.title, wp.status, wp.classification, wp.row_version,
		        r.id, r.revision_number, r.body, r.metadata, r.payload_hash, r.content_hash, r.created_by, r.created_at,
		        pm.template_key, pm.template_version, pm.manifest,
		        b.effective_plan_revision_id, b.config_generation
		 FROM core.project_plan_bindings b
		 JOIN core.work_products wp ON wp.id = b.plan_work_product_id
		 JOIN core.work_product_revisions r ON r.work_product_id = wp.id
		 JOIN core.project_plan_manifests pm ON pm.revision_id = r.id
		 WHERE b.project_id = $1 ORDER BY r.revision_number DESC LIMIT 1`, projectID,
	).Scan(&detail.WorkProduct.ID, &detail.WorkProduct.ProjectID, &detail.WorkProduct.Code, &detail.WorkProduct.Type,
		&detail.WorkProduct.Profile, &detail.WorkProduct.Title, &detail.WorkProduct.Status, &detail.WorkProduct.Classification, &detail.WorkProduct.RowVersion,
		&detail.Revision.ID, &detail.Revision.RevisionNumber, &detail.Revision.Body, &detail.Revision.Metadata, &detail.Revision.PayloadHash,
		&detail.Revision.ContentHash, &detail.Revision.CreatedBy, &detail.Revision.CreatedAt,
		&detail.TemplateKey, &detail.TemplateVersion, &detail.Manifest,
		&detail.EffectivePlanRevisionID, &detail.ConfigGeneration)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanDetail{}, ErrPlanNotFound
	}
	if err != nil {
		return PlanDetail{}, fmt.Errorf("читання Generic Project Plan: %w", err)
	}
	detail.Revision.WorkProductID = detail.WorkProduct.ID

	// Читання проекції фаз виконання
	pRows, err := s.pool.Query(ctx,
		`SELECT phase_key, name, COALESCE(planned_start::text, ''), COALESCE(planned_finish::text, ''), status, depends_on
		 FROM core.project_phases WHERE project_id = $1 ORDER BY planned_start ASC NULLS LAST, phase_key ASC`, projectID)
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var p ProjectPhase
			if err := pRows.Scan(&p.PhaseKey, &p.Name, &p.PlannedStart, &p.PlannedFinish, &p.Status, &p.DependsOn); err == nil {
				detail.Phases = append(detail.Phases, p)
			}
		}
	}

	// Читання проекції віх виконання
	mRows, err := s.pool.Query(ctx,
		`SELECT milestone_key, phase_key, name, COALESCE(target_date::text, ''), status, deliverable_keys, acceptance_rule_keys
		 FROM core.project_milestones WHERE project_id = $1 ORDER BY target_date ASC NULLS LAST, milestone_key ASC`, projectID)
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var m ProjectMilestone
			if err := mRows.Scan(&m.MilestoneKey, &m.PhaseKey, &m.Name, &m.TargetDate, &m.Status, &m.DeliverableKeys, &m.AcceptanceRuleKeys); err == nil {
				detail.Milestones = append(detail.Milestones, m)
			}
		}
	}

	return detail, nil
}

func (s *Store) RevisePlan(ctx context.Context, actorID, projectID uuid.UUID, expectedRowVersion int64, body string, manifest GenericPlanManifest) (PlanDetail, error) {
	if err := ValidateGenericPlanManifest(manifest); err != nil {
		return PlanDetail{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PlanDetail{}, fmt.Errorf("початок транзакції ревізії плану: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var wp WorkProduct
	err = tx.QueryRow(ctx,
		`SELECT wp.id, wp.project_id, wp.code, wp.type, wp.profile, wp.title, wp.status, wp.classification, wp.row_version
		 FROM core.project_plan_bindings b JOIN core.work_products wp ON wp.id = b.plan_work_product_id
		 WHERE b.project_id = $1 FOR UPDATE`, projectID,
	).Scan(&wp.ID, &wp.ProjectID, &wp.Code, &wp.Type, &wp.Profile, &wp.Title, &wp.Status, &wp.Classification, &wp.RowVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanDetail{}, ErrPlanNotFound
	}
	if err != nil {
		return PlanDetail{}, fmt.Errorf("читання плану для ревізії: %w", err)
	}
	if wp.RowVersion != expectedRowVersion {
		return PlanDetail{}, ErrVersionConflict
	}

	for _, deliverable := range manifest.Deliverables {
		if deliverable.WorkProductID == "" {
			continue
		}
		var found bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM core.work_products WHERE id = $1 AND project_id = $2)`, deliverable.WorkProductID, projectID).Scan(&found); err != nil {
			return PlanDetail{}, fmt.Errorf("перевірка deliverable: %w", err)
		}
		if !found {
			return PlanDetail{}, fmt.Errorf("%w: deliverable %s належить іншому проєкту або не існує", ErrInvalidPlanManifest, deliverable.Key)
		}
	}

	var lastNumber int
	if err := tx.QueryRow(ctx, `SELECT max(revision_number) FROM core.work_product_revisions WHERE work_product_id = $1`, wp.ID).Scan(&lastNumber); err != nil {
		return PlanDetail{}, fmt.Errorf("читання версії плану: %w", err)
	}
	metadata := map[string]any{"plan_schema_version": "1.0.0", "template_key": genericPlanTemplateKey, "template_version": genericPlanTemplateVersion}
	payloadHash, contentHash := canonicalPlanHashes(wp.Code, wp.Type, wp.Profile, wp.Title, body, metadata, manifest)
	manifestHash := canonicalManifestHash(manifest)
	var revision WorkProductRevision
	err = tx.QueryRow(ctx,
		`INSERT INTO core.work_product_revisions (work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by, created_at`,
		wp.ID, lastNumber+1, body, metadata, payloadHash, contentHash, actorID,
	).Scan(&revision.ID, &revision.WorkProductID, &revision.RevisionNumber, &revision.Body, &revision.Metadata, &revision.PayloadHash, &revision.ContentHash, &revision.CreatedBy, &revision.CreatedAt)
	if err != nil {
		return PlanDetail{}, fmt.Errorf("створення ревізії плану: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO core.project_plan_manifests (revision_id, template_key, template_version, manifest, manifest_hash) VALUES ($1, $2, $3, $4, $5)`, revision.ID, genericPlanTemplateKey, genericPlanTemplateVersion, manifest, manifestHash); err != nil {
		return PlanDetail{}, fmt.Errorf("збереження маніфесту плану: %w", err)
	}
	if tag, err := tx.Exec(ctx, `UPDATE core.work_products SET row_version = row_version + 1 WHERE id = $1 AND row_version = $2`, wp.ID, expectedRowVersion); err != nil || tag.RowsAffected() == 0 {
		if err != nil {
			return PlanDetail{}, fmt.Errorf("оновлення версії плану: %w", err)
		}
		return PlanDetail{}, ErrVersionConflict
	}
	if err := s.recordProjectAuditTx(ctx, tx, actorID, projectID, "plan.revise", map[string]any{"revision_number": revision.RevisionNumber}); err != nil {
		return PlanDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PlanDetail{}, fmt.Errorf("фіксація ревізії плану: %w", err)
	}
	wp.RowVersion++
	return PlanDetail{WorkProduct: wp, Revision: revision, Manifest: manifest, TemplateKey: genericPlanTemplateKey, TemplateVersion: genericPlanTemplateVersion}, nil
}

func (s *Store) ApplyPlan(ctx context.Context, actorID, projectID uuid.UUID, revisionID *uuid.UUID, expectedRowVersion int64) (PlanApplied, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PlanApplied{}, fmt.Errorf("початок транзакції застосування плану: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var wp WorkProduct
	var currentGen int64
	var currentEffectiveRevID *uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT wp.id, wp.project_id, wp.code, wp.type, wp.profile, wp.title, wp.status, wp.classification, wp.row_version,
		        b.config_generation, b.effective_plan_revision_id
		 FROM core.project_plan_bindings b
		 JOIN core.work_products wp ON wp.id = b.plan_work_product_id
		 WHERE b.project_id = $1 FOR UPDATE`, projectID,
	).Scan(&wp.ID, &wp.ProjectID, &wp.Code, &wp.Type, &wp.Profile, &wp.Title, &wp.Status, &wp.Classification, &wp.RowVersion, &currentGen, &currentEffectiveRevID)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanApplied{}, ErrPlanNotFound
	}
	if err != nil {
		return PlanApplied{}, fmt.Errorf("читання плану для застосування: %w", err)
	}
	if wp.RowVersion != expectedRowVersion {
		return PlanApplied{}, ErrVersionConflict
	}

	var targetRevID uuid.UUID
	var targetRevNum int
	var manifest GenericPlanManifest
	if revisionID != nil && *revisionID != uuid.Nil {
		err = tx.QueryRow(ctx,
			`SELECT r.id, r.revision_number, pm.manifest
			 FROM core.work_product_revisions r
			 JOIN core.project_plan_manifests pm ON pm.revision_id = r.id
			 WHERE r.id = $1 AND r.work_product_id = $2`, *revisionID, wp.ID,
		).Scan(&targetRevID, &targetRevNum, &manifest)
	} else {
		err = tx.QueryRow(ctx,
			`SELECT r.id, r.revision_number, pm.manifest
			 FROM core.work_product_revisions r
			 JOIN core.project_plan_manifests pm ON pm.revision_id = r.id
			 WHERE r.work_product_id = $1
			 ORDER BY r.revision_number DESC LIMIT 1`, wp.ID,
		).Scan(&targetRevID, &targetRevNum, &manifest)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanApplied{}, fmt.Errorf("ревізію плану не знайдено")
	}
	if err != nil {
		return PlanApplied{}, fmt.Errorf("читання ревізії для застосування: %w", err)
	}

	if err := ValidateGenericPlanManifest(manifest); err != nil {
		return PlanApplied{}, err
	}

	// ADR-009 §5: план є артефактом і вводиться в дію лише після погодження
	// ТІЄЇ САМОЇ ревізії. Перевіряється наявність погодження на конкретну
	// ревізію, а не статус артефакту: після першого затвердження план
	// лишається approved, і нова непогоджена ревізія інакше пройшла б.
	approved, err := planRevisionApproved(ctx, tx, targetRevID)
	if err != nil {
		return PlanApplied{}, err
	}
	if !approved {
		return PlanApplied{}, fmt.Errorf("%w: ревізія %s", ErrPlanNotApproved, targetRevID)
	}

	// SWR-46.2: повторне застосування тієї самої ревізії ідемпотентне: без
	// цього клієнт, що повторив запит після тайм-ауту, отримав би друге
	// покоління та другий запис у project_plan_applications, хоча чинна конфігурація
	// не змінилася.
	if currentEffectiveRevID != nil && *currentEffectiveRevID == targetRevID {
		if err := tx.Commit(ctx); err != nil {
			return PlanApplied{}, fmt.Errorf("фіксація повторного застосування плану: %w", err)
		}
		return PlanApplied{
			EffectivePlanRevisionID: targetRevID,
			ConfigGeneration:        currentGen,
			RowVersion:              wp.RowVersion,
		}, nil
	}

	newGeneration := currentGen + 1
	if _, err := tx.Exec(ctx,
		`UPDATE core.project_plan_bindings
		 SET effective_plan_revision_id = $1, config_generation = $2
		 WHERE project_id = $3`,
		targetRevID, newGeneration, projectID); err != nil {
		return PlanApplied{}, fmt.Errorf("оновлення binding плану: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO core.project_plan_applications (project_id, plan_revision_id, config_generation, applied_by)
		 VALUES ($1, $2, $3, $4)`,
		projectID, targetRevID, newGeneration, actorID); err != nil {
		return PlanApplied{}, fmt.Errorf("запис факту застосування плану: %w", err)
	}

	// Проєкція фаз у core.project_phases
	for _, ph := range manifest.Phases {
		var startVal, finishVal *string
		if ph.PlannedStart != "" {
			s := ph.PlannedStart
			startVal = &s
		}
		if ph.PlannedFinish != "" {
			f := ph.PlannedFinish
			finishVal = &f
		}
		depends := ph.DependsOn
		if depends == nil {
			depends = []string{}
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO core.project_phases (project_id, phase_key, name, planned_start, planned_finish, status, depends_on, config_generation)
			 VALUES ($1, $2, $3, $4::date, $5::date, 'not_started', $6, $7)
			 ON CONFLICT (project_id, phase_key) DO UPDATE
			 SET name = EXCLUDED.name,
			     planned_start = EXCLUDED.planned_start,
			     planned_finish = EXCLUDED.planned_finish,
			     depends_on = EXCLUDED.depends_on,
			     config_generation = EXCLUDED.config_generation`,
			projectID, ph.Key, ph.Name, startVal, finishVal, depends, newGeneration); err != nil {
			return PlanApplied{}, fmt.Errorf("проєкція фази %s: %w", ph.Key, err)
		}
	}

	// Нова ревізія може вилучити фазу: без цього видалена з маніфесту фаза
	// лишалася б у проєкції назавжди (CORE-CONTRACT-002 §5). Фаза з уже
	// записаною активністю не видаляється мовчки: RESTRICT на work_records/
	// expense_records перетворюється в ясний конфлікт, а не опакуву помилку СУБД.
	if _, err := tx.Exec(ctx,
		`DELETE FROM core.project_phases WHERE project_id = $1 AND NOT (phase_key = ANY($2))`,
		projectID, phaseKeys(manifest.Phases)); err != nil {
		if isForeignKeyViolation(err) {
			return PlanApplied{}, ErrPlanPhaseHasActivity
		}
		return PlanApplied{}, fmt.Errorf("вилучення застарілих фаз: %w", err)
	}

	// Проєкція віх у core.project_milestones
	for _, ms := range manifest.Milestones {
		var targetVal *string
		if ms.TargetDate != "" {
			t := ms.TargetDate
			targetVal = &t
		}
		delKeys := ms.DeliverableKeys
		if delKeys == nil {
			delKeys = []string{}
		}
		ruleKeys := ms.AcceptanceRuleKeys
		if ruleKeys == nil {
			ruleKeys = []string{}
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO core.project_milestones (project_id, milestone_key, phase_key, name, target_date, status, deliverable_keys, acceptance_rule_keys, config_generation)
			 VALUES ($1, $2, $3, $4, $5::date, 'pending', $6, $7, $8)
			 ON CONFLICT (project_id, milestone_key) DO UPDATE
			 SET phase_key = EXCLUDED.phase_key,
			     name = EXCLUDED.name,
			     target_date = EXCLUDED.target_date,
			     deliverable_keys = EXCLUDED.deliverable_keys,
			     acceptance_rule_keys = EXCLUDED.acceptance_rule_keys,
			     config_generation = EXCLUDED.config_generation`,
			projectID, ms.Key, ms.PhaseKey, ms.Name, targetVal, delKeys, ruleKeys, newGeneration); err != nil {
			return PlanApplied{}, fmt.Errorf("проєкція віхи %s: %w", ms.Key, err)
		}
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM core.project_milestones WHERE project_id = $1 AND NOT (milestone_key = ANY($2))`,
		projectID, milestoneKeys(manifest.Milestones)); err != nil {
		return PlanApplied{}, fmt.Errorf("вилучення застарілих віх: %w", err)
	}

	if tag, err := tx.Exec(ctx,
		`UPDATE core.work_products SET row_version = row_version + 1 WHERE id = $1 AND row_version = $2`,
		wp.ID, expectedRowVersion); err != nil || tag.RowsAffected() == 0 {
		return PlanApplied{}, ErrVersionConflict
	}

	if err := s.recordProjectAuditTx(ctx, tx, actorID, projectID, "plan.apply", map[string]any{
		"revision_id":       targetRevID.String(),
		"revision_number":   targetRevNum,
		"config_generation": newGeneration,
	}); err != nil {
		return PlanApplied{}, err
	}

	// CORE-CONTRACT-003 §5: комітована подія plan.applied — без неї жоден споживач
	// виобоку (наприклад, реєстр метрик) не дізнається про нову чинну конфігурацію.
	if err := automation.EmitEvent(ctx, tx, "plan.applied", &projectID, &actorID, uuid.New(), map[string]any{
		"revision_id":       targetRevID.String(),
		"revision_number":   targetRevNum,
		"config_generation": newGeneration,
	}); err != nil {
		return PlanApplied{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PlanApplied{}, fmt.Errorf("фіксація застосування плану: %w", err)
	}

	return PlanApplied{
		EffectivePlanRevisionID: targetRevID,
		ConfigGeneration:        newGeneration,
		RowVersion:              wp.RowVersion + 1,
	}, nil
}

func canonicalPlanHashes(code, wpType, profile, title, body string, metadata map[string]any, manifest GenericPlanManifest) ([]byte, []byte) {
	contentSum := sha256.Sum256([]byte(body))
	metadataJSON, _ := json.Marshal(metadata)
	manifestJSON, _ := json.Marshal(manifest)
	var canonical bytes.Buffer
	for _, field := range [][]byte{[]byte(code), []byte(wpType), []byte(profile), []byte(title), []byte(body), metadataJSON, manifestJSON} {
		canonical.Write(field)
		canonical.WriteByte('\x00')
	}
	payloadSum := sha256.Sum256(canonical.Bytes())
	return payloadSum[:], contentSum[:]
}

func canonicalManifestHash(manifest GenericPlanManifest) []byte {
	data, _ := json.Marshal(manifest)
	sum := sha256.Sum256(data)
	return sum[:]
}

func validateKeys(section string, keys []string) error {
	seen := map[string]bool{}
	for _, key := range keys {
		if !planKeyPattern.MatchString(key) || seen[key] {
			return fmt.Errorf("%w: %s має порожній або дубльований key", ErrInvalidPlanManifest, section)
		}
		seen[key] = true
	}
	return nil
}
func keySet(keys []string) map[string]bool {
	result := map[string]bool{}
	for _, key := range keys {
		result[key] = true
	}
	return result
}
func objectiveKeys(items []ProjectObjective) []string {
	keys := make([]string, len(items))
	for i := range items {
		keys[i] = items[i].Key
	}
	return keys
}
func scopeKeys(items []ScopeItem) []string {
	keys := make([]string, len(items))
	for i := range items {
		keys[i] = items[i].Key
	}
	return keys
}
func assumptionKeys(items []Assumption) []string {
	keys := make([]string, len(items))
	for i := range items {
		keys[i] = items[i].Key
	}
	return keys
}
func constraintKeys(items []Constraint) []string {
	keys := make([]string, len(items))
	for i := range items {
		keys[i] = items[i].Key
	}
	return keys
}
func responsibilityKeys(items []ResponsibilityAssignment) []string {
	keys := make([]string, len(items))
	for i := range items {
		keys[i] = items[i].Key
	}
	return keys
}
func deliverableKeys(items []Deliverable) []string {
	keys := make([]string, len(items))
	for i := range items {
		keys[i] = items[i].Key
	}
	return keys
}
func phaseKeys(items []PlanPhase) []string {
	keys := make([]string, len(items))
	for i := range items {
		keys[i] = items[i].Key
	}
	return keys
}
func milestoneKeys(items []PlanMilestone) []string {
	keys := make([]string, len(items))
	for i := range items {
		keys[i] = items[i].Key
	}
	return keys
}

// isForeignKeyViolation розпізнає і 23503 (звичайне порушення FK), і 23001
// (restrict_violation — саме цей код PostgreSQL повертає на ON DELETE
// RESTRICT, а не 23503).
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23503" || pgErr.Code == "23001"
}
func acceptanceRuleKeys(items []AcceptanceRule) []string {
	keys := make([]string, len(items))
	for i := range items {
		keys[i] = items[i].Key
	}
	return keys
}
func validStatus(value string) bool {
	return value == "draft" || value == "in_review" || value == "approved" || value == "obsolete"
}
func dateRange(start, finish string) (time.Time, time.Time, error) {
	s, err := time.Parse(time.DateOnly, start)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	f, err := time.Parse(time.DateOnly, finish)
	return s, f, err
}
func hasPhaseCycle(phases []PlanPhase) bool {
	graph := map[string][]string{}
	for _, phase := range phases {
		graph[phase.Key] = phase.DependsOn
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(key string) bool {
		if visiting[key] {
			return true
		}
		if visited[key] {
			return false
		}
		visiting[key] = true
		for _, dependency := range graph[key] {
			if visit(dependency) {
				return true
			}
		}
		visiting[key] = false
		visited[key] = true
		return false
	}
	for key := range graph {
		if visit(key) {
			return true
		}
	}
	return false
}
