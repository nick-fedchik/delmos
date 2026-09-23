package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrBindingNotFound = errors.New("прив'язку сховища не знайдено")

type Binding struct {
	ProjectID     uuid.UUID
	ProviderType  ProviderType
	RemoteURL     string
	DefaultBranch string
	Status        string
	LastProbeAt   *time.Time
	LastError     *string
}

type Store struct {
	pool     *pgxpool.Pool
	provider Provider
}

func NewStore(pool *pgxpool.Pool, provider Provider) *Store {
	return &Store{pool: pool, provider: provider}
}

// Bind прив'язує проєкт до простого Git-сховища: якщо remoteURL — локальний
// шлях без наявного репозиторію, ініціалізує порожній bare-репозиторій, потім
// виконує Probe і зберігає результат разом із прив'язкою.
func (s *Store) Bind(ctx context.Context, actorID, projectID uuid.UUID, remoteURL, defaultBranch string) (Binding, error) {
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	if err := EnsureLocalBareRepository(remoteURL); err != nil {
		return Binding{}, err
	}

	probeResult, probeErr := s.provider.Probe(ctx, remoteURL, defaultBranch)

	status := "active"
	var lastError *string
	if probeErr != nil {
		status = "error"
		message := probeErr.Error()
		lastError = &message
	}

	now := time.Now()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO core.repository_bindings
		    (project_id, provider_type, remote_url, default_branch, status, last_probe_at, last_error, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (project_id) DO UPDATE SET
		    remote_url = excluded.remote_url, default_branch = excluded.default_branch,
		    status = excluded.status, last_probe_at = excluded.last_probe_at, last_error = excluded.last_error`,
		projectID, ProviderPlainGit, remoteURL, defaultBranch, status, now, lastError, actorID)
	if err != nil {
		return Binding{}, fmt.Errorf("збереження прив'язки сховища: %w", err)
	}

	correlationID := uuid.New()
	auditOutcome := "success"
	if probeErr != nil {
		auditOutcome = "denied"
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'repository.bind', 'project', $2, $3, $4, $5)`,
		actorID, projectID, auditOutcome, map[string]any{"remote_url": remoteURL}, correlationID,
	); err != nil {
		return Binding{}, fmt.Errorf("запис аудиторської події прив'язки сховища: %w", err)
	}

	_ = probeResult // HeadCommitSHA поки не персистується окремо — Probe лише підтверджує доступність

	return Binding{
		ProjectID: projectID, ProviderType: ProviderPlainGit, RemoteURL: remoteURL,
		DefaultBranch: defaultBranch, Status: status, LastProbeAt: &now, LastError: lastError,
	}, nil
}

func (s *Store) Get(ctx context.Context, projectID uuid.UUID) (Binding, error) {
	var b Binding
	err := s.pool.QueryRow(ctx,
		`SELECT project_id, provider_type, remote_url, default_branch, status, last_probe_at, last_error
		 FROM core.repository_bindings WHERE project_id = $1`,
		projectID,
	).Scan(&b.ProjectID, &b.ProviderType, &b.RemoteURL, &b.DefaultBranch, &b.Status, &b.LastProbeAt, &b.LastError)
	if errors.Is(err, pgx.ErrNoRows) {
		return Binding{}, ErrBindingNotFound
	}
	if err != nil {
		return Binding{}, fmt.Errorf("читання прив'язки сховища: %w", err)
	}

	return b, nil
}

// ExportWorkProduct записує вміст переданої ревізії у файл `{code}.md` прив'язаного
// сховища і атомарно комітить зміну (ручний експорт Docs-as-Code, без автоматики outbox).
func (s *Store) ExportWorkProduct(ctx context.Context, actorID, projectID uuid.UUID, wpCode, body, authorName, authorEmail string) (CommitResult, error) {
	binding, err := s.Get(ctx, projectID)
	if err != nil {
		return CommitResult{}, err
	}

	result, err := s.provider.ExportFile(ctx, binding.RemoteURL, binding.DefaultBranch, FileExport{
		Path:        wpCode + ".md",
		Content:     []byte(body),
		Message:     "Export " + wpCode + " from DELMOS",
		AuthorName:  authorName,
		AuthorEmail: authorEmail,
	})

	correlationID := uuid.New()
	outcome := "success"
	detail := map[string]any{"work_product_code": wpCode}
	if err != nil {
		outcome = "denied"
		detail["error"] = err.Error()
	} else {
		detail["commit_sha"] = result.CommitSHA
	}
	_, auditErr := s.pool.Exec(ctx,
		`INSERT INTO core.audit_events (actor_user_id, action, scope_type, scope_id, outcome, detail, correlation_id)
		 VALUES ($1, 'repository.export', 'project', $2, $3, $4, $5)`,
		actorID, projectID, outcome, detail, correlationID)
	if auditErr != nil {
		return CommitResult{}, fmt.Errorf("запис аудиторської події експорту: %w", auditErr)
	}

	if err != nil {
		return CommitResult{}, err
	}

	return result, nil
}
