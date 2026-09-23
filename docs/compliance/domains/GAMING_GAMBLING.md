# DELMOS: домен комплаєнсу — iGaming, Gambling, Lottery & Sports Betting (Азартні ігри, лотереї, букмекерство)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md),
[INFORMATION_SECURITY_CYBERSECURITY.md](INFORMATION_SECURITY_CYBERSECURITY.md), [PERSONAL_DATA_PROTECTION.md](PERSONAL_DATA_PROTECTION.md),
[FINANCIAL.md](FINANCIAL.md).

---

## 1. Огляд домену

Домен азартних ігор (наземні казино, iGaming-платформи, лотереї, спортивний беттинг) переважно software-орієнтований (як і [FINANCIAL.md](FINANCIAL.md)), але з двома специфічними технічними вимірами, відсутніми у фінтеху: **сертифікація генераторів випадкових чисел та математичної моделі гри** (RNG/Game Math Certification) та **моніторинг цілісності ставок** (Betting Integrity Monitoring). Апаратний вимір з'являється у наземних казино (ігрові автомати, термінали) — регулюється тими самими лабораторними стандартами (GLI), що й програмні платформи. Ключова відмінність від інших доменів: ліцензування є **юрисдикційно роздробленим** — кожна країна/штат (UKGC, MGA Malta, Curaçao, окремі штати США) має власні технічні вимоги та цикл переатестації.

## 2. Ключові стандарти

| Стандарт / Орган | Область застосування |
| --- | --- |
| GLI-11 | Ігрові автомати та термінали наземних казино (апаратна + програмна частина) |
| GLI-19 | Інтерактивні ігрові системи (iGaming-платформи, онлайн-казино, вимоги до RNG) |
| GLI-31 | Системи спортивного букмекерства (Sports Wagering Systems) |
| GLI-33 | Лотерейні системи та термінали |
| WLA-SCS:2020 (World Lottery Association Security Control Standard) | Комплексна система контролю безпеки для операторів лотерей (аналог ISO 27001, адаптований для лотерей) |
| UKGC LCCP + RTS (Remote Gambling and Software Technical Standards) | Умови ліцензії та технічні стандарти віддаленого гемблінгу у Великій Британії |
| MUSL / NASPL Technical Standards | Технічні стандарти багатоштатних лотерейних асоціацій (США) |
| ISO/IEC 17025 | Акредитація незалежних випробувальних лабораторій (GLI, eCOGRA, BMM Testlabs, iTech Labs), що сертифікують RNG |
| FATF Recommendation 16 / EU AMLD 5-6 | Вимоги AML/KYC для операторів азартних ігор як «зобов'язаних суб'єктів» |
| IBIA / ESSA Integrity Standards | Моніторинг цілісності спортивних ставок, виявлення підозрілих патернів (match-fixing) |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.gaming_rng_fairness`
- **Внесок у Project Plan:** розділ `RNG Certification & Game Mathematics Plan`.
- **Словники:** `RNG_CERTIFICATION_LAB` (GLI, eCOGRA, BMM Testlabs, iTech Labs), `GAME_MATH_MODEL_TYPE` (RTP-таблиця, волатильність, hit frequency).
- **Майлстоуни:** `rng_certification_review`, `game_math_verification`, `periodic_recertification` (після кожної зміни коду гри).
- **Артефакти:** `gaming:rng_certification_report`, `gaming:game_math_model_verification`, `gaming:payout_percentage_disclosure` (теоретичний RTP).
- **Правила:** `rule.gaming.rtp_disclosure_required`: артефакт релізу гри блокується (`veto`), якщо відсутній опублікований теоретичний відсоток повернення (RTP).

### Плановий модуль: `compliance.gambling_aml_kyc`
- **Словники:** `KYC_VERIFICATION_LEVEL` (`basic`, `enhanced`, `full`), `AML_RISK_TIER`.
- **Артефакти:** `gambling:kyc_verification_record`, `gambling:suspicious_activity_report` (SAR).
- **Правила:** `rule.gambling.sar_filing_deadline`: запис `suspicious_activity_report` фіксує обов'язковий дедлайн подання регулятору відповідно до юрисдикції.

### Плановий модуль: `compliance.sports_betting_integrity`
- **Словники:** `INTEGRITY_ALERT_TYPE` (аномальний обсяг ставок, підозра на договірний матч).
- **Артефакти:** `sports_betting:integrity_monitoring_report` (інтеграція з фідами IBIA/аналогічних систем моніторингу).
- **Правила:** `rule.sports_betting.alert_requires_investigation_record`: сповіщення про підозрілу активність вимагає створення розслідувального запису протягом визначеного терміну.

### Плановий модуль: `compliance.lottery_wla_scs`
- **Артефакти:** `lottery:wla_scs_certification`, `lottery:draw_integrity_record` (доказ цілісності розіграшу).

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`rng_certification_report`, `kyc_verification_record`, `integrity_monitoring_report`), `DictionaryProvider` (реєстр юрисдикцій та ліцензій, лабораторій сертифікації), `MilestoneProvider` (переатестація RNG після кожної зміни коду гри, подання SAR), `TriggerRuleProvider` (обов'язковість розкриття RTP, дедлайни AML), `IntegrationConnection` (фіди зовнішнього моніторингу цілісності ставок, національні реєстри самовиключення на кшталт GAMSTOP).
* Прогалина 1 (юрисдикційна роздробленість): на відміну від решти доменів, ліцензування тут вимагає моделювання **багатьох паралельних юрисдикцій для одного проєкту** (UKGC + MGA + окремий штат США одночасно) з різними циклами переатестації — потрібна сутність `LicenseJurisdiction`, відсутня в поточній предметній моделі ([DOMAIN_MODEL.md](../../architecture/DOMAIN_MODEL.md)).
* Прогалина 2 (незалежна лабораторна сертифікація): RNG-сертифікат видається зовнішньою акредитованою лабораторією і має обмежений термін дії — платформі потрібен механізм імпорту зовнішнього сертифіката як `Report` з автоматичним нагадуванням про закінчення терміну дії (аналогічно періодичному скануванню вразливостей у [FINANCIAL.md](FINANCIAL.md)).
* Перетин із наскрізними доменами: AML/KYC спирається на ту саму модель захисту персональних даних, що й [PERSONAL_DATA_PROTECTION.md](PERSONAL_DATA_PROTECTION.md) (зберігання документів посвідчення особи), а загальна кібербезпека платформи — на мінімум [INFORMATION_SECURITY_CYBERSECURITY.md](INFORMATION_SECURITY_CYBERSECURITY.md).

## 5. Посилання

* [MODULE_CATALOG.md](../../architecture/MODULE_CATALOG.md) — зразок структури галузевого модуля.
* [FINANCIAL.md](FINANCIAL.md) — суміжний домен з подібними вимогами AML/KYC та контролю змін.
* [PERSONAL_DATA_PROTECTION.md](PERSONAL_DATA_PROTECTION.md) — захист даних верифікації особи (KYC).
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
