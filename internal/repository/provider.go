// Package repository реалізує мінімальний RepositoryProvider для простого Git
// (SPEC-02, ADR-005): локальний bare репозиторій або віддалений по HTTPS.
// Корпоративні адаптери (GitHub/GitLab/Azure/Bitbucket/Gitea) — поза межами v0.5.0.
package repository

import (
	"context"
	"errors"
	"fmt"
)

type ProviderType string

const ProviderPlainGit ProviderType = "plain_git"

var (
	ErrRemoteUnreachable = errors.New("сховище недоступне")
	ErrBranchNotFound    = errors.New("гілку не знайдено у сховищі")
)

// ProbeResult — результат неруйнівної діагностики підключення (SPEC-02 §1).
type ProbeResult struct {
	HeadCommitSHA string
}

// FileExport — один файл для атомарного коміту (спрощений FileAction/CommitRequest SPEC-02).
type FileExport struct {
	Path        string
	Content     []byte
	Message     string
	AuthorName  string
	AuthorEmail string
}

// CommitResult — наслідок успішного експорту.
type CommitResult struct {
	CommitSHA string
}

// Provider — контракт простого Git-сховища DELMOS (звужений SPEC-02 RepositoryProvider
// до операцій, потрібних для ручного експорту Docs-as-Code у v0.5.0).
type Provider interface {
	Type() ProviderType
	Probe(ctx context.Context, remoteURL, branch string) (ProbeResult, error)
	ExportFile(ctx context.Context, remoteURL, branch string, file FileExport) (CommitResult, error)
}

func wrapUnreachable(remoteURL string, err error) error {
	return fmt.Errorf("%w: %s (%w)", ErrRemoteUnreachable, remoteURL, err)
}
