# SPEC-02: Технічна специфікація інтерфейсу провайдерів сховищ (RepositoryProvider)

Дата: 2026-09-23. Статус: нормативна технічна специфікація інтерфейсу сховищ.  
Ліцензія: Apache 2.0.  
Контекст: [проєкт і сховища](../architecture/PROJECT_MODEL.md), [ADR-005](../architecture/decisions/ADR-005-multi-provider-repository-abstraction.md),
[SWR-01..02](../requirements/software/SWR-01-core-project-plan.md).

---

## 1. Контракт інтерфейсу на мові Go

Платформа DELMOS взаємодіє зі сховищами артефактів через типізований інтерфейс `RepositoryProvider`:

```go
package repository

import (
	"context"
	"time"
)

type ProviderType string

const (
	ProviderInternal  ProviderType = "internal"
	ProviderPlainGit  ProviderType = "plain_git"
	ProviderGitHub    ProviderType = "github"
	ProviderGitLab    ProviderType = "gitlab"
	ProviderAzure     ProviderType = "azure_repos"
	ProviderBitbucket ProviderType = "bitbucket"
	ProviderGitea     ProviderType = "gitea"
)

type FileContent struct {
	Path        string
	BlobID      string
	Content     []byte
	CommitSHA   string
	Author      string
	CommittedAt time.Time
}

type CommitRequest struct {
	Branch      string
	BaseCommit  string // Очікуваний батьківський коміт для контролю конфліктів
	Message     string
	AuthorName  string
	AuthorEmail string
	Actions     []FileAction
}

type FileAction struct {
	Action   string // "create", "update", "delete"
	FilePath string
	Content  []byte
}

type RepositoryProvider interface {
	// Type повертає унікальний тип провайдера
	Type() ProviderType

	// Probe виконує неруйнівну діагностику підключення та перевіряє права
	Probe(ctx context.Context, config BindingConfig) (*ProbeResult, error)

	// GetFile читає окремий файл за ревізією чи гілкою без клонування всього репозиторію
	GetFile(ctx context.Context, config BindingConfig, path string, ref string) (*FileContent, error)

	// ListFiles повертає перелік файлів у каталозі за ревізією
	ListFiles(ctx context.Context, config BindingConfig, dirPath string, ref string) ([]string, error)

	// CreateCommit здійснює атомарний коміт одного або кількох файлів
	CreateCommit(ctx context.Context, config BindingConfig, req CommitRequest) (*CommitResult, error)

	// GetLatestCommit повертає SHA останнього коміту в гілці для перевірки дрейфу
	GetLatestCommit(ctx context.Context, config BindingConfig, branch string) (string, error)
}
```

---

## 2. Реалізації провайдерів

### 2.1. Internal Storage Provider (`internal`)
* **Механізм:** Зберігання файлів Markdown та frontmatter безпосередньо в реляційній таблиці `internal_repository_blobs` у PostgreSQL (або локальному bare-репозиторії на сервері через бібліотеку `go-git`).
* **Особливості:** Не вимагає жодних мережевих викликів, токенів чи зовнішніх облікових записів. Ідеально для автономних ізольованих інсталяцій та R&D.

### 2.2. Plain Remote Git Provider (`plain_git`)
* **Механізм:** Робота з віддаленими стандартними Git-серверами по протоколу SSH або HTTPS з використанням автентифікації за ключем/паролем без прив'язки до вендорських API.

### 2.3. Enterprise REST Adapters (GitHub, GitLab, Azure Repos)
* **Механізм:** Взаємодія через офіційні REST API платформ:
  * GitHub: `GET /repos/{owner}/{repo}/contents/{path}`, `POST /repos/{owner}/{repo}/git/commits`.
  * GitLab: `GET /projects/{id}/repository/files/{path}/raw`, `POST /projects/{id}/repository/commits`.
  * Azure Repos: `GET /_apis/git/repositories/{id}/items`, `POST /_apis/git/repositories/{id}/pushes`.
* **Особливості:** Повний обхід без локального клонування на диск сервера; прямий контроль прав за токеном користувача.

---

## 3. Таблиця зберігання прив'язок у PostgreSQL

```sql
CREATE TABLE repository_bindings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider_type VARCHAR(32) NOT NULL,
    endpoint_url TEXT,
    repository_identifier VARCHAR(255) NOT NULL, -- "owner/repo" або project_id
    default_branch VARCHAR(64) NOT NULL DEFAULT 'main',
    root_path TEXT NOT NULL DEFAULT '',
    secret_reference_id UUID REFERENCES integration_secrets(id),
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    last_probe_at TIMESTAMPTZ,
    last_probe_status VARCHAR(32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT uq_project_repo UNIQUE (project_id)
);
```
