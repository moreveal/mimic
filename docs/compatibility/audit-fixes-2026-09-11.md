# Первый пакет исправлений широкого аудита — 2026-09-11

Работа выполнена в исходной ветке `main`, поверх `3faacc6`. Исходная
[таблица аудита](audit-2026-09-11.md) сохраняет измерения до исправлений;
это дополнение отражает изменения, а не переопределяет старые результаты.

| Группа | Изменение | Проверка |
|---|---|---|
| A02: часы evaluation | Внешняя JS evaluation выполняется как отдельный turn планировщика с живыми часами до завершения microtask checkpoint; очередные timers не исполняются внутри него | Та же CDP-проба: вместо performance/wall delta 0/0 — 5.4/6 и 6.2/7 мс при awaitPromise false/true. Сумма вычислений прежняя. Тесты body, microtasks, исключения и child execution context |
| A03: redirect context | Одна неизменяемая история URL питает SameSite cookie access, Sec-Fetch-Site, Origin, credentials и CORS taint | `samesite-navigation`: **7 → 0** diff в свежем Chrome 152 comparison; GET302, POST303, POST307 |
| A04: Fetch CORS | Общие проверки для Window/Worker: same-origin mode, ACAO/ACAC, OPTIONS preflight, exposed headers, opaque response | Исходная CORS-проба теперь совпадает с Chrome; дополнительные allowed/denied/credential/opaque/empty-header/preflight tests |
| A05: history state | Приватная копия хранимого графа отделена от cached history.state; V8 native serializer, ограниченный Goja fallback; неуспешная копия не меняет историю | Исходная history-проба совпадает; cycles, shared references, builtins, getter exception identity, atomic failure, traversal и cold/warm bootstrap tests |
| A01: native lifecycle | Добавлены serial regression и opt-in concurrent reproducer/controls; production snapshot-код не изменён | **Открыто.** Есть intermittent native failure, но точная причина не установлена. [Отдельное расследование](snapshot-lifecycle-audit-2026-09-11.md) |

Измеренные длительности clock-пробы подтверждают ход часов, **не** преимущество
Mimic в скорости над Chrome: режимы исполнения/инструментации здесь не являются
контролируемым performance benchmark.

## Повторное широкое сравнение

Сырые артефакты: `compatibility/private-captures/audit-fixes-20260911/`.
Первый подкаталог `comparison/` сохраняет результат до исправления redirect
chain; `comparison-request-chain/` — результат после него; `comparison-final/`
повторяет оба набора на окончательном коде, включая явный unsupported отказ
для History Blob/File. SHA256 финальной сборки:
`4E9AAB43073090CF19A1FE9AAEFD650D8E61388D51895E85AF8389344B88B125`.
Каждый запуск
создаёт собственные процессы и профиль Chrome, сохраняет аргументы запуска и
завершает только эти процессы. Chrome — 152.0.7977.82, headful.

* Общий неизменённый corpus: **12/15** сценариев полностью совпадают вместо
  **11/15**; diff записей **41 вместо 48**. Остались network headers (12),
  surface (7), observations (22); новых отличающихся сценариев нет.
* State relations: **4 diff вместо 12**. History и CORS совпали. Остались
  borrowed HTMLAllCollection method, два наблюдения remote prototype mutation
  и srcdoc navigation. Это три открытые группы, не четыре независимых причины.
* Clock-проба сохранена отдельно в `clock-await-false/` и `clock-await-true/`
  с исходным expression SHA256 и raw CDP ответом.

## Проверки и границы

Полный `internal/browser` suite прошёл (339.660 s) на объединённых clock,
history и CORS изменениях до последнего redirect-chain дополнения. Полные
`internal/cdp`, `internal/network`, `internal/scheduler` прошли; engine Goja/V8
проверены для serializer изменений. Целевые race-тесты часов/history/CORS и
network прошли. Последнее request-chain дополнение проверено полным network
suite, focused race-тестами и повторным Chrome corpus. На финальном дереве
снова прошли engine Goja/V8, CDP, network, scheduler, выбранные browser-тесты
(23.523 s до последнего Blob/File guard; отдельные serialization/clock tests
после guard — 2.203 s) и совместный focused race-прогон (browser 11.280 s).
Warm history fixture явно проверяет факт восстановления snapshot.
В двух performance
fixtures и одном redirect fixture добавлены разрешающие CORS headers:
проверяемые assertions сохранены.

Полный browser запуск с лимитом 180 s остановился по общему лимиту, без
установленного зависшего теста; повтор с достаточным лимитом прошёл. Первый
race-прогон clock-теста исчерпал слишком короткий общий deadline во время
Goja child initialization. Deadline увеличен с 3 до 15 s; условия проверки
движения часов не ослаблены, повтор прошёл.

History остаётся ограниченным представлением realm-owned graph, не форматом
сохранения полной browser session; см. [границы сериализации](history-state.md).
Fetch не получает streaming body, preflight cache или полную поддержку
XHR/module CORS в этом изменении. Перечисленные в исходном аудите WebIDL
receiver/constructibility, realm ownership, CSS/layout и missing API группы
этим пакетом не объявляются исправленными.
