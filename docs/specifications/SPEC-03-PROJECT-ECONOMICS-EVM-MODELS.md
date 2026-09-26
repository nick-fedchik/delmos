# SPEC-03: Технічна специфікація проєктної економіки, ресурсів та моделей EVM

Дата: 2026-09-23. Статус: нормативна технічна специфікація економічного рушія.  
Ліцензія: Apache 2.0.  
Контекст: [каталог модулів](../architecture/MODULE_CATALOG.md), [метрики та економіка](../architecture/METRICS.md),
[ADR-006](../architecture/decisions/ADR-006-project-economics-resource-accounting.md),
[SWR-39..41](../requirements/software/SWR-09-project-economics-resources.md).

---

## 1. Реляційні моделі даних у PostgreSQL

### 1.1. Облік відпрацьованого часу (Work Records)

```sql
CREATE TABLE work_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    subject_id UUID NOT NULL, -- посилання на WorkProduct або WorkItem
    work_date DATE NOT NULL,
    duration_hours NUMERIC(6, 2) NOT NULL CHECK (duration_hours > 0 AND duration_hours <= 24),
    work_category VARCHAR(32) NOT NULL DEFAULT 'development', -- 'design', 'development', 'review', 'testing'
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX idx_work_records_project_date ON work_records(project_id, work_date);
CREATE INDEX idx_work_records_user ON work_records(user_id, work_date);
```

### 1.2. Базовий кошторис (Cost Baseline) та ставки ролей

```sql
CREATE TABLE labor_rates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE, -- NULL для системної ставки за замовчуванням
    role_key VARCHAR(64) NOT NULL,
    hourly_rate NUMERIC(10, 2) NOT NULL CHECK (hourly_rate >= 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'EUR',
    valid_from DATE NOT NULL,
    valid_to DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT uq_role_rate_period UNIQUE (project_id, role_key, valid_from)
);

CREATE TABLE cost_baselines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    name VARCHAR(255) NOT NULL,
    total_budget NUMERIC(14, 2) NOT NULL CHECK (total_budget >= 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'EUR',
    status VARCHAR(32) NOT NULL DEFAULT 'approved',
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT uq_project_cost_baseline UNIQUE (project_id, version)
);

CREATE TABLE budget_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cost_baseline_id UUID NOT NULL REFERENCES cost_baselines(id) ON DELETE CASCADE,
    phase_id VARCHAR(64),
    cost_category VARCHAR(32) NOT NULL, -- 'Labor', 'Hardware', 'NRE', 'Licenses', 'Testing', 'Contingency'
    planned_amount NUMERIC(14, 2) NOT NULL CHECK (planned_amount >= 0),
    funding_limit NUMERIC(14, 2) NOT NULL CHECK (funding_limit >= planned_amount)
);

CREATE TABLE expense_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    phase_id VARCHAR(64),
    cost_category VARCHAR(32) NOT NULL,
    expense_type VARCHAR(16) NOT NULL CHECK (expense_type IN ('CapEx', 'OpEx')),
    amount NUMERIC(14, 2) NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'EUR',
    invoice_reference VARCHAR(128),
    expense_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
```

---

> **Відхилення реалізації від §1.1 (зафіксовано 2026-09-26).** У коді
> `core.work_records` має явний зовнішній ключ `work_product_id` замість
> нетипованого `subject_id`. Причина: сутності `WorkItem` у ядрі ще не існує,
> а UUID без зовнішнього ключа — діра в цілісності, яку СУБД не контролює.
> Коли `WorkItem` зʼявиться, додається окрема колонка з власним FK і CHECK на
> взаємну виключність. Також `core.cost_baselines` має колонку `created_by`
> та констрейнт `cost_baselines_segregation_check`, якого немає в §1.3:
> без автора неможливо перевірити самозатвердження.

## 2. Математичні алгоритми розрахунку EVM
Показники аналізу здобутої цінності (**Earned Value Management за ДСТУ ISO 21508:2022**, див. [ADR-011](../architecture/decisions/ADR-011-iso-21500-series-normative-base.md)) обчислюються на конкретну контрольну дату $T$. Позначення BCWS / BCWP / ACWP наведено довідково як усталені англомовні синоніми з практики PMBOK та SWEBOK:

### 2.1. Базові показники:
1. **Planned Value ($PV$):** Сума планових бюджетних призначень усіх робіт, запланованих до завершення на дату $T$:
   $$PV(T) = \sum_{i \in \text{Tasks}_{planned \le T}} \text{PlannedCost}(i)$$
2. **Actual Cost ($AC$):** Сума фактично понесених прямих трудовитрат та прямих витрат на дату $T$:
   $$AC(T) = \sum (\text{hours} \times \text{LaborRate}) + \sum_{\text{Expenses} \le T} \text{amount}$$
3. **Earned Value ($EV$):** Планова вартість фактично виконаних та затверджених артефактів/результатів на дату $T$:
   $$EV(T) = \sum_{j \in \text{Deliverables}_{approved \le T}} \text{PlannedValue}(j)$$

### 2.2. Індекси ефективності та прогнозування:
* **Cost Variance ($CV$):**
  $$CV = EV - AC \quad (\text{позитивне значення означає економію})$$
* **Schedule Variance ($SV$):**
  $$SV = EV - PV \quad (\text{позитивне значення означає випередження})$$
* **Cost Performance Index ($CPI$):**
  $$CPI = \frac{EV}{AC} \quad (\text{якщо } CPI < 1.0 \text{ — витрати перевищують бюджет})$$
* **Schedule Performance Index ($SPI$):**
  $$SPI = \frac{EV}{PV} \quad (\text{якщо } SPI < 1.0 \text{ — проєкт відстає від графіка})$$
* **Estimate at Completion ($EAC$):**
  $$EAC = \frac{BAC}{CPI} \quad (\text{де } BAC \text{ — Budget at Completion / загальний бюджет за планом})$$
