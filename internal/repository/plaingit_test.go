package repository_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"delmos/internal/repository"
)

func newBareRepoPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "repo.git")
}

func TestProbeOnEmptyRepositoryReturnsNoHead(t *testing.T) {
	remote := newBareRepoPath(t)
	if err := repository.EnsureLocalBareRepository(remote); err != nil {
		t.Fatalf("ініціалізація bare-репозиторію: %v", err)
	}

	provider := repository.NewPlainGitProvider()
	result, err := provider.Probe(context.Background(), remote, "main")
	if err != nil {
		t.Fatalf("Probe порожнього репозиторію не повинен повертати помилку: %v", err)
	}
	if result.HeadCommitSHA != "" {
		t.Errorf("порожній репозиторій не повинен мати HEAD, отримано %q", result.HeadCommitSHA)
	}
}

func TestProbeUnreachableRemoteReturnsError(t *testing.T) {
	provider := repository.NewPlainGitProvider()
	if _, err := provider.Probe(context.Background(), filepath.Join(t.TempDir(), "not-a-repo"), "main"); err == nil {
		t.Error("Probe неіснуючого сховища має повертати помилку")
	}
}

func TestExportFileCreatesCommitAndProbeSeesIt(t *testing.T) {
	ctx := context.Background()
	remote := newBareRepoPath(t)
	if err := repository.EnsureLocalBareRepository(remote); err != nil {
		t.Fatalf("ініціалізація bare-репозиторію: %v", err)
	}

	provider := repository.NewPlainGitProvider()
	result, err := provider.ExportFile(ctx, remote, "main", repository.FileExport{
		Path: "PLAN-001.md", Content: []byte("# Plan\n"), Message: "Export PLAN-001",
		AuthorName: "DELMOS", AuthorEmail: "delmos@example.invalid",
	})
	if err != nil {
		t.Fatalf("експорт файлу: %v", err)
	}
	if result.CommitSHA == "" {
		t.Fatal("очікувався непорожній commit_sha")
	}

	probe, err := provider.Probe(ctx, remote, "main")
	if err != nil {
		t.Fatalf("Probe після експорту: %v", err)
	}
	if probe.HeadCommitSHA != result.CommitSHA {
		t.Errorf("HEAD після Probe (%s) має збігатися з коміт-хешем експорту (%s)", probe.HeadCommitSHA, result.CommitSHA)
	}
}

func TestExportFileTwiceCreatesSecondCommit(t *testing.T) {
	ctx := context.Background()
	remote := newBareRepoPath(t)
	if err := repository.EnsureLocalBareRepository(remote); err != nil {
		t.Fatalf("ініціалізація bare-репозиторію: %v", err)
	}
	provider := repository.NewPlainGitProvider()

	first, err := provider.ExportFile(ctx, remote, "main", repository.FileExport{
		Path: "REQ-001.md", Content: []byte("v1"), Message: "v1", AuthorName: "A", AuthorEmail: "a@example.invalid",
	})
	if err != nil {
		t.Fatalf("перший експорт: %v", err)
	}

	second, err := provider.ExportFile(ctx, remote, "main", repository.FileExport{
		Path: "REQ-001.md", Content: []byte("v2"), Message: "v2", AuthorName: "A", AuthorEmail: "a@example.invalid",
	})
	if err != nil {
		t.Fatalf("другий експорт: %v", err)
	}

	if first.CommitSHA == second.CommitSHA {
		t.Error("другий коміт має мати інший SHA")
	}
}

func TestEnsureLocalBareRepositoryIsIdempotent(t *testing.T) {
	remote := newBareRepoPath(t)
	if err := repository.EnsureLocalBareRepository(remote); err != nil {
		t.Fatalf("перша ініціалізація: %v", err)
	}
	if err := repository.EnsureLocalBareRepository(remote); err != nil {
		t.Fatalf("повторний виклик має бути no-op: %v", err)
	}
	if _, err := os.Stat(remote); err != nil {
		t.Fatalf("репозиторій має існувати на диску: %v", err)
	}
}
