// Package project реалізує CRUD проєкту та атомарне створення обов'язкового
// Generic Project Plan (PLAN-001) — PROJECT_MODEL.md, WORK_PRODUCTS.md.
package project

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	planWorkProductCode = "PLAN-001"
	planType            = "plan"
	planProfile         = "core:project_plan"
	planTitle           = "Generic Project Plan"
	planClassification  = "internal"
)

var (
	ErrCodeTaken   = errors.New("код проєкту вже використовується")
	ErrNotFound    = errors.New("проєкт не знайдено")
	ErrInvalidCode = errors.New("код проєкту обов'язковий")
)

type Project struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Description string
	Status      string
	CreatedBy   uuid.UUID
	CreatedAt   time.Time
	RowVersion  int64
}

type PlanBinding struct {
	ProjectID               uuid.UUID
	PlanWorkProductID       uuid.UUID
	EffectivePlanRevisionID *uuid.UUID
	ConfigGeneration        int64
}

type WorkProductRevision struct {
	ID             uuid.UUID
	WorkProductID  uuid.UUID
	RevisionNumber int
	Body           string
	Metadata       map[string]any
	PayloadHash    []byte
	ContentHash    []byte
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
}

type WorkProduct struct {
	ID             uuid.UUID
	ProjectID      uuid.UUID
	Code           string
	Type           string
	Profile        string
	Title          string
	Status         string
	Classification string
	RowVersion     int64
}

// ProjectDetail — проєкт разом з обов'язковим планом, повернений одразу після створення/читання.
type ProjectDetail struct {
	Project     Project
	Plan        WorkProduct
	PlanLatest  WorkProductRevision
	PlanBinding PlanBinding
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateWithPlan атомарно створює Project, обов'язковий WP PLAN-001 з першою
// ревізією, ProjectPlanBinding та Project-scope RoleBinding creator'а (SWR-43,
// PROJECT_MODEL.md §1: "жоден проєкт не існує без плану").
func (s *Store) CreateWithPlan(ctx context.Context, actorID uuid.UUID, code, name, description string) (ProjectDetail, error) {
	if code == "" {
		return ProjectDetail{}, ErrInvalidCode
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("початок транзакції створення проєкту: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var projectID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO core.projects (code, name, description, created_by) VALUES ($1, $2, $3, $4) RETURNING id`,
		code, name, description, actorID,
	).Scan(&projectID)
	if err != nil {
		if isUniqueViolation(err) {
			return ProjectDetail{}, ErrCodeTaken
		}
		return ProjectDetail{}, fmt.Errorf("створення проєкту: %w", err)
	}

	var planWorkProductID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO core.work_products (project_id, code, type, profile, title, classification)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		projectID, planWorkProductCode, planType, planProfile, planTitle, planClassification,
	).Scan(&planWorkProductID)
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("створення PLAN-001: %w", err)
	}

	metadata := map[string]any{
		"plan_schema_version": "1.0.0",
		"name":                name,
	}
	body := "# " + planTitle + "\n\n" + name + "\n"
	payloadHash, contentHash := canonicalHashes(planWorkProductCode, planType, planProfile, planTitle, body, metadata)

	var revisionID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO core.work_product_revisions
		    (work_product_id, revision_number, body, metadata, payload_hash, content_hash, created_by)
		 VALUES ($1, 1, $2, $3, $4, $5, $6) RETURNING id`,
		planWorkProductID, body, metadata, payloadHash, contentHash, actorID,
	).Scan(&revisionID)
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("створення першої ревізії PLAN-001: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO core.project_plan_bindings (project_id, plan_work_product_id) VALUES ($1, $2)`,
		projectID, planWorkProductID)
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("прив'язка плану до проєкту: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO core.role_bindings (user_id, role_key, scope_type, scope_id, granted_by, granted_reason)
		 VALUES ($1, 'project.owner', 'project', $2, $1, 'automatic: creator of the project')`,
		actorID, projectID)
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("автоматичне надання project.owner: %w", err)
	}

	correlationID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'project.create', 'project', $2, 'success', $3, $4)`,
		actorID, projectID, map[string]any{"code": code}, correlationID)
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("запис аудиторської події створення проєкту: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ProjectDetail{}, fmt.Errorf("фіксація створення проєкту: %w", err)
	}

	return ProjectDetail{
		Project: Project{
			ID: projectID, Code: code, Name: name, Description: description,
			Status: "active", CreatedBy: actorID, RowVersion: 1,
		},
		Plan: WorkProduct{
			ID: planWorkProductID, ProjectID: projectID, Code: planWorkProductCode, Type: planType,
			Profile: planProfile, Title: planTitle, Status: "draft", Classification: planClassification, RowVersion: 1,
		},
		PlanLatest: WorkProductRevision{
			ID: revisionID, WorkProductID: planWorkProductID, RevisionNumber: 1,
			Body: body, Metadata: metadata, PayloadHash: payloadHash, ContentHash: contentHash, CreatedBy: actorID,
		},
		PlanBinding: PlanBinding{ProjectID: projectID, PlanWorkProductID: planWorkProductID, ConfigGeneration: 0},
	}, nil
}

// Get повертає проєкт разом з обов'язковим планом і його останньою ревізією.
func (s *Store) Get(ctx context.Context, projectID uuid.UUID) (ProjectDetail, error) {
	var detail ProjectDetail

	err := s.pool.QueryRow(ctx,
		`SELECT id, code, name, description, status, created_by, created_at, row_version
		 FROM core.projects WHERE id = $1`,
		projectID,
	).Scan(&detail.Project.ID, &detail.Project.Code, &detail.Project.Name, &detail.Project.Description,
		&detail.Project.Status, &detail.Project.CreatedBy, &detail.Project.CreatedAt, &detail.Project.RowVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProjectDetail{}, ErrNotFound
	}
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("читання проєкту: %w", err)
	}

	err = s.pool.QueryRow(ctx,
		`SELECT wp.id, wp.project_id, wp.code, wp.type, wp.profile, wp.title, wp.status, wp.classification, wp.row_version,
		        r.id, r.revision_number, r.body, r.metadata, r.payload_hash, r.content_hash, r.created_by, r.created_at
		 FROM core.project_plan_bindings b
		 JOIN core.work_products wp ON wp.id = b.plan_work_product_id
		 JOIN core.work_product_revisions r ON r.work_product_id = wp.id
		 WHERE b.project_id = $1
		 ORDER BY r.revision_number DESC LIMIT 1`,
		projectID,
	).Scan(&detail.Plan.ID, &detail.Plan.ProjectID, &detail.Plan.Code, &detail.Plan.Type, &detail.Plan.Profile,
		&detail.Plan.Title, &detail.Plan.Status, &detail.Plan.Classification, &detail.Plan.RowVersion,
		&detail.PlanLatest.ID, &detail.PlanLatest.RevisionNumber, &detail.PlanLatest.Body, &detail.PlanLatest.Metadata,
		&detail.PlanLatest.PayloadHash, &detail.PlanLatest.ContentHash, &detail.PlanLatest.CreatedBy, &detail.PlanLatest.CreatedAt)
	if err != nil {
		return ProjectDetail{}, fmt.Errorf("читання PLAN-001: %w", err)
	}
	detail.PlanLatest.WorkProductID = detail.Plan.ID

	return detail, nil
}

// Summary — рядок списку проєктів, доступних актору (без деталей плану).
type Summary struct {
	ID     uuid.UUID
	Code   string
	Name   string
	Status string
}

// ListForActor повертає лише проєкти, де актор має чинний Project-scope RoleBinding
// (SWR-42 §3: права одного проєкту не дають видимості в іншому).
func (s *Store) ListForActor(ctx context.Context, actorID uuid.UUID) ([]Summary, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT p.id, p.code, p.name, p.status
		 FROM core.projects p
		 JOIN core.role_bindings rb ON rb.scope_type = 'project' AND rb.scope_id = p.id
		 WHERE rb.user_id = $1 AND rb.revoked_at IS NULL
		   AND (rb.expires_at IS NULL OR rb.expires_at > now())
		 ORDER BY p.code`,
		actorID)
	if err != nil {
		return nil, fmt.Errorf("читання списку проєктів: %w", err)
	}
	defer rows.Close()

	var summaries []Summary
	for rows.Next() {
		var summary Summary
		if err := rows.Scan(&summary.ID, &summary.Code, &summary.Name, &summary.Status); err != nil {
			return nil, fmt.Errorf("розбір рядка списку проєктів: %w", err)
		}
		summaries = append(summaries, summary)
	}

	return summaries, rows.Err()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// canonicalHashes обчислює content_hash (від тіла) та payload_hash (від канонічного
// представлення ревізії) — точний алгоритм канонізації не регламентовано
// WORK_PRODUCTS.md, тому зафіксовано тут: JSON з детермінованим порядком полів
// (map-ключі metadata сортуються encoding/json автоматично).
func canonicalHashes(code, wpType, profile, title, body string, metadata map[string]any) (payloadHash, contentHash []byte) {
	contentSum := sha256.Sum256([]byte(body))

	metadataJSON, _ := json.Marshal(metadata)
	var canonical bytes.Buffer
	canonical.WriteString(code)
	canonical.WriteByte('\x00')
	canonical.WriteString(wpType)
	canonical.WriteByte('\x00')
	canonical.WriteString(profile)
	canonical.WriteByte('\x00')
	canonical.WriteString(title)
	canonical.WriteByte('\x00')
	canonical.WriteString(body)
	canonical.WriteByte('\x00')
	canonical.Write(metadataJSON)

	payloadSum := sha256.Sum256(canonical.Bytes())

	return payloadSum[:], contentSum[:]
}
