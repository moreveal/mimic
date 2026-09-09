# Аудит повторного использования стандартных алгоритмов

Дата: 2026-09-09. Основа: HEAD `93656f59f2b9a49a67efe20a399ef6513b63f81a` и рабочие изменения текущего исследования hydration. Это отдельный аудит: production-код, зависимости и frozen workload этим аудитом не изменялись. Родительская задача продолжает исправления независимо. Ссылки на upstream проверены во время аудита; выбор версии библиотеки должен фиксироваться отдельным lock/manifest.

Наиболее полезные замены — CSS selectors, CSS syntax, URL, encoding и streams. HTML parsing, криптографические примитивы и декомпрессия уже используют сторонние зрелые реализации. Для них первичная проблема — браузерная интеграция и Web API semantics, а не собственноручный алгоритм.

Обновление после завершения параллельных исправлений: Window selectors уже переведены на CSS-tree/DOMSelector preprocessing/css-select; Streams — на web-streams-polyfill4.3.0. Таблица LOC ниже сохраняет исходный снимок аудита, а не утверждает, что эти handwritten algorithms до сих пор являются основным production путем. В итоговом коде restricted native selector leaf helper занимает42строки, canonical selector adapter198, AST translator34; vendor algorithms не включены. Fetch/body integration130строк использует зрелые Streams. Старый Worker streams layer остается отдельным незавершенным потребителем. Эти миграции и их измеренные ограничения описаны в [selectors domain](selectors-domain-20260909/report.md), [Fetch/Streams](fetch-domain-20260909/report.md) и [performance report](../performance/report.md). Остальные рекомендации аудита пока не внедрены.

## Метод измерения

LOC ниже — физические строки выбранных областей, включая пустые строки, а не logical statements. `surface.js` содержит длинные строки, поэтому рядом приведен размер UTF-8 после нормализации CRLF в LF. Области явно перечислены: это не оценка всего транзитивного кода подсистемы. Generated bindings и код библиотек не включены. Файлы меняются параллельно; номера относятся к моменту аудита. Не следует суммировать пересекающиеся области. Машинные измерения сохранены отдельно в `.build/standards-audit-loc.json`.

| Подсистема | Измеренная handwritten область | LOC / байт |
|---|---|---:|
| HTML | `internal/dom/dom.go:32–82`, Parse→canonical import | 51 / 1344 |
| HTML fragments | `internal/dom/dom.go:627–683`, область SetInnerHTML, включая начало следующей функции | 57 / 1778 |
| Selectors | `internal/dom/selectors.go`, рабочая новая реализация целиком | 372 / 7475 |
| CSS syntax/cascade | `internal/webapi/surface.js:90–103`, включая JS selector helper | 14 / 8274 |
| URLSearchParams | `surface.js:248–251` | 4 / 2996 |
| URL object | `surface.js:267–270` | 4 / 2526 |
| URL host parsing/setters | `internal/browser/realm.go:1110–1173` | 64 / 1946 |
| UTF-8 encoder | `surface.js:59–61` | 3 / 1453 |
| bytes→string helper | `surface.js:253` | 1 / 181 |
| TextDecoder | `internal/webapi/dom_compatibility.js:157–176`, включая следующую matches registration | 20 / 1943 |
| Window streams | `surface.js:254–264` | 11 / 8308 |
| Worker streams | `internal/webapi/worker.js:32–35` | 4 / 2127 |
| WebCrypto classes | `surface.js:278–281`, без отдельного Crypto/getRandomValues | 4 / 3930 |
| WebCrypto host area | `realm.go:756–876`, включая random helpers | 121 / 3987 |
| Worker crypto host area | `internal/browser/worker.go:193–240` | 48 / 1837 |
| Compression dispatch | `internal/network/loader.go:328–360` | 33 / 1050 |
| Structured-clone-adjacent messaging | `internal/browser/message_port.go` целиком; это delivery/ownership, не clone algorithm | 84 / 2455 |

Отдельный полноценный handwritten structured-clone algorithm не найден поиском по production исходникам: сообщения проходят через export/import значений runtime, а некоторые JS каналы передают исходное значение. Поэтому «0 LOC полноценного алгоритма» не означает поддержку. Общую стоимость преобразования `engine.Value.Export/Value` аудит не приписывает structured cloning.

## Матрица решений

Сложность: Н — локальный адаптер; С — несколько потребителей/тестовых слоев; В — engine/FFI/lifetime/scheduler изменения. Gain и performance — прогноз, не результат benchmark. Проценты WPT не выдумываются: полный доменный baseline в этом аудите не запускался.

| Домен | Кандидат / язык / лицензия | Canonical-state adapter | Сложность | Ожидаемый gain | Ожидаемый performance |
|---|---|---|---|---|---|
| HTML | оставить x/net/html, Go BSD-3-Clause; резерв parse5, TS/JS MIT | Сейчас временное дерево→canonical IDs. parse5 имеет TreeAdapter | Н для import fixes; В для замены builder | Высокий от исправления import/script integration; неопределенный от замены parser | Сохранение Go ядра нейтрально; JS adapter с host call на токен может ухудшить |
| Selectors | css-select, TS/JS BSD-2-Clause; Servo selectors, Rust MPL-2.0 | css-select explicit Adapter; Servo Element trait; Cascadia прямого adapter не имеет | С JS / В Rust FFI | Высокий: grammar, combinators, functional pseudos | Compile cache полезен; per-node host crossings опасны; измерить |
| CSS syntax | tdewolff/parse, Go MIT; rust-cssparser, Rust MPL-2.0 | Да: tokens/AST не являются вторым DOM | С Go / В Rust FFI | Высокий для syntax; cascade/layout отдельно | Больше работы чем split, меньше повторного parsing при кеше |
| URL | nlnwa/whatwg-url, Go Apache-2.0; Ada, C++ MIT OR Apache-2.0 | Да: один URL record, net/url только transport representation | С Go / В C++ | Высокий: WHATWG parsing/setters/host normalization | Неизвестен в Mimic; batch getters/record важнее рекламных upstream ns |
| Encoding | x/text/encoding + htmlindex, Go BSD-3-Clause | Да, поток decoder state не дублирует DOM | С | Высокий для labels/legacy encodings | Go bulk decoding вероятно лучше JS массивов; вызовы на tiny chunks могут стоить дороже |
| Structured clone | V8 ValueSerializer/Deserializer, C++ BSD-3-Clause | Да: host-object delegate и transfer ownership table | В | Высокий для JS graphs; platform objects потребуют адаптеры | Ожидается меньше JSON-like преобразований; нужны реальные buffers/graphs профили |
| Streams | web-streams-polyfill, TS/JS MIT | Да: realm-local JS state + canonical network source | С–В | Очень высокий относительно упрощенных очередей | Больше bootstrap/Promise overhead; bounded queues могут улучшить память |
| Crypto | оставить Go crypto/*, Go BSD-3-Clause; x/crypto при нужных алгоритмах | Да: opaque key handles, semantic JS wrapper | С | Высокий при завершении WebCrypto contract; ядро уже зрелое | Bulk bytes и cache parsed keys перспективнее замены primitives |
| Compression | оставить Go compress/* BSD-3-Clause, andybalholm/brotli MIT, klauspost/compress BSD-3-Clause с отдельными license notices | Да: io.Reader→canonical response body | Н–С | Gain от content-coding chain и streaming wrapper | Потоковая обработка уменьшит peak RAM; сам codec менять без профиля не нужно |

## HTML tokenizer / tree builder

`dom.Parse` уже вызывает `golang.org/x/net/html.Parse`, fragments — `ParseFragment`, serialization — `html.Render`. Собственного tokenizer или adoption-agency algorithm здесь нет. [Документация x/net/html](https://pkg.go.dev/golang.org/x/net/html) описывает HTML5 parser и ограничения, в том числе отличие полного дерева от tokenizer.

Наблюдаемые gaps находятся в адаптере: основной Parse превращает comments/doctype в `other` и теряет данные; атрибуты складываются по одному Key без namespace; foreign tag names uppercased. Fragment context не передает namespace, импорт fragment elements не записывает Namespace. Полный HTML разбирается до исполнения script, поэтому простая замена parser не исправит parser-blocking script/document.write insertion point. Template content требует отдельного canonical fragment, а не второго независимого дерева.

Оставить x/net/html первым вариантом. Если WPT покажет именно tree-builder gaps, сравнить parse5: MIT, широко используемый parser; его [TreeAdapter](https://parse5.js.org/interfaces/parse5.TreeAdapter.html) позволяет направлять insert/adopt/template operations в canonical store. [Upstream parse5](https://github.com/inikulin/parse5) — самостоятельное ядро, не целый jsdom. Замена на jsdom означала бы второй DOM и здесь неприемлема. Вводить JS parser только после сравнения host crossings и teardown.

## CSS selector parser / matcher

В рабочем дереве появились `selectorParts`, `matchSelectorChain`, `matchCompound`; старый JS `cssSelectorMatch` остается в computed style path. Это уже два расходящихся matcher. По коду видны неполные CSS escapes, отсутствие широкой grammar validation/SyntaxError, namespaces, :scope/:has/:nth-* и dynamic-state coverage. Точный список меняется с текущими фикcами; наличие support пары GitHub selectors не является domain completion.

[css-select](https://github.com/fb55/css-select) предоставляет Adapter через getParent/getChildren/getAttributeValue/isTag и compilation. Это наиболее прямой путь без второго DOM. Не следует включать jQuery-only extensions в browser API. Результатные кеши отключить либо инвалидировать по mutation generation; AST кеш допустим. Dynamic pseudos должны читать реальное Page state. JS adapter поверх canonical proxies может сделать тысячи host calls; сравнить с compact read-only traversal bridge, который не хранит второй mutable DOM.

[Cascadia](https://github.com/andybalholm/cascadia), Go BSD-2-Clause, принимает `*html.Node`. Нельзя объявлять ее drop-in canonical adapter: нужен fork matcher доступа к узлам или временный snapshot. Snapshot на каждый query дорог и рискует устаревать, persistent mirror противоречит единственному источнику состояния. [Servo selectors в Stylo](https://github.com/servo/stylo), Rust MPL-2.0, архитектурно подходит через Element trait, но FFI/ownership/build сложнее. Приоритет — ограниченный css-select adapter spike, затем тесты, а не расширение собственного parser.

## CSS syntax

`parseCSS` делит текст по `;` и первому `:`, stylesheet rules извлекаются regex по `{}`. Строки, data URLs, escapes, вложенные функции/блоки, @media/@supports/@layer и nested rules нарушают такую модель; specificity также regex-based. Смена syntax parser не дает готового CSSOM, cascade, property validation или layout.

[tdewolff/parse/css](https://github.com/tdewolff/parse) — Go MIT, lexer/parser CSS Syntax Level 3, streaming grammar units. Предпочтителен первым из-за существующего Go host и отсутствия FFI. [rust-cssparser](https://github.com/servo/rust-cssparser) — MPL-2.0; намеренно не реализует последний слой property-specific grammar и selectors. Он интересен совместно с Servo selectors, но отдельно вводить Rust ради tokenization дороже. Parse immutable text→tokens/AST; stylesheet identity/owner/rule mutations оставить canonical. Кешировать по source/version, не пересчитывать все style elements на каждый computed property.

## URL

Здесь используется зрелый `net/url`, но это не браузерный WHATWG URL parser. `urlParts` по умолчанию использует documentURL как base даже когда JS URL constructor не получил base. `setURLPart` напрямую меняет поля net/url; default-port removal, special schemes/backslash, opaque paths, IPv4 legacy forms, IDNA, setter validation требуют отдельной семантики. `URLSearchParams` использует decodeURIComponent, который бросает исключение на malformed percent/UTF-8 вместо replacement behavior; iterator сейчас строится на snapshots.

[nlnwa/whatwg-url](https://github.com/nlnwa/whatwg-url), Apache-2.0, дает Go API WHATWG records и setters. В README заявлен релевантный WPT pass, но snapshot датирован 24 мая 2023: это не доказательство соответствия Chrome152. [Ada](https://github.com/ada-url/ada), C++ MIT/Apache-2.0, используется в крупных runtimes и заявляет полный набор specification tests; рассмотреть, если Go-кандидат провалит актуальные tests или профиль. Мигрировать JS URL, navigation resolution, workers, history, fetch и origin calculation согласованно. Сериализованный canonical record переводить в net/url на границе транспорта, не поддерживать два независимых URL состояния.

## Encoding

UTF-8 encoder написан вручную; bytesString сводит любой decode error к одному U+FFFD, теряя остальной текст. Рабочий TextDecoder реализует только UTF-8 и несколько aliases. Нет широкой label table, legacy/stateful encodings и полного BufferSource/stream error contract. UTF-16 input→USVString нужно проверять до Go conversion, чтобы не потерять lone-surrogate semantics.

В зависимостях уже есть `golang.org/x/text`. [htmlindex](https://pkg.go.dev/golang.org/x/text/encoding/htmlindex) сопоставляет web encoding labels; Decoder/Transformer дают incremental state. Нужен тонкий слой fatal/ignoreBOM/end-of-stream и проверка отличий replacement behavior. Само присутствие codec не означает готовый TextDecoder. HTML charset sniffing, CSS byte decoding и Fetch body decoding должны использовать согласованные правила, но их алгоритмы выбора encoding различаются. Приоритет — подключить существующие codecs, прекратить расширять ручные byte tables.

## Structured cloning

`message_port.go` занимается ownership/delivery, значения приходят через engine export и возвращаются runtime.Value. Это не подтверждает preservation cycles, identity sharing, Map/Set, Error, BigInt, typed-array offsets или detachment. Некоторые legacy JS MessagePort/BroadcastChannel paths захватывают исходный объект. `structuredClone` не имеет явной полноценной реализации в просмотренных production source.

[V8 ValueSerializer](https://v8.github.io/api/head/classv8_1_1ValueSerializer.html) — естественное ядро для V8 engine: graph serialization плюс delegate host objects и ArrayBuffer transfers. Оно не равно всему HTML structured clone: Blob/File/MessagePort/CryptoKey, cross-realm ownership, rejection DOM nodes, security и transfer transaction остаются Mimic. Нужны bindings gov8 и engine interface, отдельная стратегия для goja/QuickJS. Нельзя silently использовать один wire format для несовместимых engine versions.

[@ungap/structured-clone](https://github.com/ungap/structured-clone) — JS ISC, возможный ограниченный fallback для обычных JS graphs, но upstream явно сообщает, что transfer option игнорируется и многие platform objects не поддержаны. Он не заменяет полный browser contract. Не писать еще один JSON clone.

## Streams

Window и worker содержат разные маленькие handwritten реализации. В Window strategy практически игнорируется, нет BYOB, desiredSize фиксирован относительно единицы, pipeTo не реализует options/abort contract; writable writes не имеют полноценной serialization/backpressure machinery. Worker implementation еще проще. Это самостоятельный домен, а не набор методов для fetch.

[web-streams-polyfill](https://github.com/MattiasBuelens/web-streams-polyfill), MIT, имеет browser WPT test suite и документирует snapshot/spec exceptions. Подключить одну pinned realm-local сборку для Window/Worker, сохраняя Promise/microtask scheduling текущего Page event loop. Network источники и Blob/Response должны создавать ее streams, а не смешивать классы разных реализаций. Transferable streams и host cancellation не появляются автоматически. Measure startup cost, large-body peak memory и cancel/teardown retention вместе с throughput.

## Crypto

Алгоритмы уже делегированы Go `crypto/rand`, SHA, `crypto/rsa`, `crypto/x509`; зависимости также включают x/crypto. Ручной слой — WebCrypto normalization, key metadata и conversion. `SubtleCrypto` реализует digest, public SPKI RSA-OAEP import/encrypt, остальные основные операции возвращают NotSupportedError. В `cryptoBytes` ветка ArrayBuffer.isView использует `Uint8Array.from(data)`, что не эквивалентно чтению underlying bytes для всех view types.

Оставить [Go crypto](https://pkg.go.dev/crypto) и завершать coherent algorithm families: key formats/usages/extractability/error order/BufferSource, затем encrypt/decrypt либо sign/verify вместе. Opaque host key handle предпочтительнее повторного DER parsing и JS-visible key storage. Не добавлять OpenSSL/BoringSSL только ради уже работающих SHA/RSA: выигрыш не измерен, build/lifetime стоимость реальна. Выбор нового backend оправдан конкретным отсутствующим алгоритмом или профилем.

## Compression

Никакого handwritten deflate/brotli/zstd ядра нет. `decodeContent` dispatch использует стандартные Go gzip/zlib/flate, [andybalholm/brotli](https://github.com/andybalholm/brotli) и [klauspost/compress/zstd](https://github.com/klauspost/compress). MIT/BSD dependencies уже зафиксированы в go.mod. Для vendoring сохранить их отдельные notices.

Gaps адаптера: поддерживается одно значение Content-Encoding, цепочка `gzip, br` не разбирается; whole-body materialization; LimitReader может вернуть усеченное тело как успех на размере limit; лимиты zstd и других codecs различаются. CompressionStream/DecompressionStream surface не имеет явного algorithm implementation в просмотренных файлах. Использовать те же codecs через streams source/sink после завершения streams domain; тестировать flush, truncated data, checksum и cancellation. Не заменять codecs без свежего профиля.

## Coverage matrix и критерий возврата к production workload

Это inventory состояния, не выдуманный WPT score. Generated `chrome/152/generated/surface-catalog.json` отражает форму API, а не его семантику. Для каждого домена следует хранить pinned WPT revision, список выбранных тестов, Chrome152 binary SHA, Mimic build SHA, PASS/FAIL/TIMEOUT/SKIP по отдельности и известные exclusions. Отсутствующий счетчик обозначен «не измерен», а не нулем.

| Домен | Существующее доказательство | Требуемый WPT subset / surface | WPT Chrome↔Mimic baseline |
|---|---|---|---|
| HTML | Go DOM tests, отдельные hydration fixtures | [html/syntax/parsing](https://github.com/web-platform-tests/wpt/tree/master/html/syntax/parsing), DOMParser/innerHTML/templates, Document/Element/HTMLTemplateElement IDL | Не измерен в аудите |
| Selectors | Basic Go queries; рабочие независимые selector probes | dom/nodes selectors + css/selectors; ParentNode.querySelector(All), Element.matches/closest | Не измерен |
| CSS syntax | CSS/layout smoke paths | css/css-syntax + css/cssom; CSSStyleDeclaration, CSSStyleSheet, CSSRule | Не измерен |
| URL | Browser URL tests / generated catalog | [url](https://github.com/web-platform-tests/wpt/tree/master/url): constructor, setters, urltestdata, URLSearchParams, IDNA | Не измерен |
| Encoding | TextEncoder tests, рабочий UTF-8 streaming probe | encoding: textdecoder/textencoder/labels/streams IDL | Не измерен |
| Structured clone | MessagePort/frame/worker delivery tests | [structured-clone](https://github.com/web-platform-tests/wpt/tree/master/html/webappapis/structured-clone) + messaging transfer cases | Не измерен |
| Streams | Blob/pipeTo/TransformStream smoke | [streams](https://github.com/web-platform-tests/wpt/tree/master/streams): readable, writable, transform, BYOB, piping, transfer | Не измерен |
| Crypto | Random/digest/RSA-OAEP regressions | WebCryptoAPI algorithm families + IDL | Не измерен |
| Compression | Loader/transport tests | compression + fetch/content-encoding tests | Не измерен |

Для каждого запуска сверять форму API с frozen Chrome152 catalog и соответствующими Blink/WebIDL файлами на pinned Chromium revision, не с moving main. Данный аудит не скачивал полный Blink checkout и не заявляет проверки всех IDL members. Перед migration нужно сделать это для выбранного домена. API exposure count и behavioral coverage должны оставаться разными колонками.

Разумный минимум завершения домена: все выбранные стандартные positive/negative cases проходят; unsupported features явно внесены в exclusions; нет известных потерь canonical identity/ownership; mutation/cancel/error paths проверены; frozen fast gate и memory/teardown не регрессируют без объяснения. Production сайт после этого проверяет интеграцию, но не определяет grammar или исключения.

Приоритет: (1) завершить измерение текущей DOM/hydration boundary; (2) объединить selector consumers за стандартным parser/matcher adapter; (3) CSS syntax и URL; (4) streams+Fetch body/cancel; (5) encoding; (6) structured clone engine bridge. HTML import и crypto/compression wrappers исправлять по подтвержденным доменным failures, сохраняя уже используемые mature cores.
