# Performance API: Chrome 152.0.7977.82 ↔ Mimic

Независимая ветка `codex/performance-api-chrome152`, исходная ревизия
`da4f93e873d508323832efafe6870d378d79d1c4`.

Реализован общий Go-owned timeline для каждого document/worker. JS-объекты
проецируют это состояние; ресурсы используют существующий network trace,
навигация — lifecycle документа, время — Page scheduler и Environment clocks.
Нового глобального runtime lock нет. Core остаётся platform-neutral.

Пакет покрывает:

- `performance`, `PerformanceEntry` hierarchy, brands/descriptors/receiver checks,
  каноническую identity, `getEntries*`, сортировку, `mark`/`measure`, detail cloning
  и clear operations;
- observers: режимы, очереди, buffering, `takeRecords`, порядок callback/microtask,
  resource buffer overflow и dropped entries;
- resource/navigation/legacy timing, redirects, TAO/CORS visibility, server timing,
  `timeOrigin`/`now`, iframe/worker ownership, reload и snapshot/restore;
- long tasks с microtasks и same-origin child attribution, trusted event timing,
  first-input, EventCounts и простую группировку interactions;
- `performance.memory`: согласованные immutable snapshots синтетической модели,
  profile-derived limit и allocation deltas через engine abstraction. Сырые
  Go/V8 heap totals и константы из Chrome capture не публикуются.

**Differential: 0 → 20 полностью совпавших групп из 23.** До изменений было
11 несовпадений и 12 capture errors; после — три несовпадения и ноль capture errors.
Проверены 28 форм интерфейсов. Это покрытие конкретных controlled observations,
а не утверждение о полной реализации всех exposed API.

Три сохранённых расхождения: Mimic заранее буферизует document body, иначе
завершает opaque Fetch и пока не умеет agent-cluster memory breakdown.
`measureUserAgentSpecificMemory` явно отклоняется с `NotSupportedError`.
Rendering-related типы представлены формой интерфейсов без производства
paint/layout/LCP/LoAF записей. Сложные input interactions, cross-origin long-task
attribution, BFCache/prerender и точная Chrome heap/GC policy остаются неполными.

Ранее полный обычный Go suite прошёл, но **финальный повтор упал**: в snapshot
navigation `responseEnd` остался нулевым на DOMContentLoaded, load и после загрузки.
Дефект открыт; expectations и skips ради него не менялись. Focused race, включая
четыре независимые Page, проходит. **Полный race не завершён:** browser package достиг десятиминутного
timeout в Goja child-navigation test; до timeout data race не зарегистрирован.
Шесть новых semantic skips явно соответствуют трём границам в ordinary/snapshot
режимах. Общие проверки manifest/audit по-прежнему падают и на чистом baseline
(278 идентичных audit findings). Все skips и неуспешные попытки сохранены.

Четыре frozen fast gates выполнили по 184 VALID executions. Однако финальный
React completion медленнее повторного baseline на **6,8%**, а throughput при
10/25 Page ниже на **9,2%/24,4%**. Разброс среды не объясняет всё снижение;
performance neutrality не установлена. Final private memory после teardown
и 250 мс recovery: static/React **151,29/162,00 MiB**.

Подробности и воспроизводимость: [полный отчёт](README.md),
[before/after differential](differential.json), [tests/race/skips](validation.json),
[все performance runs](performance.json).
