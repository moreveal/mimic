# Публичный контракт контекстов: история решений и оставшиеся границы

2026-09-25. Этот документ сохраняет исходный PoC-план и допущения, на которых
он был основан. Текущий контракт описан в
[environment-profiles.md](environment-profiles.md). После PoC генератор получил
положение окна и аудиорецепт (верхняя граница 3,612,980,639,528 комбинаций),
а bootstrap snapshots переиспользуются разными профилями с одинаковым графом
API. Пункты ниже, называющие их нереализованными, относятся к исходному PoC.
Следующая работа по реальным альтернативным GPU/font bundles требует новых
измеренных captures, потому что произвольные строки и независимый шум нарушают
согласованность наблюдений. Измерения RAM обновлены в
[профильном отчёте](performance/profile-context-final-20260925.md).
Первый отдельный этап компактного DOM storage —
[перестановка полей Node](performance/dom-node-layout-20260925.md) — измерен
отдельно. Блочный allocator [отклонён](performance/dom-node-blocks-20260925.md)
после нестабильного correctness gate; крупное разделение hot/cold полей остаётся
открытым.

## Принятые решения

- Один текущий контракт на релиз Mimic, без contractVersion и параллельных
  обработчиков старых API. Клиент читает Mimic.getVersion и адаптируется.
- createContext принимает генерацию или переносимую строку profile. Эта строка
  содержит проверяемые данные, а не ссылку на растущий серверный реестр.
- Полный JSON окружения доступен только для CDP import/export и диагностики;
  --profile, browser.Options.ProfileJSON и произвольный JSON в createContext убраны.
- Явный manual import позволяет задавать поля, проходит валидацию и возвращает
  предупреждение о пределах проверки. Генерация и manual никогда не обходят
  известные инварианты. Identity/graphics/fonts остаются на установленной базе,
  пока нет полноценных проверенных альтернативных recipes.
- Прокси и политика ресурсов задаются отдельно от профиля. Credentials не входят
  в профиль, его ID, export или диагностический ответ.
- Поддерживаются очередь из 100 заданий и 100 одновременно живых страниц;
  численные требования к RAM для них различаются.

Точный доступный синтаксис: [environment-profiles.md](environment-profiles.md).

## Что реализовано в PoC

Чистый генератор с crypto-random seed по умолчанию и детерминированным разрешением
заданного seed. Он меняет геометрию окна, тему и reduced motion, сохраняя измеренные
window insets и установленный device recipe. Переносимый профиль проверяется при
каждом restore; изменившийся результат на новой сборке означает ошибку, а не
молчаливую подмену. Hash не является подписью или security boundary.

CDP generate/import/export/create, явный manual mode, параметры proxy отдельно,
валидация policy до создания Context. Контексты с такими профилями блокируют
изменения identity/metrics/locale/media в общей Page-границе. Это ограничение PoC:
даже resize пока требует нового контекста. Обычный Target.createBrowserContext
сохраняет стандартные команды эмуляции, но не получает гарантий generated profile.

Внутренний Document/Normalize сохраняет поле schemaVersion только внутри CDP
import/export JSON. ResourcePolicy больше не принимает schemaVersion: контракт
определяется релизом Mimic без параллельного legacy handler.

## Проверки и ограничения

Тесты покрывают 100 фиксированных seed без повторов profileId, roundtrip,
случайный seed, некорректные параметры, ручные поля, отказ при изменённом export,
отсутствие Context при генерации/импорте, атомарный отказ create, proxy redaction,
worker fetch через proxy и блокировку CDP mutations.

Density probe создаёт 300 страниц в трёх сериях по 100 на локальном HTML с 200
ссылками, проверяет извлечение и профиль, затем закрывает Context. Для concurrency
100 barrier сохраняет все 100 страниц живыми до замера. Измеряются process RSS,
private bytes, Go heap и goroutines; принудительного GC нет. Это маленький fixture,
не оценка Wikipedia/React, не сравнение с Chrome и не полный soak test.

Результаты и ограничения измерений: [PoC report](performance/profile-context-poc.md).

Большое пространство fingerprint пока НЕ реализовано. На установленном экране
2560x1440, available 2560x1392, insets 8x147 генератор имеет максимум
1753 × 766 × 4 = 5,371,192 комбинации (~22.4 бит), а не 2^256.
Разные seed могут давать одинаковые profileId. GPU, fonts, canvas и audio от seed
не меняются. Уникальность конкретной fingerprint-сигнатуры сайта не гарантируется.

## Следующие отдельные изменения

1. По frozen Chrome 152 проверить surface/dependency matrix, геометрию и
   соответствие выбранных recipes. Расширять hardware/locale/graphics/fonts
   связанными моделями. Сначала измерить допустимое пространство, затем обещать
   его размер; ориентир 2^40 требует отдельной проверки достижимости.
2. Ужесточить полноту manual cross-field validation; дополнить диагностикой
   неподдержанных связей. Не трактовать warning как разрешение известных конфликтов.
3. Закрыть/документировать обходы через request interception и пользовательские
   скрипты: PoC не является sandbox и не гарантирует абсолютную согласованность.
   Разрешить согласованный resize как атомарную операцию, проверить auto-overrides
   Playwright/Puppeteer; сейчас для managed Context требуется raw CDP.
4. Расширить proxy matrix на WebSockets, HTTPS/SOCKS5 auth, redirects,
   cancellation и failed connections. Не обещать UDP/WebRTC routing.
5. Матрица RAM: empty Context, static/JS-heavy, один/разные профили, policy off/on,
   реальные разные локальные proxies, минимум 10 волн, crash/disconnect/stall.
   Принять ceilings по RAM и latency на обозначенном оборудовании.
6. Профилировать доли V8/Go/native/storage. Snapshot cache уже ограничен
   4 entries/32 MiB, но environment входит в ключ: разные профили теряют reuse.
   Не удалять его из ключа без доказательства корректного rebind. Не включать
   общий isolate для независимых Pages: в коде зафиксированы native teardown races.
7. Bounded helper/streaming results, timeout/cancellation/retry policy и устойчивый
   cleanup. Пример не должен создавать все контексты до допуска заданий.

## Отдельный этап: компактное хранение самых затратных структур

Дополнение по запросу пользователя: подготовить самостоятельное изменение
compact storage, отдельное от публичного API. Планируемый implementation commit:
`perf(storage): compact measured high-retention structures`. Не включать перепись
в контрактный PR и не выбирать структуру только по размеру её Go-заголовка.

Уже проверено по исходникам:

- `internal/dom/dom.go`: Node содержит много строк, две maps, slices, редкие
  document/script/template поля. Parent/Children уже используют int64 IDs,
  поэтому замена указателей на handles здесь не является новой оптимизацией.
- `nodeArena.nodes` — `map[int64]*Node`: это область ownership, а не плотный
  allocator. Возможный выигрыш — chunked indexed storage и hot/cold split.
- `internal/dom/arena.go`: документы одной Page объединяют ID domain; remapping
  делается до публикации. Это существенный invariant для новой реализации.
- `internal/layoutflat/state.go`: retained maps боксов, JSON request/response и
  defensive copies — кандидаты для измерения, не доказанный основной потребитель.
- `internal/state/clone.go`: рекурсивно копируются maps/slices environment,
  включая вложенные каталоги. Проверить multiplicity на разных профилях.
- V8/WebAPI graph, native layout/font allocations и response bodies учитывать
  отдельно: их нельзя убрать упаковкой Go Node.

Порядок работы:

1. Снять in-use и allocated memory profiles на свежем production build для
   DOM-heavy и JS-heavy workload, при 1/8/100 разных профилях. Для top owners
   записать количество объектов, shallow/retained bytes без двойного счёта,
   backing strings/maps/slices, время жизни и долю в process private bytes.
2. Выбрать один измеренно значимый owner. Сначала проверить перенос редких полей
   в side tables, enums/flags и chunked records. StringID использовать для
   действительно повторяемых names/namespace/property names; уникальные text,
   URLs, id/class и динамические values не интернировать автоматически.
3. Статические имена допускают общий immutable словарь. Динамический interner
   ограничить Page/arena lifetime и измерить его индекс: глобальная таблица
   случайных строк сама станет утечкой. Сохранить DOMString code units,
   включая TextJSON для непарных UTF-16 surrogates.
4. uint32 применять только внутри storage при явной проверке переполнения.
   Сохранить стабильные наружные CDP IDs; никаких wraparound и ABA при reuse.
   Chunked storage не должен оставлять указатели на переместившийся backing array.
   Сохранить attribute order/namespaces, adoption, template/shadow semantics,
   detached node identity, live collections и JS canonical wrappers.
5. Удобные accessors допустимы, но не создавать одновременно полную старую
   Node-копию на каждый compact record. Генератор accessors вводить только если
   несколько подтверждённых storage layouts оправдывают его стоимость.
6. Сравнить live/peak/post-close память и throughput/GC, затем focused semantics,
   race checks и существующий fast gate. Не принимать экономию sizeof при росте
   process memory или существенном замедлении обходов/мутаций. Записать результат
   и tradeoffs в performance report; только подтверждённую реализацию коммитить.

Оценка верхней границы до работы: если выбранная группа занимает долю f всей
памяти и уменьшается в k раз, общий коэффициент улучшения равен
`1 / (1 - f + f/k)`. Например, при f=20% и k=3 суммарный выигрыш около 1.15x.
Заявление о 2–5x по всей RAM допустимо только после измерения всего процесса.
