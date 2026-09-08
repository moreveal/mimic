# Navigator capability domains — Chrome 152

Эталон: `Chrome/152.0.7977.82`, Windows x64, headless, отдельный профиль, без feature overrides; синтетические локальные страницы. `Runtime.evaluate` выполняется с `userGesture:false`. Сеть и доступность OS-служб — состояние машины, а не константы версии Chrome.

Контексты в столбцах exposure: **secure / secure+COOP+COEP / opaque about:blank**. Данные insecure Window дополнительно получены на `data:`. Другие профили, origin trials, PWA, managed apps и разрешения iframe этим отчётом не сертифицированы.

Полная форма совпала: **108/108** наблюдений, исходный HEAD `73d6b8b49d05d6c1cf27695e0c67560bac1ef0eb`: **0/108**. В secure-контексте совпали **51/54** результатов операций; в opaque — **53/54**. Для isolated здесь проверена форма, не операции. Это ограниченный набор наблюдений, а не процент совместимости браузера.

Проверяются владелец, дескрипторы, native Function#toString, name/length, собственные ключи функций, неконструируемость методов/getters, readonly constructor.prototype, цепочки наследования, constructor identity, same-object, отсутствие утечек собственных полей и illegal receiver.

| member / domain | Chrome exposed? | Mimic exposed? | descriptor parity | prototype parity | capability/value parity | semantics / capability-only / unsupported |
|---|---|---|---|---|---|---|
| `vendorSub` / Legacy identity | да/да/да | да/да/да | да | да | совпали в пробах | semantics implemented |
| `productSub` / Legacy identity | да/да/да | да/да/да | да | да | совпали в пробах | semantics implemented |
| `appCodeName` / Legacy identity | да/да/да | да/да/да | да | да | совпали в пробах | semantics implemented |
| `doNotTrack` / Legacy identity | да/да/да | да/да/да | да | да | совпали в пробах | semantics implemented |
| `userActivation` / Input | да/да/да | да/да/да | да | да | совпали в пробах | capability-only; input activation implemented |
| `scheduling` / Input | да/да/да | да/да/да | да | да | совпали в пробах | capability-only; input activation implemented |
| `keyboard` / Input | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; input activation implemented |
| `virtualKeyboard` / Input | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; input activation implemented |
| `ink` / Input | да/да/да | да/да/да | да | да | совпали в пробах | capability-only; input activation implemented |
| `permissions` / Permissions / credentials | да/да/да | да/да/да | да | да | совпали в пробах | capability-only; shared permission store implemented |
| `credentials` / Permissions / credentials | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; shared permission store implemented |
| `clipboard` / Permissions / credentials | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; shared permission store implemented |
| `geolocation` / Permissions / credentials | да/да/да | да/да/да | да | да | совпали в пробах | capability-only; shared permission store implemented |
| `connection` / Network / storage | да/да/да | да/да/да | да | да | расхождения: secure: connection, opaque: connection | semantics implemented for tested operations; see limits |
| `storage` / Network / storage | да/да/нет | да/да/нет | да | да | совпали в пробах | semantics implemented for tested operations; see limits |
| `storageBuckets` / Network / storage | да/да/нет | да/да/нет | да | да | совпали в пробах | semantics implemented for tested operations; see limits |
| `locks` / Network / storage | да/да/нет | да/да/нет | да | да | совпали в пробах | semantics implemented for tested operations; see limits |
| `webkitTemporaryStorage` / Network / storage | да/да/да | да/да/да | да | да | совпали в пробах | semantics implemented for tested operations; see limits |
| `webkitPersistentStorage` / Network / storage | да/да/да | да/да/да | да | да | совпали в пробах | semantics implemented for tested operations; see limits |
| `mediaDevices` / Media | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only |
| `mediaCapabilities` / Media | да/да/да | да/да/да | да | да | совпали в пробах | capability-only |
| `mediaSession` / Media | да/да/да | да/да/да | да | да | совпали в пробах | capability-only |
| `presentation` / Media | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only |
| `bluetooth` / Devices | да/да/нет | да/да/нет | да | да | расхождения: secure: bluetooth.available | capability-only; transport unsupported by design |
| `hid` / Devices | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; transport unsupported by design |
| `serial` / Devices | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; transport unsupported by design |
| `usb` / Devices | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; transport unsupported by design |
| `devicePosture` / Devices | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; transport unsupported by design |
| `serviceWorker` / Browser services | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; service backends unsupported by design |
| `wakeLock` / Browser services | да/да/нет | да/да/нет | да | да | расхождения: secure: wakeLock.request | capability-only; service backends unsupported by design |
| `windowControlsOverlay` / Browser services | да/да/да | да/да/да | да | да | совпали в пробах | capability-only; service backends unsupported by design |
| `xr` / Browser services | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; service backends unsupported by design |
| `managed` / Browser services | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; service backends unsupported by design |
| `login` / Browser services | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; service backends unsupported by design |
| `protectedAudience` / Privacy | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; auctions unsupported by design |
| `deprecatedRunAdAuctionEnforcesKAnonymity` / Privacy | да/да/нет | да/да/нет | да | да | совпали в пробах | capability-only; auctions unsupported by design |

## Состояние и границы

- Legacy: фиксированные правила Blink и preference DoNotTrack; это не значения из сайта.
- Input: trusted `Page.DispatchInput`, очередь UserInteraction и время Scheduler; synthetic dispatchEvent не активирует страницу. Поддержаны mousedown/keydown/touchend, expiry и sticky activation. Это не полный CDP Input, hit-testing или полный алгоритм распространения/потребления активации. KeyboardLayoutMap берёт раскладку из Environment. Реальных keyboard lock, virtual keyboard и ink renderer нет.
- Permissions: один origin store в Context; query, location, clipboard, media и keyboard используют его; изменения PermissionStatus доставляются задачами. Полный набор специализированных descriptor-алгоритмов (MIDI sysex, requestedOrigin, fullscreen и другие) не реализован. Credential transport, WebAuthn/FedCM и ClipboardItem transport отсутствуют. У location без backend нет фиктивной позиции.
- Network/storage: connection читает Environment.Network. Quota имеет общий профиль; Web Storage, как в Chrome, не учитывается в StorageManager usage. Поддержаны bucket metadata/lifecycle, expiry и persistence. Quota-consuming IDB/Cache/OPFS clients отсутствуют: estimate не является статистикой реального диска. Locks поддерживает очередь, shared/exclusive, callback lifetime, ifAvailable, abort и освобождение при закрытии realm; steal явно не поддерживается. Старые два quota getter возвращают один объект без публичного конструктора.
- Media: одна модель установленных классов устройств, redaction и capability profile. Потоки не создаются. Codec negotiation ограничен настроенными content types и проверенными конфигурациями; это не полный decoder/encoder/EME, media pipeline или проверка производительности произвольных разрешений/битрейтов. MediaSession хранит playback/action/position state; конструкторы MediaMetadata/PresentationRequest и Presentation transport не реализованы.
- Devices: один пустой transport registry, а не независимые успешные устройства. HID/USB/Serial возвращают пустые списки; chooser без активации отвергается. Bluetooth getDevices отсутствует в выбранном Chrome-профиле. Availability Bluetooth на этой машине иногда не успевает завершиться за 1500 ms, иногда возвращает false; Mimic сообщает отсутствие адаптера сразу.
- Browser services: нет service-worker execution, XR session backend, presentation receiver, managed configuration и системного power backend. Controller/registration queries описывают пустое состояние; ready остаётся pending. Неподдержанные операции явно отвергаются, не возвращают успешный null из semanticMissing. Chrome с power backend выдаёт WakeLockSentinel; Mimic отклоняет request. Это сознательное расхождение, не доказанная Chrome parity. Window controls overlay выключен для обычного окна. Login status хранится по origin.
- Privacy: поддержаны наблюдённые capability ответы; auctions/interest-group backend не реализован. Связанные операции Navigator из того же IDL-источника явно отклоняют Promise вместо успешного null из semanticMissing. deprecatedRunAdAuctionEnforcesKAnonymity равен false в выбранном профиле.

Exposure выбирается из закреплённого снимка, затем может быть ограничен явными Environment.Features. Неизвестный или включённый override не добавляет API поверх снимка. Сохранены исходники и условия partial/mixin и условного Exposed(Window Feature); происхождение не подменяется слитыми условиями Navigator.

## Проверяемые артефакты

- [Chrome capture](../compatibility/captures/navigator-chrome152.json), [исходный HEAD](../compatibility/captures/navigator-mimic-baseline.json), [Mimic после изменений](../compatibility/captures/navigator-mimic-current.json).
- [Точные расхождения операций](navigator-capability-differences.json), [происхождение и условия каждого члена](navigator-capability-provenance.json).
- [Общий сценарий захвата](../compatibility/navigator_capabilities.py), [пробы](../compatibility/probes-navigator-capabilities.json), [регрессионные тесты](../internal/browser/capabilities_test.go).
- [109 закреплённых IDL-исходников](../chrome/152/generated/navigator-idl-sources.json), [их воспроизводимое восстановление](../tools/refresh_navigator_idl.py), [тесты парсера](../tools/test_generate_compat.py).

Матрица не означает завершённую реализацию всех методов этих интерфейсов. `capability-only` и `unsupported by design` остаются явными границами, даже когда shape полностью совпадает.
