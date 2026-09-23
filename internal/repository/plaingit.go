package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// PlainGitProvider виконує операції над одним "чистим" Git-сховищем: локальним
// bare репозиторієм (шлях без схеми URL) або віддаленим по HTTPS.
type PlainGitProvider struct{}

func NewPlainGitProvider() *PlainGitProvider {
	return &PlainGitProvider{}
}

func (p *PlainGitProvider) Type() ProviderType {
	return ProviderPlainGit
}

// IsLocalPath визначає, чи remoteURL є локальним файловим шляхом (без схеми URL),
// а не HTTPS/SSH-адресою.
func IsLocalPath(remoteURL string) bool {
	return !strings.Contains(remoteURL, "://") && !strings.Contains(remoteURL, "@")
}

// EnsureLocalBareRepository ініціалізує порожній bare-репозиторій за локальним
// шляхом, якщо він ще не існує; виклик над наявним репозиторієм — безпечний no-op.
func EnsureLocalBareRepository(remoteURL string) error {
	if !IsLocalPath(remoteURL) {
		return nil
	}
	if _, err := os.Stat(remoteURL); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(remoteURL), 0o750); err != nil {
		return fmt.Errorf("створення батьківського каталогу репозиторію: %w", err)
	}
	if _, err := git.PlainInit(remoteURL, true); err != nil {
		return fmt.Errorf("ініціалізація bare-репозиторію %s: %w", remoteURL, err)
	}

	return nil
}

// Probe перевіряє доступність сховища й повертає SHA останнього коміту гілки
// без клонування (аналог `git ls-remote`), не змінюючи стан репозиторію.
func (p *PlainGitProvider) Probe(ctx context.Context, remoteURL, branch string) (ProbeResult, error) {
	remote := git.NewRemote(nil, &config.RemoteConfig{Name: "origin", URLs: []string{remoteURL}})

	refs, err := remote.ListContext(ctx, &git.ListOptions{})
	if errors.Is(err, transport.ErrEmptyRemoteRepository) {
		return ProbeResult{}, nil
	}
	if err != nil {
		return ProbeResult{}, wrapUnreachable(remoteURL, err)
	}

	refName := plumbing.NewBranchReferenceName(branch)
	for _, ref := range refs {
		if ref.Name() == refName {
			return ProbeResult{HeadCommitSHA: ref.Hash().String()}, nil
		}
	}

	// Порожній щойно ініціалізований репозиторій без жодного коміту — не помилка Probe.
	if len(refs) == 0 {
		return ProbeResult{}, nil
	}

	return ProbeResult{}, fmt.Errorf("%w: %s", ErrBranchNotFound, branch)
}

// ExportFile записує/оновлює один файл і атомарно комітить зміну: клонує сховище
// в тимчасовий робочий каталог, застосовує зміну, комітить і надсилає push.
func (p *PlainGitProvider) ExportFile(ctx context.Context, remoteURL, branch string, file FileExport) (CommitResult, error) {
	workDir, err := os.MkdirTemp("", "delmos-export-*")
	if err != nil {
		return CommitResult{}, fmt.Errorf("створення тимчасового робочого каталогу: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	repo, err := cloneOrInit(ctx, remoteURL, branch, workDir)
	if err != nil {
		return CommitResult{}, err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return CommitResult{}, fmt.Errorf("отримання робочого дерева: %w", err)
	}

	fullPath := filepath.Join(workDir, filepath.FromSlash(file.Path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return CommitResult{}, fmt.Errorf("створення каталогу для %s: %w", file.Path, err)
	}
	if err := os.WriteFile(fullPath, file.Content, 0o600); err != nil {
		return CommitResult{}, fmt.Errorf("запис файлу %s: %w", file.Path, err)
	}

	if _, err := worktree.Add(filepath.ToSlash(file.Path)); err != nil {
		return CommitResult{}, fmt.Errorf("додавання %s до індексу: %w", file.Path, err)
	}

	commitHash, err := worktree.Commit(file.Message, &git.CommitOptions{
		Author: &object.Signature{Name: file.AuthorName, Email: file.AuthorEmail, When: time.Now()},
	})
	if err != nil {
		return CommitResult{}, fmt.Errorf("створення коміту: %w", err)
	}

	err = repo.PushContext(ctx, &git.PushOptions{RemoteName: "origin"})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return CommitResult{}, fmt.Errorf("надсилання (push) у %s: %w", remoteURL, err)
	}

	return CommitResult{CommitSHA: commitHash.String()}, nil
}

// cloneOrInit клонує наявний репозиторій у workDir; якщо гілка ще не має жодного
// коміту (порожній bare-репозиторій), ініціалізує локальний клон і прив'язує origin.
func cloneOrInit(ctx context.Context, remoteURL, branch, workDir string) (*git.Repository, error) {
	repo, err := git.PlainCloneContext(ctx, workDir, false, &git.CloneOptions{
		URL:           remoteURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
	})
	if err == nil {
		return repo, nil
	}
	if !errors.Is(err, transport.ErrEmptyRemoteRepository) {
		return nil, wrapUnreachable(remoteURL, err)
	}

	repo, err = git.PlainInit(workDir, false)
	if err != nil {
		return nil, fmt.Errorf("ініціалізація локального робочого репозиторію: %w", err)
	}
	if _, err := repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{remoteURL}}); err != nil {
		return nil, fmt.Errorf("прив'язка origin: %w", err)
	}
	head := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(branch))
	if err := repo.Storer.SetReference(head); err != nil {
		return nil, fmt.Errorf("встановлення початкової гілки %s: %w", branch, err)
	}

	return repo, nil
}
