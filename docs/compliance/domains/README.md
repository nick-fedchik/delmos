# DELMOS: домени комплаєнсу (Compliance Domain Areas)

Дата: 2026-09-23. Статус: нормативний реєстр напрямків розширення комплаєнсу за галузями.
Ліцензія: Apache 2.0.
Контекст: [compliance/README.md](../README.md), [каталог модулів](../../architecture/MODULE_CATALOG.md),
[модульна система та провайдери розширень](../../architecture/MODULES.md),
[каталог сутностей та розширень](../../architecture/ENTITY_CATALOG.md).

---

## 1. Призначення

Кожен файл цієї директорії описує **напрямок розширення** платформи DELMOS для конкретного домену комплаєнсу, де існує усталена практика для HW/SW-проєктів. Це не готові модулі, а цільова специфікація: які стандарти покриваються, які типізовані провайдери розширень ([MODULES.md, розділ 3](../../architecture/MODULES.md#3-типізовані-провайдери-розширень-extension-providers)) потрібно задіяти, і які артефакти/словники/віхи/правила має постачати відповідний модуль `compliance.*`.

Домени поділяються на два роди:
* **Вертикальні галузеві домени** (розділ 2) — характерні для конкретної індустрії (Automotive, Aviation, Medicine тощо), не перетинаються між собою за предметом регулювання.
* **Наскрізні (горизонтальні) стандарти** (розділ 3) — застосовуються до будь-якого проєкту незалежно від галузі (інформаційна безпека, захист персональних даних); вертикальні домени доповнюють їх власними галузевими контролями, а не замінюють.

## 2. Реєстр вертикальних галузевих доменів

| Домен | Файл | Провідні стандарти | Стан у DELMOS |
| --- | --- | --- | --- |
| Automotive (Автомобілебудування) | [AUTOMOTIVE.md](AUTOMOTIVE.md) | ASPICE 4.0, ISO 26262, ISO 21434, ISO 21448, IATF 16949 | Цільові модулі описані, не реалізовані |
| Aviation & Aerospace (Авіація) | [AVIATION.md](AVIATION.md) | DO-178C, DO-254, ARP4754A, ARP4761, DO-160 | Заплановано |
| Medicine (Медичні вироби) | [MEDICAL.md](MEDICAL.md) | IEC 62304, ISO 14971, IEC 60601-1, EU MDR, FDA 21 CFR Part 820 | Заплановано |
| Marine & Offshore (Морська та офшорна техніка) | [MARINE.md](MARINE.md) | IEC 61508, IACS UR E22, DNV-GL, IMO SOLAS, IEC 60945 | Заплановано |
| Military & Defense (Військова та оборонна техніка) | [MILITARY_DEFENSE.md](MILITARY_DEFENSE.md) | MIL-STD-882E, DO-178C (airborne), MIL-STD-810, DEF STAN 00-56, STANAG 4671 | Заплановано |
| Financial Services (Фінансові технології) | [FINANCIAL.md](FINANCIAL.md) | PCI DSS, SOX, ISO 27001, GDPR, MiFID II | Заплановано |
| Railway (Залізничний транспорт) | [RAILWAY.md](RAILWAY.md) | EN 50126 (RAMS), EN 50128, EN 50129 | Заплановано |
| Industrial Automation & Machinery (Промислова автоматизація) | [INDUSTRIAL_AUTOMATION.md](INDUSTRIAL_AUTOMATION.md) | IEC 61508, IEC 62061, ISO 13849, IEC 61511 | Заплановано |
| Space (Космічна галузь) | [SPACE.md](SPACE.md) | ECSS-Q/E/M-серії, NASA-STD-8719, CCSDS, AS9100 | Заплановано |
| Nuclear Energy (Атомна енергетика) | [NUCLEAR_ENERGY.md](NUCLEAR_ENERGY.md) | IEC 61513, IEC 60880, IAEA SSG-39 | Заплановано |
| Power Generation & Distribution (Енергетика) | [POWER_GENERATION_DISTRIBUTION.md](POWER_GENERATION_DISTRIBUTION.md) | IEC 61850, IEC 62351, NERC CIP, IEEE 1547, EU NIS2 | Заплановано |
| Public Safety (Громадська безпека) | [PUBLIC_SAFETY.md](PUBLIC_SAFETY.md) | NFPA 1221, ANSI/TIA-102 (P25), EN 54, IEC 60839, ISO 22320 | Заплановано |
| IoT & Consumer Cybersecurity (Побутова електроніка та IoT) | [IOT_CYBERSECURITY.md](IOT_CYBERSECURITY.md) | ETSI EN 303 645, IEC 62443, NISTIR 8259 | Заплановано |
| iGaming, Gambling, Lottery & Sports Betting (Азартні ігри, лотереї, беттинг) | [GAMING_GAMBLING.md](GAMING_GAMBLING.md) | GLI-11/19/31/33, WLA-SCS:2020, UKGC LCCP/RTS, FATF R.16, IBIA | Заплановано |
| Digital Media (Цифрові медіа, мовлення, стрімінг) | [DIGITAL_MEDIA.md](DIGITAL_MEDIA.md) | SMPTE, ATSC 3.0/DVB, DRM (Widevine/PlayReady/FairPlay), FCC CVAA, EN 301 549 | Заплановано |
| Home Security (Побутові системи безпеки) | [HOME_SECURITY.md](HOME_SECURITY.md) | EN 50131, UL 1023/639, IEC 60839, UL 2900-2-3 | Заплановано |
| Telecom (Телекомунікації: оператори та обладнання) | [TELECOM.md](TELECOM.md) | 3GPP, ETSI EN 300/301, EU RED, FCC Part 22/24/27, GSMA NESAS | Заплановано |
| Commodity Networks & Wi-Fi (Побутові мережі Інтернет та Wi-Fi) | [COMMODITY_NETWORKS_WIFI.md](COMMODITY_NETWORKS_WIFI.md) | IEEE 802.11, Wi-Fi Alliance, FCC Part 15, EN 300 328/301 893, DOCSIS | Заплановано |
| Consumer Electronics (Побутові електронні пристрої) | [CONSUMER_ELECTRONICS.md](CONSUMER_ELECTRONICS.md) | IEC 62368-1, EU LVD/EMC/RoHS/WEEE/REACH, Ecodesign, IEC 62133/UN 38.3 | Заплановано |
| Biology Research (Біологічні дослідження) | [BIOLOGY_RESEARCH.md](BIOLOGY_RESEARCH.md) | WHO Biosafety Manual, CDC/NIH BMBL, OECD GLP, ICH E6(R2) GCP | Заплановано |
| Pharmaceutical (Фармацевтика) | [PHARMACEUTICAL.md](PHARMACEUTICAL.md) | FDA 21 CFR Part 210/211/11, EU GMP Annex 11, ICH Q7-Q10, GAMP 5, DSCSA/FMD | Заплановано |

## 3. Реєстр наскрізних (горизонтальних) стандартів

На відміну від вертикальних доменів (розділ 2), ці стандарти застосовуються **до будь-якого проєкту незалежно від галузі** — кожен вертикальний домен лише доповнює цей мінімум власними галузевими контролями.

| Наскрізний стандарт | Файл | Провідні стандарти | Стан у DELMOS |
| --- | --- | --- | --- |
| Information Security & Cybersecurity (Інформаційна та кібербезпека) | [INFORMATION_SECURITY_CYBERSECURITY.md](INFORMATION_SECURITY_CYBERSECURITY.md) | ISO/IEC 27001/27002, NIST CSF 2.0, NIST SP 800-53, SOC 2, IEC 62443, CIS Controls v8 | Заплановано (частково покрито [SECURITY_SPECIFICATION.md](../../security/SECURITY_SPECIFICATION.md)) |
| Personal Data Protection (Захист персональних даних) | [PERSONAL_DATA_PROTECTION.md](PERSONAL_DATA_PROTECTION.md) | GDPR, ePrivacy, EU-U.S. DPF, CCPA/CPRA, HIPAA, GLBA, COPPA, NIST Privacy Framework | Заплановано |

## 4. Спільна структура файлу домену

1. **Огляд домену** — регуляторний контекст, чому HW/SW комплаєнс критичний.
2. **Ключові стандарти** — таблиця стандартів з областю застосування.
3. **Модуль(і) комплаєнсу** — цільовий `module.id`, словники, майлстоуни, артефакти, правила за зразком [MODULE_CATALOG.md, розділ 3](../../architecture/MODULE_CATALOG.md#3-галузеві-модулі-комплаєнсу-engineering-compliance).
4. **Напрямок розширення** — які типізовані провайдери ядра ([ENTITY_CATALOG.md, розділ 2.2](../../architecture/ENTITY_CATALOG.md#22-типізовані-провайдери-розширень)) задіюються та які прогалини лишаються.
5. **Посилання**.

## 5. Правило додавання нового домену

Новий вертикальний домен додається копіюванням структури наявного файлу (найближчого за характером регулювання) та реєстрацією рядка в таблиці розділу 2. Новий наскрізний (горизонтальний) стандарт реєструється в таблиці розділу 3 та явно позначається в розділі 1 свого файлу («Чому цей стандарт не є галузевим доменом»). У обох випадках, за потреби, оновлюється [compliance/README.md, розділ 2](../README.md#2-мапування-стандартів).
