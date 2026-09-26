-- 0016: справжній розподіл обов'язків для кошторису та подія переходу фази.
--
-- Виправляє дефекти, виявлені ревізією коду:
--   1) core.cost_baselines не мала created_by, тож система не знала, хто склав
--      кошторис, і перевірити самозатвердження було технічно неможливо.
--      Розділення існувало лише на рівні ключів прав.
--   2) Перехід фази не публікував подію, хоча всі інші команди публікують.

ALTER TABLE core.cost_baselines
    ADD COLUMN created_by uuid REFERENCES core.users(id) ON DELETE RESTRICT;

-- Наявні рядки (лише dev-дані) отримують автора проєкту: історію складання
-- відновити нізвідки, а лишати NULL означало б обійти перевірку нижче.
UPDATE core.cost_baselines cb
SET created_by = p.created_by
FROM core.projects p
WHERE p.id = cb.project_id AND cb.created_by IS NULL;

ALTER TABLE core.cost_baselines
    ALTER COLUMN created_by SET NOT NULL;

-- Розподіл обов'язків на рівні рядка: складач кошторису не може бути його
-- затверджувачем. Перевірку тримає СУБД, а не код, бо це контроль, дотичний
-- до фінансів — помилка в застосунку не повинна його знімати.
ALTER TABLE core.cost_baselines
    ADD CONSTRAINT cost_baselines_segregation_check
    CHECK (approved_by IS NULL OR approved_by <> created_by);

-- Подія закриття/відкриття фази: на неї підписуватиметься реєстр метрик
-- (SWR-05), тож вона потрібна до його появи.
INSERT INTO core.event_definitions (event_key, name, owner_module) VALUES
    ('phase.transitioned', 'Фаза проєкту змінила статус', 'core');
