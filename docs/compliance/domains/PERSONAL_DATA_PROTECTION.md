# DELMOS: наскрізний стандарт — Personal Data Protection (Захист персональних даних)

Дата: 2026-09-23. Статус: наскрізний (горизонтальний) напрямок розширення, застосовний до будь-якого галузевого домену.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [INFORMATION_SECURITY_CYBERSECURITY.md](INFORMATION_SECURITY_CYBERSECURITY.md),
[security/SECURITY_SPECIFICATION.md](../../security/SECURITY_SPECIFICATION.md).

---

## 1. Чому цей стандарт не є галузевим доменом

Захист персональних даних — це горизонтальна юридична вимога, що діє паралельно з інформаційною безпекою ([INFORMATION_SECURITY_CYBERSECURITY.md](INFORMATION_SECURITY_CYBERSECURITY.md)), але має власний предмет регулювання: права суб'єкта даних (фізичної особи), а не лише технічний захист систем. Застосовується до будь-якого проєкту DELMOS, що обробляє дані користувачів, пацієнтів, працівників чи клієнтів — незалежно від галузі.

## 2. Європейський комплаєнс (EU)

| Регламент / Директива | Область застосування |
| --- | --- |
| GDPR (Regulation (EU) 2016/679) | Загальний регламент захисту персональних даних — права суб'єкта даних, законні підстави обробки, DPIA, повідомлення про витік протягом 72 годин |
| ePrivacy Directive 2002/58/EC (та проєкт ePrivacy Regulation) | Конфіденційність електронних комунікацій, файли cookie, прямий маркетинг |
| EU-U.S. Data Privacy Framework (DPF) | Правова підстава для транскордонної передачі персональних даних з ЄС до США |
| NIS2 Directive | Кібербезпека операторів критичної інфраструктури (перетинається з [INFORMATION_SECURITY_CYBERSECURITY.md](INFORMATION_SECURITY_CYBERSECURITY.md)) |

## 3. Американський комплаєнс (US)

| Закон / Регламент | Область застосування |
| --- | --- |
| CCPA / CPRA (California Consumer Privacy Act / California Privacy Rights Act) | Права споживачів Каліфорнії щодо власних персональних даних (доступ, видалення, відмова від продажу) |
| HIPAA Privacy & Security Rule | Захист медичної інформації пацієнтів (перетинається з доменом [MEDICAL.md](MEDICAL.md)) |
| GLBA (Gramm-Leach-Bliley Act) | Захист непублічної фінансової інформації клієнтів (перетинається з доменом [FINANCIAL.md](FINANCIAL.md)) |
| COPPA (Children's Online Privacy Protection Act) | Захист персональних даних дітей віком до 13 років |
| Державні закони штатів (Virginia CDPA, Colorado CPA, Connecticut CTDPA та ін.) | Регіональні вимоги до обробки персональних даних за межами Каліфорнії |
| NIST Privacy Framework 1.0 | Добровільна методологія управління ризиками приватності (аналог NIST CSF для privacy) |

## 4. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.gdpr`
- **Внесок у Project Plan:** розділ `Data Protection & Privacy Plan`.
- **Словники:** `LAWFUL_BASIS` (`consent`, `contract`, `legal_obligation`, `vital_interest`, `public_task`, `legitimate_interest`), `DATA_SUBJECT_RIGHT` (`access`, `rectification`, `erasure`, `portability`, `objection`).
- **Майлстоуни:** `dpia_completion` (Data Protection Impact Assessment), `breach_notification_drill`.
- **Артефакти:** `gdpr:record_of_processing_activities` (ROPA, ст. 30 GDPR), `gdpr:data_protection_impact_assessment`, `gdpr:data_subject_request_log`.
- **Правила:** `rule.gdpr.breach_notification_72h`: запис про витік даних (`data_subject_request_log` типу `breach`) вимагає позначки дедлайну сповіщення регулятора не пізніше 72 годин.

### Плановий модуль: `compliance.ccpa_cpra`
- **Словники:** `CONSUMER_REQUEST_TYPE` (`know`, `delete`, `opt_out_of_sale`, `correct`, `limit_use_of_sensitive_data`).
- **Артефакти:** `ccpa:consumer_request_log`, `ccpa:opt_out_mechanism_verification`.

## 5. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`record_of_processing_activities`, `data_protection_impact_assessment`, `consumer_request_log`), `DictionaryProvider` (правові підстави обробки, типи запитів суб'єктів даних), `MilestoneProvider` (DPIA, навчання реагування на витік), `TriggerRuleProvider` (дедлайн сповіщення про витік 72 години).
* Пряма залежність від наявної специфікації безпеки: виконання прав на видалення (`erasure`) персональних даних перетинається з інваріантом незмінності ревізій ([WORK_PRODUCTS.md](../../architecture/WORK_PRODUCTS.md)) — потрібне явне архітектурне рішення (майбутній ADR) щодо способу узгодження права на забуття з незмінними доказовими ревізіями (наприклад, псевдонімізація полів персональних даних замість фізичного видалення ревізії).
* Прогалина: платформа поки не має вбудованого реєстру обробки даних (`Record of Processing Activities`, ст. 30 GDPR) як формальної сутності — це головний артефакт, що потребує реалізації першим.

## 6. Посилання

* [INFORMATION_SECURITY_CYBERSECURITY.md](INFORMATION_SECURITY_CYBERSECURITY.md) — суміжний наскрізний стандарт технічної безпеки.
* [security/SECURITY_SPECIFICATION.md](../../security/SECURITY_SPECIFICATION.md) — шифрування персональних даних у стані спокою/передачі.
* [MEDICAL.md](MEDICAL.md), [FINANCIAL.md](FINANCIAL.md) — галузеві домени з додатковими вимогами до персональних даних (HIPAA, GLBA).
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
