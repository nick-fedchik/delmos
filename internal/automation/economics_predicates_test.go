package automation

import "testing"

func evalCtx(fields map[string]any) EvalContext {
	return EvalContext{Fields: fields, Permissions: map[string]bool{}}
}

// Межовий випадок, заради якого предикати рахують у big.Rat: у float64
// 0.1+0.2 дає 0.30000000000000004, і витрата рівно на ліміт хибно вважалася б
// перевищенням.
func TestWithinFundingLimitIsExactAtBoundary(t *testing.T) {
	cases := []struct {
		name   string
		actual string
		limit  string
		want   bool
	}{
		{"рівно на ліміті", "0.30", "0.30", true},
		{"сума копійок рівно на ліміті", "0.30", "0.30", true},
		{"перевищення на копійку", "12000.01", "12000.00", false},
		{"під лімітом", "11999.99", "12000.00", true},
		{"ліміт не задано", "999999.00", "0", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := predicateWithinFundingLimit(
				evalCtx(map[string]any{"actual_cost": tc.actual, "funding_limit": tc.limit}), nil)
			if err != nil {
				t.Fatalf("несподівана помилка: %v", err)
			}
			if got != tc.want {
				t.Errorf("actual=%s limit=%s => %v, очікувано %v", tc.actual, tc.limit, got, tc.want)
			}
		})
	}
}

// Відсутність даних про витрати не має трактуватися як дотримання ліміту.
func TestWithinFundingLimitFailsWithoutActualCost(t *testing.T) {
	_, err := predicateWithinFundingLimit(
		evalCtx(map[string]any{"funding_limit": "1000.00"}), nil)
	if err == nil {
		t.Fatal("очікувалась помилка: немає даних про фактичні витрати")
	}
}

func TestCPIAboveThreshold(t *testing.T) {
	cases := []struct {
		name  string
		cpi   any
		want  bool
		fails bool
	}{
		{name: "рівно поріг", cpi: "0.85", want: true},
		{name: "нижче порога", cpi: "0.8499", want: false},
		{name: "вище порога", cpi: "1.02", want: true},
		{name: "CPI відсутній", cpi: nil, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]any{}
			if tc.cpi != nil {
				fields["cpi"] = tc.cpi
			}
			got, err := predicateCPIAbove(evalCtx(fields), map[string]any{"threshold": 0.85})
			if err != nil {
				t.Fatalf("несподівана помилка: %v", err)
			}
			if got != tc.want {
				t.Errorf("cpi=%v => %v, очікувано %v", tc.cpi, got, tc.want)
			}
		})
	}
}

// Предикати мають бути зареєстровані в платформі, інакше засіяні міграцією
// правила дадуть evaluation_error замість рішення.
func TestEconomicsPredicatesAreRegistered(t *testing.T) {
	for _, key := range []string{"within_funding_limit", "cpi_above"} {
		if _, ok := predicates[key]; !ok {
			t.Errorf("предикат %q не зареєстровано в реєстрі платформи", key)
		}
	}
}
