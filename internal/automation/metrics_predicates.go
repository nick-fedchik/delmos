package automation

import "fmt"

// Предикат якості метрик (SWR-22.3). Як і предикати економіки, він лишається
// чистою функцією: перелік метрик, що блокують шлюз, обчислюється в
// транзакції переходу фази і передається через EvalContext.Fields.

// FieldMetricBlockers — назва факту з переліком ключів метрик, які не мають
// чинного вимірювання.
const FieldMetricBlockers = "metric_blockers"

func init() {
	predicates["metrics_fresh"] = predicateMetricsFresh
}

// predicateMetricsFresh істинний, коли жодна обов'язкова для шлюзу метрика не
// є відсутньою, застарілою чи помилковою.
//
// Відсутність самого факту трактується як відмова, а не як згода: якщо
// перевірку якості не виконано, немає підстав стверджувати, що метрики чинні
// (METRICS.md §6).
func predicateMetricsFresh(ctx EvalContext, _ map[string]any) (bool, error) {
	raw, exists := ctx.Fields[FieldMetricBlockers]
	if !exists {
		return false, fmt.Errorf("metrics_fresh: відсутнє поле %s", FieldMetricBlockers)
	}
	blockers, ok := raw.([]string)
	if !ok {
		return false, fmt.Errorf("metrics_fresh: поле %s має тип %T, очікувано []string", FieldMetricBlockers, raw)
	}
	return len(blockers) == 0, nil
}
