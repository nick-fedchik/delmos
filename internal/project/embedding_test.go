package project_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"delmos/internal/auth"
	"delmos/internal/migrate"
	"delmos/internal/project"
	"delmos/internal/testsupport"
)

// TestFindSimilarWorkProductsHybridSearch перевіряє VEC-01/VEC-02: вектор
// прив'язується до точної ревізії, а гібридний пошук повертає семантично
// (лексично) близькі вимоги в межах проєкту, ігноруючи артефакт-джерело й
// артефакти інших проєктів.
func TestFindSimilarWorkProductsHybridSearch(t *testing.T) {
	ctx := context.Background()
	store, authStore := newTestStore(t)
	userID := newTestUser(t, authStore, "vec-author")
	proj, err := store.CreateWithPlan(ctx, userID, "VEC-001", "Vector project", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}
	other, err := store.CreateWithPlan(ctx, userID, "VEC-002", "Other vector project", "")
	if err != nil {
		t.Fatalf("створення іншого проєкту: %v", err)
	}

	base, _, err := store.CreateWorkProduct(ctx, userID, proj.Project.ID, "REQ-100", "requirement",
		"Гальмівна система", "Автомобіль повинен зупинятися за нормативну дистанцію на мокрій дорозі", nil)
	if err != nil {
		t.Fatalf("створення базової вимоги: %v", err)
	}
	similar, _, err := store.CreateWorkProduct(ctx, userID, proj.Project.ID, "REQ-101", "requirement",
		"Гальмівна дистанція", "Автомобіль повинен зупинятися за нормативну дистанцію на мокрій дорозі за низької швидкості", nil)
	if err != nil {
		t.Fatalf("створення схожої вимоги: %v", err)
	}
	unrelated, _, err := store.CreateWorkProduct(ctx, userID, proj.Project.ID, "REQ-102", "requirement",
		"Освітлення салону", "Внутрішнє світло вмикається автоматично при відкритті дверей водія", nil)
	if err != nil {
		t.Fatalf("створення непов'язаної вимоги: %v", err)
	}
	// Не векторизований тип не повинен потрапляти у видачу.
	if _, _, err := store.CreateWorkProduct(ctx, userID, proj.Project.ID, "PLAN-EXTRA", "report",
		"Гальмівна система", "Автомобіль повинен зупинятися за нормативну дистанцію на мокрій дорозі", nil); err != nil {
		t.Fatalf("створення звіту: %v", err)
	}
	// Артефакт іншого проєкту з ідентичним текстом не має потрапити у видачу.
	if _, _, err := store.CreateWorkProduct(ctx, userID, other.Project.ID, "REQ-200", "requirement",
		"Гальмівна дистанція", "Автомобіль повинен зупинятися за нормативну дистанцію на мокрій дорозі за низької швидкості", nil); err != nil {
		t.Fatalf("створення вимоги в іншому проєкті: %v", err)
	}

	baseDetail, err := store.GetWorkProduct(ctx, proj.Project.ID, base.ID)
	if err != nil {
		t.Fatalf("читання базової вимоги: %v", err)
	}

	matches, err := store.FindSimilarWorkProducts(ctx, proj.Project.ID, base.ID, baseDetail.Latest.ID, 10, 0.5)
	if err != nil {
		t.Fatalf("гібридний пошук: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("очікувався хоча б один схожий результат, отримано 0")
	}
	if matches[0].WorkProductID != similar.ID {
		t.Fatalf("найсхожішою мала бути REQ-101, отримано %+v", matches[0])
	}
	for _, m := range matches {
		if m.WorkProductID == base.ID {
			t.Fatalf("джерело пошуку не повинно потрапляти у власні результати: %+v", m)
		}
		if m.WorkProductID == unrelated.ID {
			t.Fatalf("непов'язана вимога не мала перевищити поріг схожості: %+v", m)
		}
	}
}

// newTestStoreWithPool — той самий контур, що й newTestStore, але додатково
// повертає пул з'єднань для прямих SQL-перевірок, недоступних через публічне
// API Store (потрібно для симуляції "застарілого" вектора в VEC-03).
func newTestStoreWithPool(t *testing.T) (*project.Store, *pgxpool.Pool) {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), testsupport.NewDatabase(t))
	if err != nil {
		t.Fatalf("підключення до тимчасової бази: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := migrate.Apply(context.Background(), pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("застосування міграцій: %v", err)
	}

	return project.NewStore(pool), pool
}

// TestFindSimilarWorkProductsIgnoresOtherModelVectors перевіряє VEC-03: зміна
// активної моделі ембедінгів не видаляє старі вектори, але пошук працює
// лише серед векторів поточної моделі — застарілий вектор не спотворює видачу.
func TestFindSimilarWorkProductsIgnoresOtherModelVectors(t *testing.T) {
	ctx := context.Background()
	store, pool := newTestStoreWithPool(t)
	authStore := auth.NewStore(pool)
	userID := newTestUser(t, authStore, "vec03-author")
	proj, err := store.CreateWithPlan(ctx, userID, "VEC-003", "Model switch project", "")
	if err != nil {
		t.Fatalf("створення проєкту: %v", err)
	}

	const text = "Двигун повинен витримувати робочу температуру до 150 градусів Цельсія"
	a, _, err := store.CreateWorkProduct(ctx, userID, proj.Project.ID, "REQ-300", "requirement", "Термостійкість двигуна", text, nil)
	if err != nil {
		t.Fatalf("створення вимоги A: %v", err)
	}
	b, _, err := store.CreateWorkProduct(ctx, userID, proj.Project.ID, "REQ-301", "requirement", "Термостійкість двигуна (дубль)", text, nil)
	if err != nil {
		t.Fatalf("створення вимоги B: %v", err)
	}

	// Симулюємо застарілий вектор: B "залишився" з попередньої моделі
	// ембедінгів після її зміни — рядок не видаляється, лише більше не
	// відповідає поточним model_name/model_version, які використовує пошук.
	if _, err := pool.Exec(ctx,
		`UPDATE core.wp_embeddings SET model_name = 'legacy-model', model_version = '0' WHERE work_product_id = $1`, b.ID); err != nil {
		t.Fatalf("симуляція застарілого вектора: %v", err)
	}

	detailA, err := store.GetWorkProduct(ctx, proj.Project.ID, a.ID)
	if err != nil {
		t.Fatalf("читання вимоги A: %v", err)
	}

	matches, err := store.FindSimilarWorkProducts(ctx, proj.Project.ID, a.ID, detailA.Latest.ID, 10, 0.5)
	if err != nil {
		t.Fatalf("гібридний пошук: %v", err)
	}
	for _, m := range matches {
		if m.WorkProductID == b.ID {
			t.Fatalf("вектор під застарілою моделлю не мав потрапити у видачу поточної моделі: %+v", m)
		}
	}
	if len(matches) != 0 {
		t.Fatalf("очікувалося 0 результатів (єдиний інший WP лишився лише під застарілою моделлю), отримано %+v", matches)
	}
}
