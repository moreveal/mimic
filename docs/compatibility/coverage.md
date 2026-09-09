# Матрица совместимости по подсистемам

Состояние исследования: 2026-09-09. Production workload задает приоритет; стандартный домен и независимые тесты определяют семантику. Эта таблица отражает конкретные выбранные subsets. Процент всей web platform не вычисляется. Форму API из generated Blink/WebIDL catalog нельзя приравнивать к выполненным behavioral assertions.

| Домен | Измеренный subset / результат Mimic | Chrome152 | Статус и следующий предел |
|---|---|---|---|
| Modules/import.meta | Независимый import.meta.url regression и module failure reporting исправлены | Независимый oracle совпадает | Pending TLA/rejection reporting отдельно от microtask pumping; [первое исследование](repository-hydration-20260909/) |
| Selectors | 76PASS/5FAIL из81 зарегистрированного; 35/36 generic probes | 2054PASS/1FAIL из2055 | Нельзя делить разные denominators;1975-case suite blocked до регистрации через document.implementation. Mature parsing/matching внедрен; [отчет](selectors-domain-20260909/report.md) |
| MutationObserver | 101 observed PASS/129; только61 PASS из успешно завершенных harness |128PASS/129| Два harness неполны; Range/Attr/namespaces/CDATA остаются. Observed passes не считать authoritative; [отчет](mutations-reactions-20260909.md) |
| Custom Elements |102PASS/189, все8 harness завершились; generic reactions12/12|184PASS/189| Cross-document registries, customized built-ins, document.write, namespace/Attr gaps; [отчет](mutations-reactions-20260909.md) |
| DOM Nodes/CharacterData | Embedded checkpoint259PASS/90FAIL из349;16/16 harness OK; CharacterData136/136 |349/349| Specialized interface prototypes, document.implementation/XML fixtures, iterator identity и mutation exception precedence остаются. [Receipt](nodes-domain-20260909/final-embedded.json) |
| Templates | Embedded independent12/12; unmodified WPT1PASS/231FAIL из232,5/5 harness OK |Independent12/12; WPT232/232|227 failures требуют createHTMLDocument;3 fixture failures;1 реальный inert ownerDocument gap. Canonical content/clone проверены отдельно; [отчет](templates-domain-20260909.md) |
| Forms values |Embedded71/71 assertions в5 WPT files; prior independent6/6 groups |71/71| Один test_driver case исключен на обеих сторонах. Embedded binary/PID/source hashes проверены; Dirty/default/input/textarea/select/reset layer; [отчет](forms-domain-20260909/report.md) |
| Fetch Body/Request/Response |Iteration8:103PASS/3FAIL из106 зарегистрированных; expected113. Generic13/14, onabort исправлен в source после сборки |99PASS/14FAIL из113|7 unregistered из-за FormData; остальное3FAIL также formData consumer. Current WPT и frozen Chrome расходятся в14assertions; [отчет](fetch-domain-20260909/report.md) |
| Streams |Mature polyfill4.3.0: BYOB/HWM/write serialization3/3 generic; Response stream WPT выше |Generic3/3| Worker, transfer и полный Streams WPT не измерены. Goja generated structuredClone stub обнаружен как integration blocker; общие Go tests и финальный fast gate PASS |
| Shadow DOM export |Independent live Chrome/Mimic совпадают; exported HTML→Chrome23/23 relative-time state + named slots/open/closed/nested regression PASS|Pinned Chrome152 restores declarative shadow trees|Immutable canonical-ID projection; adopted stylesheets/manual slots остаются. [Отчет](shadow-snapshot-20260909/report.md) |
| HTML parsing |x/net/html уже используется; import layer audited |Полный parser corpus не запускался| Comments/doctype/namespaces/script insertion point — отдельные домены, не повод писать собственный tokenizer |
| CSS syntax/cascade |Regex/split handwritten layer audited; selector consumer уже переведен |Semantic denominator не измерен| Кандидат tdewolff/parse; full CSSOM/cascade не следует из parser adoption |
| URL |net/url + handwritten Web API; известные WHATWG расхождения |Полный URL corpus не запускался| Canonical WHATWG record migration nlnwa/whatwg-url или Ada после spike |
| Encoding |UTF-8 limited implementation; отдельный probe |Полный Encoding corpus не запускался| x/text codecs уже доступны; label/fatal/BOM/legacy/USVString domain требуется |
| Structured clone |Нет полноценного graph/transfer contract |Не измерен| V8 ValueSerializer candidate; fake generated callable нельзя использовать как capability evidence |
| Crypto |Go crypto primitives; digest/public RSA-OAEP wrapper tests|Полный WebCrypto corpus не запускался|Algorithms уже mature; key usages/formats/extractability/BufferSource layer ограничен |
| Compression |Go gzip/zlib/flate + Brotli/Zstd mature codecs |Полный compression corpus не запускался|Content-Encoding chains, bounded body errors, streaming adapters требуют corpus; менять codecs без профиля не нужно |

Известная ошибка измерения: прежние DOM.getOuterHTML snapshots брали original source, поэтому непригодны как post-hydration DOM evidence. Runtime actual DOM и Mimic.captureSnapshot читают canonical state; CDP исправлен и проверен regression tests. Skeleton count0 недостаточен: React error fallback тоже удаляет skeletons.

Для нового checkpoint обязательны: pinned source revision/hash; живой endpoint→PID→executable verification; binary SHA; режим (собранный код / injected source / mocked host); registered/expected test counts; успешность harness; PASS/FAIL/TIMEOUT/NOTRUN и блок регистрации отдельно. При изменении диапазона tests сохранять прежний denominator. Финальную страницу проверять по заполненному UI и отсутствию error fallback, не наличию API или исчезнувшей skeleton CSS class.

Отдельный read-only аудит libraries, лицензий, LOC, adapter feasibility и ожидаемых затрат: [standards-reuse-audit.md](standards-reuse-audit.md). Его performance прогнозы не являются benchmark результатами.


