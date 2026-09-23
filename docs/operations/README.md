# DELMOS: операційні посібники (Operations)

Дата: 2026-09-23. Статус: нормативний реєстр експлуатаційної документації.
Ліцензія: Apache 2.0.
Контекст: [ядро та архітектура](../architecture/ARCHITECTURE.md), [вимоги до середовища](../requirements/SYSTEM_REQUIREMENTS.md).

---

## 1. Реєстр операційних документів

| Документ | Призначення |
| --- | --- |
| [RUNBOOK.md](RUNBOOK.md) | Керування процесом служби, дерево діагностики (triage), відомі несправності та їх усунення |
| [BACKUP_AND_RESTORE.md](BACKUP_AND_RESTORE.md) | Процедури резервного копіювання PostgreSQL, розклад та перевірка відновлення |
| [DISASTER_RECOVERY.md](DISASTER_RECOVERY.md) | План аварійного відновлення, цілі RPO/RTO, регулярні навчання (DR drills) |
| [UPGRADE_AND_ROLLBACK.md](UPGRADE_AND_ROLLBACK.md) | Процедура оновлення версії, попередній перегляд міграцій, відкат при збої |
| [LOGGING_AND_OBSERVABILITY.md](LOGGING_AND_OBSERVABILITY.md) | Рівні журналювання, ротація логів, метрики та розподілене трасування |

## 2. Заплановані контрольні операції (Scheduled Controls)

| Операція | Періодичність | Відповідальний | Документ |
| --- | --- | --- | --- |
| Резервне копіювання бази даних | Щоденно | Host Operator | [BACKUP_AND_RESTORE.md](BACKUP_AND_RESTORE.md) |
| Репетиція відновлення (Restore Rehearsal) | Щотижня | Host Operator + System Administrator | [BACKUP_AND_RESTORE.md](BACKUP_AND_RESTORE.md) |
| Навчання з аварійного відновлення (DR Drill) | Щомісяця | System Administrator | [DISASTER_RECOVERY.md](DISASTER_RECOVERY.md) |
| Перевірка вільного дискового простору та ротації логів | Щотижня | Host Operator | [LOGGING_AND_OBSERVABILITY.md](LOGGING_AND_OBSERVABILITY.md) |
| Перевірка застосованих міграцій перед релізом | Перед кожним оновленням | System Administrator | [UPGRADE_AND_ROLLBACK.md](UPGRADE_AND_ROLLBACK.md) |

## 3. Ворота якості перед випуском

Після реалізації збірки та Makefile перед публікацією нової версії платформи має виконуватися команда:

```bash
make validate
```

Зараз її в каталозі DELMOS немає. Після реалізації команда перевірятиме форматування коду, типізацію, юніт-тести, цілісність документації та успішну збірку бінарника. Опис майбутнього переліку перевірок: [вимоги до середовища розробки](../requirements/SYSTEM_REQUIREMENTS.md#2-вимоги-до-середовища-розробки-development-environment).
