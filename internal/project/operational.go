package project

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Stakeholder struct {
	ID         uuid.UUID `json:"id"`
	ProjectID  uuid.UUID `json:"project_id"`
	Kind       string    `json:"kind"`
	Name       string    `json:"name"`
	ContactRef string    `json:"contact_ref"`
	Interest   string    `json:"interest"`
	CreatedAt  time.Time `json:"created_at"`
}

type ProjectRisk struct {
	ID               uuid.UUID `json:"id"`
	ProjectID        uuid.UUID `json:"project_id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Status           string    `json:"status"`
	Impact           string    `json:"impact"`
	Likelihood       string    `json:"likelihood"`
	ResponseStrategy string    `json:"response_strategy"`
	OwnerRef         string    `json:"owner_ref"`
	CreatedAt        time.Time `json:"created_at"`
}

func (s *Store) ListStakeholders(ctx context.Context, projectID uuid.UUID) ([]Stakeholder, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, kind, name, contact_ref, interest, created_at
		 FROM core.project_stakeholders WHERE project_id = $1 ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("читання стейкхолдерів: %w", err)
	}
	defer rows.Close()

	var result []Stakeholder
	for rows.Next() {
		var item Stakeholder
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Kind, &item.Name, &item.ContactRef, &item.Interest, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("розбір стейкхолдера: %w", err)
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Store) CreateStakeholder(ctx context.Context, actorID, projectID uuid.UUID, kind, name, contactRef, interest string) (Stakeholder, error) {
	var item Stakeholder
	err := s.pool.QueryRow(ctx,
		`INSERT INTO core.project_stakeholders (project_id, kind, name, contact_ref, interest, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, project_id, kind, name, contact_ref, interest, created_at`,
		projectID, kind, name, contactRef, interest, actorID,
	).Scan(&item.ID, &item.ProjectID, &item.Kind, &item.Name, &item.ContactRef, &item.Interest, &item.CreatedAt)
	if err != nil {
		return Stakeholder{}, fmt.Errorf("створення стейкхолдера: %w", err)
	}
	correlationID := uuid.New()
	_ = s.recordProjectAudit(ctx, actorID, projectID, "stakeholder.create", correlationID, map[string]any{"stakeholder_id": item.ID.String()})
	return item, nil
}

func (s *Store) ListRisks(ctx context.Context, projectID uuid.UUID) ([]ProjectRisk, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, title, description, status, impact, likelihood, response_strategy, owner_ref, created_at
		 FROM core.project_risks WHERE project_id = $1 ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("читання ризиків: %w", err)
	}
	defer rows.Close()

	var result []ProjectRisk
	for rows.Next() {
		var item ProjectRisk
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Title, &item.Description, &item.Status, &item.Impact, &item.Likelihood, &item.ResponseStrategy, &item.OwnerRef, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("розбір ризику: %w", err)
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Store) CreateRisk(ctx context.Context, actorID, projectID uuid.UUID, title, description, impact, likelihood, responseStrategy, ownerRef string) (ProjectRisk, error) {
	var item ProjectRisk
	err := s.pool.QueryRow(ctx,
		`INSERT INTO core.project_risks (project_id, title, description, impact, likelihood, response_strategy, owner_ref, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, project_id, title, description, status, impact, likelihood, response_strategy, owner_ref, created_at`,
		projectID, title, description, impact, likelihood, responseStrategy, ownerRef, actorID,
	).Scan(&item.ID, &item.ProjectID, &item.Title, &item.Description, &item.Status, &item.Impact, &item.Likelihood, &item.ResponseStrategy, &item.OwnerRef, &item.CreatedAt)
	if err != nil {
		return ProjectRisk{}, fmt.Errorf("створення ризику: %w", err)
	}
	correlationID := uuid.New()
	_ = s.recordProjectAudit(ctx, actorID, projectID, "risk.create", correlationID, map[string]any{"risk_id": item.ID.String()})
	return item, nil
}
