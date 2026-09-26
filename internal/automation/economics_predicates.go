package automation

import (
	"fmt"
	"math/big"
)

// Предикати проєктної економіки (ADR-006, SHR-13). Вони лишаються чистими
// функціями без доступу до БД, як вимагає SWR-26: показники здобутої цінності
// обчислюються заздалегідь і передаються через EvalContext.Fields.
//
// Порівняння виконуються над math/big.Rat, а не float64: десяткові суми на
// кшталт 0.1+0.2 у двійковому float дають похибку, і шлюз бюджету спрацював
// би хибно на межі ліміту.

func init() {
	predicates["within_funding_limit"] = predicateWithinFundingLimit
	predicates["cpi_above"] = predicateCPIAbove
}

// predicateWithinFundingLimit істинний, поки фактичні витрати не перевищили
// ліміт фінансування. Відсутність ліміту трактується як «ліміту не задано»,
// тобто перевірка незастосовна й не блокує перехід.
func predicateWithinFundingLimit(ctx EvalContext, _ map[string]any) (bool, error) {
	limit, ok, err := ratField(ctx, "funding_limit")
	if err != nil {
		return false, err
	}
	if !ok || limit.Sign() == 0 {
		return true, nil
	}
	actual, ok, err := ratField(ctx, "actual_cost")
	if err != nil {
		return false, err
	}
	if !ok {
		// Немає даних про витрати — це не привід стверджувати, що ліміт
		// дотримано (METRICS.md §6: no_data не дорівнює «успіх»).
		return false, fmt.Errorf("within_funding_limit: відсутнє поле actual_cost")
	}
	return actual.Cmp(limit) <= 0, nil
}

// predicateCPIAbove істинний, поки індекс виконання бюджету не нижчий за
// поріг. Відсутній CPI означає, що робіт ще не здобуто — попередження про
// перевитрату в цьому стані беззмістовне.
func predicateCPIAbove(ctx EvalContext, params map[string]any) (bool, error) {
	threshold, err := toRat(params["threshold"])
	if err != nil {
		return false, fmt.Errorf("cpi_above: некоректний параметр threshold: %w", err)
	}
	cpi, ok, err := ratField(ctx, "cpi")
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	return cpi.Cmp(threshold) >= 0, nil
}

func ratField(ctx EvalContext, name string) (*big.Rat, bool, error) {
	raw, exists := ctx.Fields[name]
	if !exists || raw == nil {
		return nil, false, nil
	}
	value, err := toRat(raw)
	if err != nil {
		return nil, false, fmt.Errorf("поле %s: %w", name, err)
	}
	return value, true, nil
}

func toRat(raw any) (*big.Rat, error) {
	if raw == nil {
		return nil, fmt.Errorf("значення відсутнє")
	}
	value := new(big.Rat)
	if _, ok := value.SetString(fmt.Sprint(raw)); !ok {
		return nil, fmt.Errorf("не є десятковим числом: %v", raw)
	}
	return value, nil
}
