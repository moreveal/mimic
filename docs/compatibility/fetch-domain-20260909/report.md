# Fetch Body и Streams: проверка домена

2026-09-09, integration binary iteration8, CDP19432. Эталон — pinned Chrome152.0.7977.82, CDP19423. Замена не зависит от GitHub. Source changes после iteration8: один вызов onabort вместо двойного; Go regression tests проходят уже с этой правкой.

Ручные Streams заменены pinned web-streams-polyfill4.3.0 (MIT); bundle/source integrity/license в internal/webapi/vendor/web-streams-polyfill. Fetch Body слой использует его nullable byte streams, disturbed/locked state, tee cloning, единый consume для text/json/arrayBuffer/bytes/blob. Request body transfer/clone и dependent AbortSignal реализованы; canonical Go transport получает bytes и per-fetch cancellation. Body/Request/Response не создают второй DOM и не вводят отдельный event loop.

Generic differential из14 случаев: baseline имел13 расхождений; iteration8 совпадает13/14. Последнее — onabort дважды — исправлено в исходниках после сборки и покрыто Go test. Дополнительные6 probes покрыли Request transfer/clone, read+release disturbance, unsigned-short status conversion, Response statics, DataView offsets. Остается независимое URL serialization отличие redirect trailing slash; здесь не добавлялся локальный workaround.

Go `TestFetchCanonicalBytesCloneAndCancellation` и `TestFetchBodyDisturbanceAndRequestTransfer` проходят. Они проверяют byte-preserving POST/response clone, повторное consumption, настоящий server Request.Context cancellation, точный AbortSignal.reason, перенос body ownership, один onabort. Это реальные canonical transport integration tests, не mock.

## WPT

Тесты и необходимые META helpers скачаны без редактирования с revision в wpt-manifest.json. Runner создает обычную HTML страницу с upstream testharness.js, scripts и resource files. Скрипты запускаются на Window. Worker/HTTPS variants здесь не запускались. Первоначальный эксперимент с инъекцией harness в уже загруженный about:blank давал harness timeout, поэтому исключен из итогового доказательства. Frozen benchmark harness не менялся.

Расширенный subset,13 файлов: Response initialization/static error, disturbed states1–6/pipe, cancel, bad chunks, propagation underlying stream errors. Chrome зарегистрировал113 cases:99PASS14FAIL. Mimic зарегистрировал106:103PASS3FAIL;7 cases не зарегистрированы из-за верхнеуровневого FormData constructor failure в response-init-002. Поэтому корректный denominator —113 ожидаемых cases, а не106.

Mimic failures3: Response.formData отсутствует в body consumer тестах. Дополнительный subset consume-empty/consume-stream/request-body-override обнаруживает тот же domain limit и прерывание регистрации на FormData; см. отдельный JSON. Нельзя представлять его частичный список как полный pass.

Chrome152 сам провалил14 актуальных upstream assertions:2 synchronous bodyUsed после pipeTo/pipeThrough и12 exact identity пользовательских ошибок underlying stream. Mimic/зрелая библиотека проходит эти assertions. Это расхождение frozen browser и текущего WPT, а не основание объявить Chrome полностью passing или переписать библиотеку под сайт.

## Явные границы

- FormData constructor/multipart extraction и formData consumption не завершены; multipart BodyInit сейчас явно NotSupportedError, если объект FormData существует.
- Network loader по-прежнему буферизует полный ответ. API body имеет настоящие stream semantics, но fetch еще не возвращается на headers до загрузки всего body. Это не полный incremental network streaming.
- no-cors filtering, CORS/opaque policy, request header guards, referrer validation и redirects требуют отдельного полного domain corpus. Наличие опций Request не гарантирует весь policy layer.
- Internal adapter читает `_disturbed` pinned polyfill в одном месте; это осознанная зависимость от версии, которую нужно проверять при обновлении. Vendor bundle не модифицирован.
- URL/encoding используют существующие subsystem реализации; библиотека Streams не исправляет их.
- Retention/cancellation после внешнего reader и cloning надо расширять отдельными memory cases; benchmark/performance выводов из этих semantic tests нет.

Исходные runner/probes находятся .build/fetch-domain; JSON evidence сохранено рядом с этим отчетом. Финальные SHA receipts/fast gate относятся к общей интеграционной сборке родительской задачи.

Final review: independent Chrome network-failure probe вернул TypeError, Mimic до correction — raw string из transport. Обертка host.fetch теперь превращает только transport rejection в TypeError, сохраняя abort reason и body stream custom error identity. TestFetchNetworkFailureIsTypeErrorAndBodyErrorIdentity и TestFetchCanonicalBytesCloneAndCancellation PASS после correction. Frozen/performance workloads не изменены.
