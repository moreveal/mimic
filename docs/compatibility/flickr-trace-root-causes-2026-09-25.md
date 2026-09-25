# Flickr: аудит первопричин ошибок Mimic

> **Дополнение после исправлений, 25 сентября 2026.** Утверждение ниже об
> отсутствии подтверждённого токена относится только к исходным автоматическим
> захватам. Пользователь показал непустой `cf-turnstile-response` в DevTools
> своего обычного Chrome на той же вкладке, в том числе новый токен после
> обновления. Проверка через расширение Computer Use показывала пустое значение
> и не отражала это состояние DevTools. Успех обычного Chrome подтверждён
> снимками пользователя; точная сеть этого сеанса расширением не доступна.
> Frozen Chrome 152 и Mimic в отдельных автоматических сеансах за 90 секунд
> токен не получили. Это разные окружения, и их исход нельзя переносить на
> обычный Chrome пользователя.

Дата: 25 сентября 2026. Проверенный commit: `059a25dc8094a9860959f3f739d6424de561c6cf`.
Эталон: frozen Chrome **152.0.7977.82**, Windows, Playwright/CDP.

**Изменений в реализацию нет.** Созданы только диагностические материалы и этот отчёт.

## Результат

Четыре исходные необработанные ошибки страницы объясняются несколькими независимыми дефектами браузерной семантики. Это не четыре ошибки Turnstile. Сбои затрагивают Weglot, Webflow и загрузку менеджера согласий. При сокращении примеров обнаружены дополнительные дефекты тех же границ.

Основные причины: неверная сериализация URL; потеря канонической идентичности DOM при переходе между realm; проверка DOM через переопределяемый getter; локальное хранение состояния Event; вызов публичного `setAttribute` из отражаемых свойств скрипта; выбор случайного, иногда ещё не инициализированного realm для Trusted Types.

| Наблюдение после загрузки Flickr | Mimic | Chrome 152 |
|---|---|---|
| Главная страница | HTTP 200, readyState complete | HTTP 200, readyState complete |
| Необработанные JS-ошибки в базовом захвате | 4 причины/места падения, 8 записей exception | Нет |
| Weglot | undefined | object |
| turnstile | undefined | object |
| Поле cf-turnstile-response | Отсутствует | Есть, длина значения 0 |
| Сетевые loadingFailed | Нет в базовом захвате | DNS и ORB, подробнее ниже |

**Успешное получение токена не подтверждено ни для одного браузера.** В Chrome загружается Turnstile, но в контрольном захвате есть два `ERR_NAME_NOT_RESOLVED` для `brunhild.challenges.cloudflare.com` и сообщения Turnstile `600010`. Отдельно TrustArc блокируется `ERR_BLOCKED_BY_ORB`. Точная причина DNS-сбоя, связь с решением сервиса и политика ORB в этом аудите не устанавливались. Исправление найденных ошибок Mimic само по себе не доказывает прохождение проверки.

## Метод и границы проверки

1. Выполнены независимые базовые захваты Flickr в Mimic и frozen Chrome; сохранены события CDP, тела документов/скриптов, состояние страницы и внутренняя трасса Mimic.
2. Для извлечения стека добавлены диагностические обёртки к захваченным скриптам. Такие прогоны явно отделены от базового: обёртки могут влиять на выполнение.
3. Для каждой основной причины выполнены сокращённые локальные проверки в обоих браузерах и сопоставление с исходниками.
4. Проверены все категории ошибок, unsupported и semantic-missing в базовой трассе. Это аудит данного выполнения и связанных сокращённых примеров, а не заявление о проверке всех Web API или всех ветвей сайта.

Захват ожидал 15 секунд после DOMContentLoaded. Отсутствие токена относится к этому наблюдению. Полные внешние ресурсы двух живых загрузок могут отличаться; выводы о семантике основаны на локальных одинаковых примерах.

## Что именно есть в трассе

В `.build/flickr-audit/baseline-mimic/trace.json`:

| Категория | Число | Интерпретация |
|---|---:|---|
| exception | 8 | Четыре ошибки, каждая записана дважды |
| error | 4 | Ошибки выполнения parser script tasks |
| semantic-missing | 83 | 82 CSS.fontSizeResolution и 1 HTMLAnchorElement.type |
| unsupported | 38 | В основном чтения отсутствующих свойств и проверки библиотек; не 38 недостающих API |

Исходные падения: `NotFoundError` при загрузке Weglot; последующий вызов `Weglot.initialize` при undefined; `Illegal invocation` при старте Webflow; переполнение стека в цепочке SOE/Osano. В инструментированных прогонах дополнительно пойман альтернативный ранний сбой Trusted Types.

## Причинная цепочка страницы

```text
URL без завершающего / → Weglot решает, что требуется URL polyfill
  → создание/настройка/вставка script через обёртки Osano
    → Trusted Types может выбрать неинициализированный isolated world → TypeError
    либо
    → заимствованный parentNode из iframe меняет канонические DOM wrappers
      → prepareInsertion сравнивает wrappers → ложный NotFoundError
  → Weglot не установлен → Weglot.initialize падает вторично

Повреждение wrapper BODY + локальное состояние Event
  → Webflow dispatchEvent(ix2-animation-started) → Illegal invocation

SOE выставляет script.async → публичный setAttribute → обёртка Osano
  → снова script.async → рекурсия → RangeError
```

Ни `Weglot.initialize`, ни отсутствие поля Turnstile не являются достаточным диагнозом сами по себе. В базовом Mimic инициализация страницы уже прервана до наблюдаемой в Chrome загрузки Turnstile.

## Подтверждённые дефекты

### 1. HTTP URL с пустым path сериализуется без `/`

**Где:** `internal/browser/url_host.go`, функции resolveURL/urlParts/setURLPart; `internal/webapi/fetch_primitives.js`, реализация URL.

Host использует `net/url` и возвращает `u.String()`/`u.EscapedPath()` без нормализации пустого пути special URL по наблюдаемому поведению Chrome. JS сохраняет полученное значение.

`new URL('http://weglot.com').href`: Mimic `http://weglot.com`, Chrome `http://weglot.com/`. После изменения search разница сохраняется. URLSearchParams в этом сокращённом примере работает в обоих браузерах.

В диагностике живого Weglot список недостающих возможностей равен `["URL"]`, после чего выбирается URL-polyfill. Значит, проблема не в отсутствии конструктора URL, а в его поведении. Проверка: `weglot-url-feature`.

### 2. Возврат DOM из другого realm перезаписывает канонический wrapper

**Где:** `internal/webapi/surface.js`: cachedDOMParent около 1581; Node.parentNode около 2979; wrap около 4539; ветка nodeId в unwrapCrossRealm около 7356.

Заимствованный getter parentNode выполняет wrap в своём realm. При возврате узла unwrapCrossRealm записывает proxy в `elementWrappers` по существующему nodeId, заменяя локальный канонический объект.

После `iframe.contentWindow.Node.prototype.parentNode` getter, вызванного на локальном ребёнке, в Mimic нарушаются сразу три равенства: возвращённый родитель с исходным, обычный child.parentNode с исходным, getElementById с исходным. В Chrome все сохраняются. Последующий insertBefore в Mimic падает.

Osano действительно заимствует Node.parentNode из своего iframe. В захвате Weglot перед вставкой оба родителя имеют имя HEAD, но `sameParent=false`, `sameHead=false`, при этом исходный первый ребёнок совпадает. Это ошибка идентичности, а не фактическое отсутствие reference node. Проверка: `borrowed-parent-canonical-identity`.

### 3. Проверка insertBefore зависит от публичного parentNode

**Где:** `internal/webapi/surface.js:3132`, prepareInsertion, сравнение около 3151.

В пути вставки resource/script проверяется `before.parentNode !== parent`. Это чтение переопределяемого JS-свойства вместо проверки членства в авторитетном DOM по идентификаторам.

Достаточно определить у настоящего ребёнка собственный getter parentNode, возвращающий null: вставка script перед ним в Mimic даёт NotFoundError, Chrome успешно вставляет. Этот самостоятельный дефект также усиливает проблему №2. Проверка: `insert-before-overridden-parent`.

### 4. dispatchEvent не принимает Event из другого realm

**Где:** `internal/webapi/events_compatibility.js:19`, stateOf; dispatchEventCore около 354; dispatchEvent около 480.

Состояние ищется через realm-local `eventSlots.get(event)`. Корректный Event из другого realm не имеет записи в этой таблице и отвергается как Illegal invocation.

Три проверки — foreign event/local target, local event/foreign target, borrowed dispatch/main event — падают в Mimic и успешны в Chrome.

В живом захвате Webflow падает отправка **ix2-animation-started** на **BODY**: событие принадлежит локальному realm, target уже не проходит локальное `instanceof Node`. Стек приходит в stateOf через чужой Proxy.dispatchEvent. Связь с №2 подтверждена наблюдением wrapper; это сбой запуска анимаций Webflow, а не вызов Turnstile API.

### 5. async/defer вызывают переопределяемый setAttribute и создают рекурсию

**Где:** `internal/webapi/surface.js:4103` и `:4112`, setters HTMLScriptElement.async/defer.

Setter вызывает `this.setAttribute(...)`. Osano оборачивает setAttribute и отражаемые свойства: установка атрибута async/defer вызывает соответствующее свойство. Возникает цикл между native-looking setter Mimic и библиотечной обёрткой.

Минимальная проверка с подменённым setAttribute: Mimic вызывает hook один раз и не выставляет атрибут; Chrome вообще не вызывает hook и выставляет свойство/атрибут. Проверка с ограничителем рекурсии: Mimic шесть вызовов до искусственного guard, Chrome ноль. Полный стек SOE повторяет эту цепочку до RangeError.

Падает setupConsentManager при установке script.async, до завершения добавления менеджера согласий. Проверки: `script-async-public-hook`, `script-defer-public-hook`, обе `*-recursion`.

### 6. Trusted Types выбирает произвольный realm документа, включая lazy world

**Где:** `internal/browser/trusted_types.go:95`, trustedTypesOwner; `internal/browser/debugger_worlds.go`, isolatedWorld; `internal/browser/deferred_runtime.go`; `internal/webapi/trusted_types.js:387`.

Владелец выбирается первым совпадением в Go map realmOwners по root/arena. Разные isolated worlds одного документа удовлетворяют условию. Lazy world может ещё не установить trustedTypesEnforcer. Пустое значение кодируется как cross-realm descriptor, JS проверяет truthiness descriptor и пытается вызвать результат unwrap, равный undefined.

Отсюда `TypeError: unwrapCrossRealm(...) is not a function`. Ошибка поймана в живом Weglot на пути trustedConvert → trustedAttributeValue → Proxy.setAttribute.

Локальный цикл borrowed setAttribute для src: 83 успеха и 17 ошибок из 100. Контролируемый эксперимент с шестью lazy isolated worlds: 3 успеха и 97 ошибок; после инициализации всех соответствующих worlds — 100 успехов. Chrome: 100 успехов на обоих этапах. Числа описывают конкретный запуск, не фиксированную вероятность: порядок map не гарантирован.

### 7. Заимствованный Document.createElement создаёт объект в realm метода

**Где:** `internal/webapi/document_compatibility.js:155–187`; `internal/webapi/surface.js`, Document.createElement около 4925.

Обёртка сначала вызывает original, создающий объект через локальный host/wrap, затем adopt меняет принадлежность документа. Это не исправляет prototype realm уже созданного объекта.

`iframe.contentWindow.Document.prototype.createElement.call(document, 'div')`: ownerDocument и вставка корректны, но prototype в Mimic принадлежит iframe. Chrome возвращает prototype основного документа. Проверка: `borrowed-create-element`.

Найдено при сокращении проблемы realm; отдельное падение живого Flickr этим пунктом не доказано. Общая обёртка охватывает и другие create-методы, но результат приведён только для измеренного createElement.

### 8. getElementsByTagName проверяет realm через instanceof

**Где:** `internal/webapi/document_compatibility.js:256–259`.

Проверка `this instanceof Document/Element` относительно realm функции отвергает корректный Document другого realm. Заимствованный getElementsByTagName.call(mainDocument, 'head') даёт Illegal invocation в Mimic; Chrome возвращает правильный head. Проверка: `borrowed-tags-head`.

Это соседний подтверждённый дефект, найденный локальным тестом. Его нельзя смешивать с живым Illegal invocation Webflow: у того другой стек, описанный в №4.

### 9. JS-resolver размера шрифта не разрешает viewport units

**Где:** `internal/webapi/css_font_metrics.js:18`, cssResolveLength; cssComputedFontSize около 140; `internal/webapi/css_computed_values.js:387`.

Resolver умеет абсолютные единицы, em/rem/% и часть calc, но не vw. Формула корня Flickr `calc(0.017733564013841074rem + 1.3840830449826986vw)` возвращает null, создавая 82 semantic-missing записи. Зависимые пути местами подставляют 16 px.

На начальном пустом документе эта формула и чистое vw дают 16 px; Chrome при ширине 1280 даёт соответственно 18 px и корректное viewport-значение. На загруженном HTTP-документе приоритет получает native producer — поэтому там ошибка выглядит иначе, см. №10. Это два пути вычисления наблюдаемого размера шрифта с разными результатами.

### 10. Native font-size наследует квантование Stylo, отличающееся от Chrome

**Где:** `internal/webapi/surface.js:2092`, blitzStyleValue; `internal/layoutblitz/native/src/lib.rs`, style/style_batch; pinned blitz `19a72d4a4163038d748603fbcf709d38bfa6a998`, resolved_style.rs; dependency `stylo-0.21.0/values/specified/length.rs:947`, `font.rs:1023`.

Stylo предварительно усекает viewport-результат в app units, а затем `FontSize::quantize_font_size` отбрасывает 14 бит мантиссы f32, оставляя 10 бит точности. Mimic возвращает сериализованный результат этого producer как computed style.

При width=1280 на загруженном HTTP-документе:

| Вход font-size | Mimic | Chrome |
|---|---|---|
| 18px | 18px | 18px |
| 1.40625vw | 18px | 18px |
| 1.3840830449826986vw | 17.6875px | 17.7163px |
| 0.017733564013841074rem | 0.283691px | 0.283737px |
| Формула Flickr из №9 | 17.9688px | 18px |
| calc из двух px-слагаемых с суммой 18 | 18px | 18px |

Проверки сохранены в font-matrix-http.json. Это несовместимость численного результата, а не доказанная причина падения JS или отказа Turnstile.

### 11. Сериализация заданного font-size сохраняет исходные числа/calc

**Где:** `internal/webapi/webkit_css.js:73`, normalizeCSSValue; `internal/webapi/surface.js`, CSSStyleDeclaration.setProperty/getPropertyValue.

Для проверенных значений font-size путь нормализации возвращает исходную строку без специализированного парсера/сериализатора. Поэтому style.fontSize сохраняет длинные дроби, а `calc(0.2837370242214572px + 17.716262975778544px)` остаётся таким же. Chrome возвращает нормализованные числа (`1.38408vw`, `0.0177336rem`) и `calc(18px)` соответственно.

Это самостоятельное наблюдаемое отличие CSSOM, подтверждённое обеими font-matrix, без доказанной связи с аварией сайта.

### 12. HTMLAnchorElement.type существует как заглушка

**Где:** `internal/webapi/surface.js:4377`, HTMLAnchorElement; generic accessor fallback около 8912/8931.

У класса нет отражаемой реализации type; установлен generic getter/setter, вызывающий semanticMissing. До установки атрибута чтение даёт undefined вместо пустой строки. После setAttribute('type','text/html') свойство всё ещё undefined; запись свойства не меняет атрибут. Chrome корректно отражает обе стороны. В трассе одна запись HTMLAnchorElement.type. Проверка: `anchor-type`.

### 13. Parser-script ошибки обходят Window error и теряют структуру в CDP

**Где:** `internal/browser/realm.go`, evaluateClassicScript около 457 и Evaluate около 554; `internal/browser/page.go` около 792; `internal/cdp/server.go`, преобразование trace.Exception; `internal/engine/v8/spike.go`, exceptionError; `internal/webapi/window_errors.js`.

Скриптовый путь не вызывает имеющийся reportWindowException. Ошибка отдельно записывается как evaluation и script, а CDP публикует обе записи. Исключение предварительно сведено к строке: теряются объект Error, stackTrace и полноценные координаты/context.

На одном inline `throw new Error('audit-uncaught-script')`:

- Mimic: обработчик window.error не вызван; два Runtime.exceptionThrown; line/column равны 0; нет structured stack/exception.
- Chrome: один window.error и один Runtime.exceptionThrown с объектом, стеком и фактической колонкой.

Это объясняет пустой массив ошибок диагностического init-script при реально упавшей странице и удвоение исходных четырёх исключений. Проверка: `parserScriptErrorReporting`. Отсутствующий DOMException.stack сюда не относится: он undefined и в Chrome, и в Mimic.

### 14. Явная навигация на about:blank не реализована в данном пути

**Где:** `internal/browser/page.go:490–496`, beginNavigationRequestWithCommit.

При подготовке дополнительной CSS-проверки `page.goto('about:blank')` вернул `unsupported navigation scheme "about"`. Причина явная: entry point принимает только http/https. Новый пустой Page при этом существует и позволяет выполнять JS; создание начального документа и явная навигация используют разные пути.

Это отдельно найденная неподдерживаемая граница навигации, не причина исходных ошибок Flickr. Для CSS-измерения использованы новый Page и локальный HTTP-документ, без изменения реализации.

## Что не следует считать найденными недостающими API

В unsupported попали documentMode, Document.namespaceURI, globalPrivacyControl, script.node, document.document/type/uniqueID, div.type и динамические jQuery/sizzle-поля. Проверенные фиксированные свойства возвращают undefined и в frozen Chrome. Отсутствие библиотечного expando до его установки также не доказывает отсутствующий Web API.

Следовательно, заполнять все 38 записей заглушками было бы неверно. Реальный HTMLAnchorElement.type выделен отдельно, поскольку его поведение отличается от Chrome. Остальные записи оставлены как диагностические наблюдения, без объявления неподтверждённых дефектов.

## Приоритеты для будущего исправления

Это порядок рассмотрения, а не выполненные изменения:

1. Каноническая DOM-идентичность и выбор realm-владельца, включая Trusted Types.
2. DOM-проверки по авторитетному состоянию и передача Event между realm.
3. Отражаемые свойства script без вызова пользовательских override; URL serialization.
4. Pipeline ошибок: window.error, однократное CDP-событие, сохранение Error/stack/context.
5. Остальные заимствованные DOM-методы, CSS-resolvers/serialization и anchor.type.
6. Отдельно — навигационная граница about:blank.

Проверять будущие изменения нужно сокращёнными примерами против frozen Chrome и повторным немодифицированным захватом Flickr. Получение Turnstile-токена должно оставаться отдельным результатом проверки; наличие API, поля или HTTP 200 его не заменяет.

## Доказательства и воспроизведение

Все пути ниже относительно корня репозитория. `.build` — локальные игнорируемые материалы, не часть commit отчёта. Полные захваты могут содержать сетевые идентификаторы; в отчёт их значения не перенесены.

| Материал | Путь |
|---|---|
| Базовая трасса и ответы Mimic | `.build/flickr-audit/baseline-mimic/` |
| Контроль Chrome | `.build/flickr-audit/baseline-chrome/` |
| Стек Trusted Types | `.build/flickr-audit/dom-instrumented-mimic/capture.json` |
| Weglot URL и DOM-вставка | `.build/flickr-audit/detailed-instrumented-mimic/capture.json` |
| Живое событие Webflow | `.build/flickr-audit/detailed-event-instrumented-mimic/capture.json` |
| Сокращённые сравнения | `.build/flickr-audit/probes-mimic.json`, `probes-chrome.json` |
| CSS, новый пустой Page | `.build/flickr-audit/font-matrix.json` |
| CSS, HTTP-документ | `.build/flickr-audit/font-matrix-http.json` |
| Скрипты диагностики | `.build/flickr_audit.cjs`, `.build/flickr_probes.cjs`, `.build/flickr_font_audit.cjs` |

Команды повторения основных проверок при запущенном Mimic на 127.0.0.1:9223:

```powershell
node .build/flickr_probes.cjs mimic
node .build/flickr_probes.cjs chrome
node .build/flickr_font_audit.cjs
node .build/flickr_audit.cjs mimic repeat-mimic
node .build/flickr_audit.cjs chrome repeat-chrome
```

Числа Trusted Types могут меняться между запусками. Локальный font-скрипт в текущем виде повторяет HTTP-матрицу; исходная матрица пустого Page сохранена отдельным файлом. Live-capture зависит от внешних ресурсов и сети. Производственный код, frozen harnesses и reference expectations не изменялись.

## Дополнение: реализация и повторная проверка

После исходного аудита дефекты №1–14 исправлены в общих механизмах URL,
DOM/realm, Event, отражаемых атрибутов, Trusted Types, CSS, передачи ошибок и
навигации. В `internal/browser/flickr_semantics_test.go` и существующих
проверках добавлены или выполнены короткие регрессии. Это не меняет
исторические результаты базового захвата выше.

На повторной странице перестали возникать необработанные ошибки Weglot,
Webflow и Osano. `window.turnstile` и одно поле ответа появляются. Следующим
отличием оказался жизненный цикл `IntersectionObserver`: Webflow начинает
Turnstile для формы только после входа в область наблюдения. После создания
первого теневого дерева Mimic переводил **всю** страницу с native layout на
запасной JS layout. На Flickr тот ошибочно менял координаты нижних форм и
инициализировал ещё два виджета. Для теневого хоста нулевого размера без
обычных дочерних узлов исправление ограничивает переход затронутой веткой;
хосты, способные менять поток документа, сохраняют общий запасной расчёт.
После этого Mimic и frozen Chrome создают
по одному виджету у формы поиска; две формы подписки остаются за пределами
области наблюдения. В замере координаты верхней формы подписки составили
`y=2764.56` (Mimic) и `y=2758.53` (Chrome), нижней — `y=4278.06` и
`y=4270.61` соответственно. Существующие короткие проверки Shadow DOM,
CSSOM и IntersectionObserver после правки проходят.

В отдельной 90-секундной сетевой трассе после исправлений Mimic показывает
одно поле с пустым значением во всех 18 замерах. Страница не выдаёт
необработанных JS-ошибок. В консоли Turnstile есть `600010`; четыре обращения
к `brunhild.challenges.cloudflare.com` завершились DNS-ошибкой. В отдельном
захвате frozen Chrome 152 также не получил токен и встретил этот DNS-сбой.
Поскольку обычный Chrome пользователя на той же странице получил токен,
эти автоматические захваты не доказывают, что ошибка в реализации Mimic
является причиной отказа Turnstile. Живая сеть и контекст успешного сеанса
обычного Chrome не были доступны через расширение Computer Use.

Повторяемые материалы: `.build/flickr_io_audit.cjs`,
`.build/flickr-audit/io-audit.txt`,
`.build/flickr-audit/challenge-mimic/trace.json` и
`.build/flickr-audit/challenge-chrome/trace.json`. Значения токенов в отчёт не
переносились.
