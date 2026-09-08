# Аудит прежних headless-ожиданий Chrome 152

Дата аудита: 2026-09-08. Сравнивались точный Chrome `152.0.7977.82` в новом
headful-профиле и тот же бинарник в новом headless-профиле. Канонический источник
теперь — `compatibility/captures/navigator-chrome152.json`; mode-specific копия —
`navigator-chrome152-headless.json`.

## Ожидания, изменённые из-за прежнего headless-происхождения

| Наблюдение | Прежнее headless-ожидание | Каноническое headful-ожидание | Классификация |
|---|---|---|---|
| Product token в UA/HTTP UA | `HeadlessChrome` | `Chrome` | режим (3) |
| `permissions.query(geolocation)` | `denied` | `prompt` | режим (3) |
| `permissions.query(camera)` | `denied` | `prompt` | режим (3) |
| `permissions.query(clipboard-read)` | `denied` | `prompt` | режим (3) |
| `permissions.query(keyboard-lock)` | `denied` | `prompt` | режим (3) |
| `getCurrentPosition` без решения пользователя | ошибка `User denied Geolocation` | остаётся pending | режим (3) |
| `clipboard.readText` без решения пользователя | `NotAllowedError` | остаётся pending | режим (3) |
| `getUserMedia({video:true})` без решения пользователя | `NotAllowedError` | остаётся pending | режим (3) |
| `keyboard.lock()` без решения пользователя | `InvalidStateError` | остаётся pending | режим (3) |
| Геометрия выбранного профиля | экран 800×600, outer 780×580, viewport 772×433 | экран 2560×1440 (available 2560×1392), outer 1280×800, viewport 1272×653 | Environment (2) |
| Задержка WebGPU adapter discovery | 250 ms headless-машины | 205 ms в выбранном headful-профиле (пять измерений: 203–226 ms) | Environment (2) |

Четыре exposure-файла (`window-secure`, `window-insecure`,
`window-secure-isolated`, `worker-secure`) пересняты headful Chrome. Состав
глобальных свойств, дескрипторы и прототипы между режимами не изменились; изменены
только происхождение capture и UA в capture context. Поэтому корректная семантика
surface не переписывалась.

## Наблюдения, не превращённые в общие ожидания

- `navigator.connection.downlink/rtt` менялись между запусками: категория (5),
  состояние машины/сессии. Регрессионный тест их не сравнивает с общей семантикой.
- Доступность Bluetooth и завершение перечисления Serial менялись между сессиями:
  категории (2)/(5). Они исключены из общего equality-теста и остаются данными
  конкретного Environment/backend.
- Разрешения и открытые permission UI изолированы по страницам. Промежуточное
  `Document is not focused` классифицировано как tooling artifact (4) и ни в одну
  фикстуру не вошло.
- Probe Clipboard раньше мог сохранить прочитанный текст. Теперь он фиксирует
  только исход (`pending`, ошибка или `{resolved:true}`); содержимое не сохраняется.

Все 17 исторических BrowserScan differential-файлов получили явную metadata с
режимом `headless` и профилем
`historical-chrome-152-windows-x64-headless-uncontrolled`. Они оставлены как
историческое доказательство исправлений и не являются production input.
