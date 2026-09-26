-- 0018: перерахунок метрик здобутої цінності після погодження результату.
--
-- EV фази залежить від статусу approved артефактів, приписаних через
-- phase_deliverables (economics/gate.go, phase_progress CTE). Без цієї
-- підписки погодження результату не запускало жодного перерахунку, і
-- вимірювання EV лишалося valid, хоча його вхідні дані вже змінилися.
INSERT INTO core.trigger_subscriptions (trigger_key, event_key, subscriber_module) VALUES
    ('trigger.core.after_economics_changed', 'wp.approved', 'core');
