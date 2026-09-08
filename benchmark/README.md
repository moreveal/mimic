# Воспроизводимый benchmark: Mimic V8 и Chrome 152

Из корня репозитория, Windows x64:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File benchmark/run.ps1
```

Команда создаёт локальное Python-окружение, устанавливает закреплённые зависимости,
собирает текущий Mimic, проверяет семантику, выполняет серии и генерирует
`benchmark/results/raw.json`, CSV, `summary.json`, `report.md` и пять PNG-графиков.
Нужны Python 3.14 x64, Go 1.26, CGO/GCC и закреплённые зависимости проекта.
Первая установка Python-пакетов использует интернет; сами нагрузки полностью локальны.
React 18.3.1 / ReactDOM 18.3.1 включены в `fixtures/vendor` с MIT-лицензией.
Их источники: npm-пакеты `react@18.3.1` и `react-dom@18.3.1`, файлы
`umd/react.production.min.js` и `umd/react-dom.production.min.js`.

Автопоиск Chrome проверяет проектный `compatibility/.chrome-for-testing/152.0.7977.82/`
и сохранённый рядом архив `mimic-cleanup-private-archive-20260908`. Другой путь:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File benchmark/run.ps1 -Chrome 'D:\browsers\chrome-win64\chrome.exe'
```

Версия CDP обязана быть точно `Chrome/152.0.7977.82`; подмена установленным Chrome
другой версии запрещена. SHA-256 исполняемых файлов, V8, Chromium revision,
Mimic commit, CPU/RAM/OS, питание и фоновые процессы сохраняются в raw.json.
Пользовательские абсолютные пути заменяются `<repo>`, `<workspace-parent>`, `<user>`.
Аргументы запуска определены в `Runtime` и одинаковы во всех итерациях системы.
Экземпляры Chrome используют новые профили, порты и TEMP; расширения отключены.

Для быстрой проверки harness используйте `-Smoke`: это отдельный результат в
`.build/benchmark-smoke`, непригодный для выводов о производительности.
Только correctness gate:

```powershell
.build/benchmark-venv/Scripts/python.exe benchmark/run.py --gate-only --output .build/benchmark-gate
```

При прерывании сохранённый запуск можно продолжить той же командой с `-Resume`.
Хеши обоих бинарников и фикстур обязаны совпасть. Завершённые серии сохраняются,
незавершённая серия запускается целиком с новым процессом; прерванные наблюдения
не удаляются, а помечаются и отдельно суммируются в отчёте. Новые даты/состояние
машины при продолжении находятся в `resumptions`. Обычный запуск без `-Resume`
откажется перезаписывать каталог с raw.json: задайте новый `-Output`.

## Зафиксированный baseline и будущие оптимизации

`results/baseline.yaml` сохраняет commit, commit/hash harness, cold/warm,
уровни 1/5/10/25/50/100, RSS, private bytes, CPU, latency и throughput.
`manifest.json` содержит SHA-256 исходных данных и всех итоговых артефактов.
Хеш harness включает измеритель, отчёт, comparator, dependency lock и фикстуры.
Он проверяется при продолжении и в конце измерения: менять harness во время
прогона нельзя. Репозиторный baseline не перезаписывается.

Будущие оптимизации сохраняются отдельным коммитом вида `perf: reduce ...`.
Нельзя одновременно менять workload, ожидаемые результаты, тайминги, флаги,
порог остановки, статистику или sampler. Запускается **тот же harness без изменений**:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File benchmark/run.ps1 -Output benchmark/runs/after-optimization
.build/benchmark-venv/Scripts/python.exe benchmark/compare.py benchmark/results benchmark/runs/after-optimization --output benchmark/runs/comparison.json
```

Comparator проверяет хеш harness/фикстур/Chrome, CPU, OS, RAM, питание и политику
повторений; сравнивает только совпадающие успешные серии и уровни concurrency.
Процент рассчитывается из чисел `(after - before) / before * 100`, без ручных
«выигрышей». Отрицательное изменение RSS/CPU/latency — улучшение, отрицательное
изменение throughput — ухудшение. Фоновые приложения и состояние памяти ОС всё
равно могут различаться: большой CV или paging требует интерпретации, а не
объявления универсального ускорения.

Перегенерировать отчёт из сохранённых данных без браузеров:

```powershell
.build/benchmark-venv/Scripts/python.exe benchmark/report.py benchmark/results/raw.json
```

Проверка счётчиков Job Object, учёта завершившихся дочерних процессов и статистики:

```powershell
.build/benchmark-venv/Scripts/python.exe benchmark/test_harness.py
```

## Контракт измерения

| Этап | Начало → конец |
|---|---|
| process_start_ms | Popen → создан приостановленный процесс |
| http_ready_ms | начало Popen → доступен /json/version |
| cdp_ready_ms | начало Popen → WebSocket подключён и получен ответ CDP |
| runtime_initialization_ms | cdp_ready_ms − process_start_ms; включает harness/протокол |
| session_create_ms | подключение control CDP → новая страница, подключение page CDP, отключение HTTP cache |
| navigate_ack_ms | Page.navigate → ACK, только диагностика |
| navigation_ms | Page.navigate → точный URL + readyState complete + функция нагрузки |
| execution_ms | явный запуск нагрузки → обнаружение done и чтение результата |
| completion_ms | Page.navigate → done; navigation_ms + execution_ms с небольшим промежутком harness |
| teardown_ms | закрытие page CDP → Target.closeTarget и отсутствие target в Target.getTargets |
| total_cold_ms | Popen → завершение runtime и его Job Object |

Workload settle = 0: нет ожидания paint, visual rendering или networkidle.
CDP readiness подтверждается одинаковым `Target.getTargets` в обеих системах.
В конце также выполняются 10 interleaved cold запусков с этой пробой и отдельным
warmup. В сохранённом первом baseline первоначальная readiness-проба различалась;
его отчёт использует для вывода о startup только исправленную отдельную серию,
оставляя первоначальные наблюдения видимыми.
На каждый controlled workload: 10 cold и 20 warm измерений, плюс исключённый
warmup. Все медленные итерации сохраняются. Ошибка correctness gate исключает
нагрузку из timed comparison обеих систем; ошибка timed iteration сохраняется
и запрещает вывод о преимуществе по соответствующей серии.

Каждая страница получает уникальный порт локального сервера, не переиспользуемый
в пределах запуска. Серверы используют только диапазон 49152–65534: на этой
машине системный ephemeral range начинается с 1024 и может выдавать порты,
запрещённые Chrome. URL и полный Page.navigate ACK сохраняются для диагностики.
`localStorage` дополнительно проверяется на отсутствие следа
предыдущей итерации. Страницы не пишут cookies, не регистрируют service workers,
не используют внешний сайт. Mimic не поддерживает создание CDP browser contexts:
используется изоляция страниц/оригинов, а не независимые security tenants.
Процесс, общий транспорт и контекст в warm сохраняются. HTTP cache отключён
с обеих сторон, `Cache-Control: no-store`; warm-cache HTTP вариант не заявляется.
DNS, кэш DLL/файлов ОС и состояние машины не сбрасываются.

| Нагрузка | Независимое ожидаемое значение |
|---|---|
| static | текст baseline, один root |
| cpu | сумма 59614380 и SHA-256 её десятичной строки; объекты/массивы/Map/JSON/regex/Promise/crypto |
| dom | 3000 элементов, 3000 active, конкретный последний текст, число дочерних узлов и общая длина текста |
| async | точные Promise/microtask/timer/MessageChannel/Worker/window.postMessage/fetch/XHR результаты |
| react | React 18.3.1: fetch, 200 карточек, три state transition, effect marker, первый/последний текст |
| wasm | async instantiate и 100000 вызовов i32 add; сумма 704982704 |

React-корпус — воспроизводимая синтетическая клиентская программа, не полный
Next.js-сайт и не утверждение о совместимости любых React-приложений.
Фикстуры и их ожидаемые результаты одинаковы для обоих движков; нет веток по UA.

## Память, CPU и concurrency

Windows Job Object назначается до возобновления процесса и наследуется потомками.
Job CPU accounting сохраняет user/kernel CPU уже завершившихся процессов.
Рабочие наборы и private bytes суммируются по Job Object, включая browser,
renderer, GPU, network, utility и служебные процессы, созданные этим экземпляром.
Управляющий Python и HTTP-серверы исключены из метрик SUT. Их системная нагрузка
остаётся фактором измерения. Общие DLL-страницы могут учитываться несколько раз.
Это сумма working set, а не измерение unique resident RAM или доступной памяти ОС.
Период sampler — 50 ms; реальные timestamps сохранены, короткие пики могут теряться.

Проверяются 1, 5, 10, 25, 50, 100 одновременных страниц для static, cpu и React.
Для каждого уровня новый процесс, исключённая warmup-волна и
`max(5, ceil(20/N))` измеряемых волн. Страницы создаются параллельно, затем барьер
запускает навигации; все завершившиеся страницы удерживаются до окончания волны
для измерения одновременного RSS. CPU считается один раз на волну, а не суммой
перекрывающихся интервалов страниц. Throughput включает create, выполнение,
барьер и teardown; HTTP-server setup/cleanup вынесены за измеряемый интервал.
Это throughput пакетной нагрузки, не результат оптимизированного steady-state pool.

После warm iteration и density wave есть отдельный период восстановления 250 ms,
одинаковый для обеих систем и не включённый в latency/batch throughput. Память до
новой волны, при активных страницах, сразу после teardown и после восстановления
сохраняется. Память сохранённых движком процессов/кэшей не вычитается из общего RSS.
Дополнительные промежутки между волнами включают очистку серверов и запись
checkpoint JSON. Они вне throughput; поэтому это не измерение непрерывного
production-пула. При остановке на warmup строка уровня диагностическая: RSS
мог быть снят до завершения всех страниц, и не используется в memory fit.
OLS fit и конечные разности отражают этот конкретный жизненный цикл и историю
волн; это не универсальная стоимость новой страницы и не security isolation cost.

Рост N прекращается при ошибке, доступной RAM ниже max(15%, 2 GiB), длительном
paging (`Memory\\Pages Input/sec` >1024 в течение 3 s) или timeout волны 180 s.
Для failed warmup уровень отмечается неуспешным, следующие N не запускаются.
Достигнутые 100 сессий означают «100 проверено», а не предел движка.

Процессы закрываются через Browser.close / Ctrl+C в специально созданной скрытой
консоли Mimic. Остаточные процессы завершаются только через принадлежащий тесту
Job Object. Job также закрывает потомков при аварии harness. Чужие Chrome не
перечисляются для остановки и не завершаются. Удаляется только собственный TEMP.

## Изменения runtime

По дополнительному поручению пользователя исправлены семантические ошибки,
выявленные исходным gate; см. `docs/benchmark-semantics.md` и отдельный коммит.
`results/pre-fix/raw.json` — только исходная проверка корректности, не baseline
производительности. Финальные числа относятся к хешу Mimic из metadata итогового
raw.json. Runtime не содержит веток или оптимизаций для benchmark-корпуса.
