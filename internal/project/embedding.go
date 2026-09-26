// Package project: локальний провайдер векторних ембедінгів (VEC-01..03,
// VECTOR_AND_GRAPH_DATA.md §2). Це тимчасова реалізація без зовнішньої мережі
// й без секретів: детермінований "hashing trick" (feature hashing) мішка слів
// нормалізованого тексту в фіксовану розмірність pgvector. Він не є
// семантичною моделлю (не розуміє синонімів чи парафраз), але коректно
// відтворює лексичну подібність (спільні слова → вища косинусна схожість) і
// повністю замкнений у процесі: без API-ключів, без мережевих викликів,
// без OWASP-ризиків витоку тексту специфікацій третій стороні.
//
// Інтерфейс EmbeddingProvider навмисно єдиний спосіб отримати вектор, щоб
// реальну модель (наприклад, локальний bge-m3 або зовнішній сервіс) можна
// було підключити пізніше, не змінюючи схему чи виклики Store — рівно так,
// як передбачає версіонування (model_name, model_version) у VEC-03.
package project

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	embeddingDimensions   = 1536
	embeddingModelName    = "local-feature-hash"
	embeddingModelVersion = "1"
)

// EmbeddingProvider обчислює вектор фіксованої розмірності для тексту
// артефакту. Реалізація підключається до Store через NewStore/WithEmbeddings.
type EmbeddingProvider interface {
	ModelName() string
	ModelVersion() string
	Dimensions() int
	Embed(text string) []float32
}

// localFeatureHashProvider — "hashing trick" bag-of-words: кожен токен тексту
// хешується у бакет [0, dims) зі знаком (другий хеш-біт), результат
// L2-нормалізується. Класична техніка feature hashing (Weinberger et al.,
// 2009) для представлення тексту без словника фіксованого розміру.
type localFeatureHashProvider struct{}

// NewLocalFeatureHashProvider повертає провайдер без зовнішніх залежностей.
func NewLocalFeatureHashProvider() EmbeddingProvider {
	return localFeatureHashProvider{}
}

func (localFeatureHashProvider) ModelName() string    { return embeddingModelName }
func (localFeatureHashProvider) ModelVersion() string { return embeddingModelVersion }
func (localFeatureHashProvider) Dimensions() int      { return embeddingDimensions }

func (localFeatureHashProvider) Embed(text string) []float32 {
	vector := make([]float32, embeddingDimensions)
	for _, token := range tokenize(text) {
		bucket, sign := hashToken(token)
		vector[bucket] += sign
	}
	normalize(vector)
	return vector
}

// tokenize розбиває текст на слова нижнього регістру довжиною від 2 символів
// (короткі сполучники/прийменники не несуть лексичної ваги для подібності).
func tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !isWordRune(r)
	})
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) >= 2 {
			tokens = append(tokens, field)
		}
	}
	return tokens
}

// isWordRune визначає символи слова (латиниця, кирилиця, цифри) для
// токенізації без зовнішніх бібліотек Unicode-класифікації.
func isWordRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= 'а' && r <= 'я' || r == 'і' || r == 'ї' || r == 'є'
}

// hashToken повертає індекс бакета та знак внеску токена (стандартний прийом
// feature hashing для зменшення упередженості колізій хешу).
func hashToken(token string) (int, float32) {
	h := fnv.New64a()
	_, _ = h.Write([]byte(token))
	sum := h.Sum64()
	bucket := int(sum % uint64(embeddingDimensions))

	signHash := fnv.New32a()
	_, _ = signHash.Write([]byte("sign:" + token))
	if signHash.Sum32()%2 == 0 {
		return bucket, 1
	}
	return bucket, -1
}

func normalize(vector []float32) {
	var sumSquares float64
	for _, v := range vector {
		sumSquares += float64(v) * float64(v)
	}
	if sumSquares == 0 {
		return
	}
	norm := math.Sqrt(sumSquares)
	for i, v := range vector {
		vector[i] = float32(float64(v) / norm)
	}
}

// embeddingText формує нормалізований текст для векторизації: заголовок і
// тіло ревізії (VECTOR_AND_GRAPH_DATA.md §2.2).
func embeddingText(title, body string) string {
	return title + "\n\n" + body
}

// formatVectorLiteral кодує вектор у текстовий формат вводу pgvector
// (`[v1,v2,...]`), щоб уникнути додаткової залежності від драйвера типів
// vector: PostgreSQL приймає й повертає такий текст через каст `::vector`.
func formatVectorLiteral(vector []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range vector {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(v), 'f', 6, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// embeddableWorkProductTypes — типи, для яких MVP рахує вектори (дедуплікація
// вимог і рекомендація тест-кейсів, ROADMAP.md Етап 3). Інші типи (plan,
// architecture, report, record) поки не векторизуються.
var embeddableWorkProductTypes = map[string]bool{
	"requirement": true,
	"test_spec":   true,
}

// upsertEmbedding рахує й зберігає вектор нової ревізії в тій самій
// транзакції, що й сама ревізія (VEC-01). SWR-36.2 передбачає асинхронний
// підхід через outbox (Етап 4, ще не реалізований) — до появи цієї черги
// розрахунок виконується синхронно локальним провайдером без мережі, щоб не
// блокувати артефакт непередбачуваною зовнішньою залежністю.
func upsertEmbedding(ctx context.Context, tx pgx.Tx, wpType string, workProductID, revisionID uuid.UUID, title, body string) error {
	if !embeddableWorkProductTypes[wpType] {
		return nil
	}
	provider := NewLocalFeatureHashProvider()
	vector := provider.Embed(embeddingText(title, body))

	_, err := tx.Exec(ctx,
		`INSERT INTO core.wp_embeddings (work_product_id, revision_id, model_name, model_version, dimensions, embedding)
		 VALUES ($1, $2, $3, $4, $5, $6::vector)
		 ON CONFLICT (revision_id, model_name, model_version) DO NOTHING`,
		workProductID, revisionID, provider.ModelName(), provider.ModelVersion(), provider.Dimensions(), formatVectorLiteral(vector))
	if err != nil {
		return fmt.Errorf("збереження ембедінгу ревізії: %w", err)
	}
	return nil
}

// SimilarWorkProduct — рядок гібридного пошуку (VEC-02): семантична
// схожість, відфільтрована за проєктом, типом і моделлю ембедінгів.
type SimilarWorkProduct struct {
	WorkProductID    uuid.UUID
	Code             string
	Title            string
	Status           string
	CosineSimilarity float64
}

// FindSimilarWorkProducts шукає найближчі за косинусною відстанню артефакти
// того самого типу в межах проєкту, використовуючи вектор конкретної ревізії
// як запит. Права доступу застосовуються фільтром project_id на рівні SQL —
// недоступні документи не потрапляють у видачу (VEC-02).
func (s *Store) FindSimilarWorkProducts(ctx context.Context, projectID, excludeWorkProductID uuid.UUID, queryRevisionID uuid.UUID, limit int, minSimilarity float64) ([]SimilarWorkProduct, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	provider := NewLocalFeatureHashProvider()

	rows, err := s.pool.Query(ctx, `
		WITH query_vector AS (
			SELECT embedding FROM core.wp_embeddings
			WHERE revision_id = $1 AND model_name = $2 AND model_version = $3
		)
		SELECT wp.id, wp.code, wp.title, wp.status, 1 - (e.embedding <=> qv.embedding) AS cosine_similarity
		FROM core.wp_embeddings e
		JOIN query_vector qv ON true
		JOIN core.work_products wp ON wp.id = e.work_product_id
		WHERE wp.project_id = $4
		  AND e.model_name = $2 AND e.model_version = $3
		  AND wp.id != $5
		  AND 1 - (e.embedding <=> qv.embedding) >= $6
		ORDER BY e.embedding <=> qv.embedding
		LIMIT $7`,
		queryRevisionID, provider.ModelName(), provider.ModelVersion(), projectID, excludeWorkProductID, minSimilarity, limit)
	if err != nil {
		return nil, fmt.Errorf("гібридний семантичний пошук: %w", err)
	}
	defer rows.Close()

	var results []SimilarWorkProduct
	for rows.Next() {
		var row SimilarWorkProduct
		if err := rows.Scan(&row.WorkProductID, &row.Code, &row.Title, &row.Status, &row.CosineSimilarity); err != nil {
			return nil, fmt.Errorf("розбір результату семантичного пошуку: %w", err)
		}
		results = append(results, row)
	}
	return results, rows.Err()
}
