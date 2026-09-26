package project

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ApplyRevisionCommittedEffects — обробник trigger.core.after_revision_committed
// (SPEC-04, реєструється зовні через automation.Engine.RegisterHandler).
// Виконує в одній транзакції з підтвердженням доставки: розрахунок вектора
// (VEC-01) та каскадне поширення is_suspect (GRP-03), які раніше рахувалися
// синхронно в тій самій транзакції ревізії — тепер асинхронно через outbox
// (SWR-36.2). Дані ревізії читаються за revisionID, а не з корисного
// навантаження події: outbox зберігає лише посилання, не текст артефакту.
func (s *Store) ApplyRevisionCommittedEffects(ctx context.Context, tx pgx.Tx, workProductID, revisionID uuid.UUID) error {
	var projectID uuid.UUID
	var wpType, title, body string
	err := tx.QueryRow(ctx,
		`SELECT wp.project_id, wp.type, wp.title, wpr.body
		 FROM core.work_product_revisions wpr
		 JOIN core.work_products wp ON wp.id = wpr.work_product_id
		 WHERE wpr.id = $1 AND wp.id = $2`,
		revisionID, workProductID,
	).Scan(&projectID, &wpType, &title, &body)
	if err != nil {
		return fmt.Errorf("читання ревізії %s для асинхронних ефектів: %w", revisionID, err)
	}

	if err := upsertEmbedding(ctx, tx, wpType, workProductID, revisionID, title, body); err != nil {
		return err
	}
	if _, err := propagateSuspect(ctx, tx, projectID, workProductID); err != nil {
		return err
	}
	return nil
}
