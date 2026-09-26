// cron.go — мінімальний 5-польовий парсер календарного розкладу
// (хвилина година день_місяця місяць день_тижня), навмисно вужчий за повний
// RFC 5545 (без секунд, без списку винятків, без "L"/"W"/"#"). Підтримує
// "*", точні числа, списки через кому та крок "*/N". Свідоме звуження
// обсягу SWR-24 calendar-режиму до найпоширенішого підмножинного синтаксису.
package automation

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type cronField struct {
	wildcard bool
	values   map[int]bool
}

func parseCronField(field string, min, max int) (cronField, error) {
	if field == "*" {
		return cronField{wildcard: true}, nil
	}
	values := make(map[int]bool)
	for _, part := range strings.Split(field, ",") {
		if strings.HasPrefix(part, "*/") {
			step, err := strconv.Atoi(strings.TrimPrefix(part, "*/"))
			if err != nil || step <= 0 {
				return cronField{}, fmt.Errorf("некоректний крок у полі cron: %q", part)
			}
			for v := min; v <= max; v += step {
				values[v] = true
			}
			continue
		}
		v, err := strconv.Atoi(part)
		if err != nil || v < min || v > max {
			return cronField{}, fmt.Errorf("некоректне значення поля cron: %q", part)
		}
		values[v] = true
	}
	return cronField{values: values}, nil
}

func (f cronField) matches(v int) bool {
	if f.wildcard {
		return true
	}
	return f.values[v]
}

// nextCronMatch шукає перший момент строго після from, що відповідає
// 5-польовому виразу cron, з точністю до хвилини. Обмежено пошуком на один
// рік уперед, щоб уникнути нескінченного циклу на некоректному виразі.
func nextCronMatch(cron string, from time.Time) (time.Time, error) {
	fields := strings.Fields(cron)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("cron-вираз має містити 5 полів (хвилина година день місяць день_тижня), отримано %q", cron)
	}
	minuteField, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return time.Time{}, err
	}
	hourField, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return time.Time{}, err
	}
	domField, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return time.Time{}, err
	}
	monthField, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return time.Time{}, err
	}
	dowField, err := parseCronField(fields[4], 0, 6)
	if err != nil {
		return time.Time{}, err
	}

	candidate := from.UTC().Add(time.Minute).Truncate(time.Minute)
	const maxIterations = 366 * 24 * 60
	for i := 0; i < maxIterations; i++ {
		if monthField.matches(int(candidate.Month())) &&
			domField.matches(candidate.Day()) &&
			dowField.matches(int(candidate.Weekday())) &&
			hourField.matches(candidate.Hour()) &&
			minuteField.matches(candidate.Minute()) {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}
	return time.Time{}, fmt.Errorf("не знайдено наступного настання для cron-виразу %q протягом року", cron)
}
