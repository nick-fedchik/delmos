package automation_test

import (
	"testing"

	"delmos/internal/automation"
)

func TestEvaluateCompositeConditions(t *testing.T) {
	ctx := automation.EvalContext{Fields: map[string]any{"status": "draft"}, Permissions: map[string]bool{"wp.approve": true}}

	cases := []struct {
		name string
		node automation.ConditionNode
		want bool
	}{
		{"always_true", automation.ConditionNode{PredicateKey: "always_true"}, true},
		{"always_false", automation.ConditionNode{PredicateKey: "always_false"}, false},
		{"field_equals match", automation.ConditionNode{PredicateKey: "field_equals", Params: map[string]any{"field": "status", "value": "draft"}}, true},
		{"field_equals mismatch", automation.ConditionNode{PredicateKey: "field_equals", Params: map[string]any{"field": "status", "value": "obsolete"}}, false},
		{"field_not_empty", automation.ConditionNode{PredicateKey: "field_not_empty", Params: map[string]any{"field": "status"}}, true},
		{"permission granted", automation.ConditionNode{PredicateKey: "permission", Params: map[string]any{"permission": "wp.approve"}}, true},
		{"permission denied", automation.ConditionNode{PredicateKey: "permission", Params: map[string]any{"permission": "wp.retire"}}, false},
		{
			"and short-circuits",
			automation.ConditionNode{PredicateKey: "and", SubConditions: []automation.ConditionNode{
				{PredicateKey: "always_true"}, {PredicateKey: "always_false"},
			}},
			false,
		},
		{
			"or finds true",
			automation.ConditionNode{PredicateKey: "or", SubConditions: []automation.ConditionNode{
				{PredicateKey: "always_false"}, {PredicateKey: "always_true"},
			}},
			true,
		},
		{
			"not inverts",
			automation.ConditionNode{PredicateKey: "not", SubConditions: []automation.ConditionNode{
				{PredicateKey: "field_equals", Params: map[string]any{"field": "status", "value": "obsolete"}},
			}},
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := automation.Evaluate(ctx, tc.node)
			if err != nil {
				t.Fatalf("неочікувана помилка: %v", err)
			}
			if got != tc.want {
				t.Fatalf("очікувалося %v, отримано %v", tc.want, got)
			}
		})
	}
}

func TestEvaluateUnknownPredicateIsError(t *testing.T) {
	_, err := automation.Evaluate(automation.EvalContext{}, automation.ConditionNode{PredicateKey: "not_registered"})
	if err == nil {
		t.Fatal("очікувалася помилка для незареєстрованого предиката")
	}
}
