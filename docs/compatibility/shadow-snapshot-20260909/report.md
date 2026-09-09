# Shadow DOM теряется при статическом экспорте

Диагностика2026-09-09, iteration10 на19602. Первичный диагноз получен до исправления.

Первое детерминированное расхождение относительно отображения дат находится на export boundary. Generic autonomous custom element создает shadow span '2 weeks ago', сохраняя light fallback 'Aug26,2026'. Chrome152 и Mimic runtime совпадают: shadow text относительный, light text абсолютный, Intl.RelativeTimeFormat('en').format(-2,'week') одинаков. Mimic.captureSnapshot сохраняет только light DOM. Exported HTML вообще не содержит shadow span или declarative shadow template; его просмотр закономерно показывает fallback.

Live GitHub подтверждает этот же путь: relative-time зарегистрирован; Aug26 shadow='2 weeks ago', Apr22='5 months ago', Jun27='3 months ago'. Titles обновлены с временем и timezone. Отсутствие относительных дат в пользовательском снимке не означает остановку JS или отсутствие Intl. ShadowRoot.innerHTML также возвращает пустую строку при непустом shadowRoot.textContent — отдельное проявление неполной serialization семантики.

Нужен generic snapshot serializer, включающий ShadowRoot subtree с CSS encapsulation и slot semantics (предпочтительно declarative shadow DOM), с учетом open/closed roots, adopted stylesheets и вложенных roots. Простое замещение light text строкой shadow text исправило бы только видимый частный случай и разрушило бы общую семантику. Shadow state хранится realm-side, поэтому Go d.InnerHTML без передачи shadow roots не может сериализовать его самостоятельно. Исправление выполняет родительская задача/агент DOM.

Evidence: independent.json, independent-export.html, site-state.json рядом. Старые DOM.getOuterHTML source-only snapshots также непригодны для оценки post-hydration mutations, но это другой баг: Mimic.captureSnapshot читает текущий light DOM корректно и отдельно теряет shadow trees.

## Реализация общего слоя сериализации

Добавлены realm callback с metadata attachment и canonical child IDs, immutable x/net/html projection, declarative templates для открытых/закрытых и вложенных roots. Light DOM и slot elements сохраняются. Getter ShadowRoot.innerHTML сериализует актуальные canonical children; setter использует существующий mature HTML parser в контексте shadow host. При экспорте shadow scripts удаляются обычным экспортным фильтром, ресурсы и CSS проходят существующий rewrite. JavaScript повторно не выполняется. Проекция строится только на время экспорта; отдельного mutable DOM или кеша мутаций нет.

Регрессия TestSnapshotPreservesCanonicalShadowComposition проверяет open/closed/nested roots, delegatesFocus, именованный slot и light fallback, актуальную экранированную мутацию, удаление shadow script и неизменность live state. Результат интеграционного запуска будет дописан после host bridge.

Ограничения: adoptedStyleSheets/CSSStyleSheet пока не имеют полноценного canonical CSSOM в Mimic и этим исправлением не реализуются. Manual slot assignment невозможно восстановить одной declarative HTML serialization; named slot composition сохраняется. Это экспорт текущего состояния, а не реализация полного getHTML(options) Web API. Сложность проекции линейна в light/shadow nodes; дополнительных measured performance numbers пока нет.

Интеграция: TestSnapshotPreservesCanonicalShadowComposition PASS (root); TestShadowProjectionDoesNotMutateCanonicalTree PASS. Свежий hash-verified V8 бинарник19608: independent custom element Chrome↔Mimic exact live state match; exported declarative HTML→pinned Chrome152 сохраняет open shadow text и innerHTML, closed nested root и named slot assignedNodes. Evidence after-independent.json, after-export.html, after-receipt.json. body.innerText не используется как доказательство shadow rendering: Chrome сам возвращает только light text для этого probe.

Возврат к production workload: свежий экспорт GitHub→Chrome152 дал точное совпадение23/23 relative-time по datetime, light text и shadow text. Визуально проверенный chrome-export.png показывает относительные даты2weeks/5months/3months и заполненные file rows. Единственное предупреждение export — upstream404 alert-fill-12.svg. Evidence after-site.json; screenshot .build/shadow-export-domain/github/chrome-export.png. Production код не содержит логики GitHub/relative-time.

Final review выявил и независимо подтвердил generic fragment-context mismatch: div shadow innerHTML='<tr><td>x</td></tr>' давал TR вместо Chrome text x. Setter исправлен на host context через canonical parser bridge, без вызова конструктора временного custom element. TestShadowInnerHTMLUsesHostParserContext и основная export regression PASS после исправления. Before evidence .build/shadow-review-before.json. Одновременно Fetch network rejection приведен к TypeError; stream body custom errors и abort reason сохраняют identity,4targeted tests PASS. Финальная performance проверка проводится root после этих правок.
