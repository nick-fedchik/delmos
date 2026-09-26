package automation

import (
	"fmt"
)

// ConditionNode — типізоване дерево умов (SWR-26): жодного довільного SQL/JS,
// лише зареєстровані предикати з фіксованим бюджетом виконання.
// PredicateKey "and"/"or"/"not" — композитні вузли з SubConditions;
// будь-яке інше значення шукається в реєстрі predicates як листовий предикат.
type ConditionNode struct {
	PredicateKey  string          `json:"predicate_key"`
	Params        map[string]any  `json:"params,omitempty"`
	SubConditions []ConditionNode `json:"sub_conditions,omitempty"`
}

// EvalContext — контекст обчислення предикатів: іменовані поля сутності та
// набір активних прав актора в межах проєкту.
type EvalContext struct {
	Fields      map[string]any
	Permissions map[string]bool
}

// PredicateFunc — сигнатура зареєстрованого листового предиката.
type PredicateFunc func(ctx EvalContext, params map[string]any) (bool, error)

// predicates — реєстр платформи (SWR-26). Розширюється лише кодом ядра чи
// модулів, ніколи текстом плану/маніфесту.
var predicates = map[string]PredicateFunc{
	"always_true":     func(EvalContext, map[string]any) (bool, error) { return true, nil },
	"always_false":    func(EvalContext, map[string]any) (bool, error) { return false, nil },
	"field_equals":    predicateFieldEquals,
	"field_not_empty": predicateFieldNotEmpty,
	"permission":      predicatePermission,
	// schema_valid — заглушка до появи валідатора JSON Schema Draft 2020-12
	// (Етап 2, ще не реалізований окремо від композитних специфікацій).
	"schema_valid": func(EvalContext, map[string]any) (bool, error) { return true, nil },
}

func predicateFieldEquals(ctx EvalContext, params map[string]any) (bool, error) {
	field, ok := params["field"].(string)
	if !ok {
		return false, fmt.Errorf("field_equals: відсутній параметр field")
	}
	expected, hasExpected := params["value"]
	if !hasExpected {
		return false, fmt.Errorf("field_equals: відсутній параметр value")
	}
	actual, exists := ctx.Fields[field]
	if !exists {
		return false, nil
	}
	return fmt.Sprint(actual) == fmt.Sprint(expected), nil
}

func predicateFieldNotEmpty(ctx EvalContext, params map[string]any) (bool, error) {
	field, ok := params["field"].(string)
	if !ok {
		return false, fmt.Errorf("field_not_empty: відсутній параметр field")
	}
	actual, exists := ctx.Fields[field]
	if !exists {
		return false, nil
	}
	s := fmt.Sprint(actual)
	return s != "", nil
}

func predicatePermission(ctx EvalContext, params map[string]any) (bool, error) {
	key, ok := params["permission"].(string)
	if !ok {
		return false, fmt.Errorf("permission: відсутній параметр permission")
	}
	return ctx.Permissions[key], nil
}

// Evaluate рекурсивно обчислює дерево умов. Композитні вузли (and/or/not)
// обробляються тут для короткого замикання; листові — через реєстр.
func Evaluate(ctx EvalContext, node ConditionNode) (bool, error) {
	switch node.PredicateKey {
	case "and":
		for _, sub := range node.SubConditions {
			ok, err := Evaluate(ctx, sub)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	case "or":
		for _, sub := range node.SubConditions {
			ok, err := Evaluate(ctx, sub)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	case "not":
		if len(node.SubConditions) != 1 {
			return false, fmt.Errorf("not: очікується рівно одна підумова, отримано %d", len(node.SubConditions))
		}
		ok, err := Evaluate(ctx, node.SubConditions[0])
		if err != nil {
			return false, err
		}
		return !ok, nil
	default:
		fn, exists := predicates[node.PredicateKey]
		if !exists {
			return false, fmt.Errorf("невідомий предикат: %s", node.PredicateKey)
		}
		return fn(ctx, node.Params)
	}
}
