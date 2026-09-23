# DELMOS: домен комплаєнсу — Digital Media (Цифрові медіа, мовлення та стрімінг)

Дата: 2026-09-23. Статус: запланований напрямок розширення.
Ліцензія: Apache 2.0.
Контекст: [domains/README.md](README.md), [PERSONAL_DATA_PROTECTION.md](PERSONAL_DATA_PROTECTION.md).

---

## 1. Огляд домену

Домен цифрових медіа (телерадіомовлення, OTT-стрімінг, видавництво) поєднує технічні стандарти доставки контенту з юридичними вимогами захисту авторських прав (DRM), доступності для людей з інвалідністю та прозорості рекламних технологій (ad-tech). На відміну від решти доменів, тут комплаєнс частково є **добровільною галузевою сертифікацією** постачальника (наприклад, рівень стійкості DRM), а частково — **юридичним обов'язком** (доступність, реклама).

## 2. Ключові стандарти

| Стандарт / Програма | Область застосування |
| --- | --- |
| SMPTE (Society of Motion Picture and Television Engineers) | Формати медіафайлів, таймкоди, контроль якості мовлення |
| ATSC 3.0 / DVB | Стандарти цифрового телерадіомовлення (США / ЄС відповідно) |
| MPEG-DASH / HLS | Індустріальні стандарти адаптивної доставки стрімінгового контенту |
| Widevine / PlayReady / FairPlay (DRM) | Системи керування цифровими правами; рівні стійкості (Robustness Level L1–L3) |
| FCC CVAA (21st Century Communications and Video Accessibility Act) | Вимоги доступності відеоконтенту та інтерфейсів у США |
| EN 301 549 / WCAG 2.1–2.2 | Стандарти цифрової доступності в ЄС та загальносвітовий вебстандарт |
| IAB Transparency & Consent Framework (TCF) | Прозорість згоди користувача для рекламних технологій (перетинається з [PERSONAL_DATA_PROTECTION.md](PERSONAL_DATA_PROTECTION.md)) |

## 3. Модуль(і) комплаєнсу

### Плановий модуль: `compliance.media_drm_accessibility`
- **Внесок у Project Plan:** розділ `Content Protection & Accessibility Plan`.
- **Словники:** `DRM_SYSTEM` (Widevine, PlayReady, FairPlay), `ROBUSTNESS_LEVEL` (`L1`..`L3`), `WCAG_CONFORMANCE_LEVEL` (`A`, `AA`, `AAA`).
- **Майлстоуни:** `drm_certification_renewal`, `accessibility_conformance_audit`.
- **Артефакти:** `media:drm_license_agreement`, `media:accessibility_conformance_report` (VPAT — Voluntary Product Accessibility Template), `media:content_rating_certificate`.
- **Правила:** `rule.media.wcag_aa_required_for_public_sector_client`: релізний артефакт блокується, якщо цільовий клієнт — держсектор ЄС, а звіт відповідності нижче рівня `AA`.

## 4. Напрямок розширення

* Задіяні провайдери: `WPTypeProvider` (`drm_license_agreement`, `accessibility_conformance_report`), `DictionaryProvider` (реєстр DRM-систем і рівнів доступності), `MilestoneProvider` (переатестація DRM-ліцензії), `IntegrationConnection` (взаємодія із зовнішнім видавцем ліцензії DRM-вендора).
* Прогалина: ліцензійна угода DRM (Widevine/PlayReady/FairPlay) — це зовнішній контрактний документ вендора з обмеженим терміном дії; потрібен механізм нагадування про продовження, аналогічний періодичному скануванню вразливостей у [FINANCIAL.md](FINANCIAL.md).

## 5. Посилання

* [PERSONAL_DATA_PROTECTION.md](PERSONAL_DATA_PROTECTION.md) — прозорість згоди для рекламних технологій.
* [STANDARD_MAPPING_TEMPLATE.md](../STANDARD_MAPPING_TEMPLATE.md) — шаблон формального мапування.
