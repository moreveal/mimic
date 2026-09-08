"""Produce an honest matrix from retained observations, without normalizing diffs."""
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]
def read(path): return json.loads((ROOT/path).read_text(encoding='utf-8'))

GROUPS = [
 ('Legacy identity','vendorSub productSub appCodeName doNotTrack','semantics implemented'),
 ('Input','userActivation scheduling keyboard virtualKeyboard ink','capability-only; input activation implemented'),
 ('Permissions / credentials','permissions credentials clipboard geolocation','capability-only; shared permission store implemented'),
 ('Network / storage','connection storage storageBuckets locks webkitTemporaryStorage webkitPersistentStorage','semantics implemented for tested operations; see limits'),
 ('Media','mediaDevices mediaCapabilities mediaSession presentation','capability-only'),
 ('Devices','bluetooth hid serial usb devicePosture','capability-only; transport unsupported by design'),
 ('Browser services','serviceWorker wakeLock windowControlsOverlay xr managed login','capability-only; service backends unsupported by design'),
 ('Privacy','protectedAudience deprecatedRunAdAuctionEnforcesKAnonymity','capability-only; auctions unsupported by design'),
]
PREFIXES={
 'userActivation':['activation'], 'scheduling':['activation'], 'connection':['connection'],
 'geolocation':['geolocation.'], 'permissions':['permissions.'], 'clipboard':['clipboard.'],
 'credentials':['credentials.'], 'bluetooth':['bluetooth.'], 'hid':['hid.'], 'serial':['serial.'], 'usb':['usb.'],
 'devicePosture':['posture'], 'storage':['storage.'], 'storageBuckets':['buckets.'], 'locks':['locks.'],
 'webkitTemporaryStorage':['quota.temporary','quota.identity'], 'webkitPersistentStorage':['quota.persistent','quota.identity'],
 'mediaDevices':['media.enumerate','media.getUserMedia','media.invalid','media.constraints'],
 'mediaCapabilities':['media.decoding','media.mp4'], 'mediaSession':['media.session'], 'presentation':['presentation'],
 'serviceWorker':['serviceWorker.'], 'xr':['xr.'], 'wakeLock':['wakeLock.'], 'keyboard':['keyboard.'],
 'virtualKeyboard':['virtualKeyboard'], 'ink':['ink'], 'managed':['managed'], 'login':['login.'],
 'windowControlsOverlay':['overlay'], 'protectedAudience':['protectedAudience'],
}

def main():
    chrome=read('compatibility/captures/navigator-chrome152.json')
    mimic=read('compatibility/captures/navigator-mimic-current.json')
    baseline=read('compatibility/captures/navigator-mimic-baseline.json')
    catalog=read('chrome/152/generated/webapi.json')
    declarations={d['name']:d for d in catalog['declarations']}
    nav={m.get('name'):m for m in declarations['Navigator']['members']}
    contexts=['secure','isolated','opaque']
    operations=chrome['secure']['operations']
    differences={k:{'chrome':v,'mimic':mimic['secure']['operations'].get(k)} for k,v in operations.items() if v!=mimic['secure']['operations'].get(k)}
    opaque_operations=chrome['opaque']['operations']
    opaque_differences={k:{'chrome':v,'mimic':mimic['opaque']['operations'].get(k)} for k,v in opaque_operations.items() if v!=mimic['opaque']['operations'].get(k)}
    names=[n for _,members,_ in GROUPS for n in members.split()]
    shapes=lambda data:sum(chrome[c]['members'][n]==data[c]['members'][n] for c in contexts for n in names)
    rows=[];provenance=[]
    for group,members,scope in GROUPS:
        for name in members.split():
            c=[chrome[ctx]['members'][name] for ctx in contexts];m=[mimic[ctx]['members'][name] for ctx in contexts]
            exposure=lambda values:'/'.join('да' if row['exposed'] else 'нет' for row in values)
            descriptor=all(a.get('descriptor')==b.get('descriptor') and a.get('prototypeDescriptors')==b.get('prototypeDescriptors') for a,b in zip(c,m))
            prototype=all(all(a.get(k)==b.get(k) for k in ['owner','chain','prototype','constructor','constructorIdentity','tag','sameObject','ownKeys','illegalReceiver']) for a,b in zip(c,m))
            failed=['secure: '+key for key in differences if any(key.startswith(prefix) for prefix in PREFIXES.get(name,[]))]
            failed+=['opaque: '+key for key in opaque_differences if any(key.startswith(prefix) for prefix in PREFIXES.get(name,[]))]
            values='совпали в пробах' if not failed else 'расхождения: '+', '.join(failed)
            rows.append(f'| `{name}` / {group} | {exposure(c)} | {exposure(m)} | {"да" if descriptor else "НЕТ"} | {"да" if prototype else "НЕТ"} | {values} | {scope} |')
            member=nav[name];interface=declarations.get(member.get('type'),{})
            provenance.append({'member':name,'domain':group,'type':member.get('type'),'navigatorExposure':declarations['Navigator']['extended'],
              'declaringSource':member.get('origin'),'memberConditions':member.get('extended'),
              'interfaceSource':interface.get('source'),'interfaceConditions':interface.get('extended'),
              'chromeExposed':dict(zip(contexts,[v['exposed'] for v in c]))})
    report=f'''# Navigator capability domains — Chrome 152

Эталон: `{chrome['product']}`, Windows x64, headful, свежий контролируемый профиль, без feature overrides; синтетические локальные страницы. `Runtime.evaluate` выполняется с `userGesture:false`. Сеть и доступность OS-служб — состояние машины, а не константы версии Chrome.

Контексты в столбцах exposure: **secure / secure+COOP+COEP / opaque about:blank**. Данные insecure Window дополнительно получены на `data:`. Другие профили, origin trials, PWA, managed apps и разрешения iframe этим отчётом не сертифицированы.

Полная форма совпала: **{shapes(mimic)}/{len(names)*len(contexts)}** наблюдений, исходный HEAD `{baseline.get('sourceRevision','recorded baseline')}`: **{shapes(baseline)}/{len(names)*len(contexts)}**. В secure-контексте совпали **{len(operations)-len(differences)}/{len(operations)}** результатов операций; в opaque — **{len(opaque_operations)-len(opaque_differences)}/{len(opaque_operations)}**. Для isolated здесь проверена форма, не операции. Это ограниченный набор наблюдений, а не процент совместимости браузера.

Проверяются владелец, дескрипторы, native Function#toString, name/length, собственные ключи функций, неконструируемость методов/getters, readonly constructor.prototype, цепочки наследования, constructor identity, same-object, отсутствие утечек собственных полей и illegal receiver.

| member / domain | Chrome exposed? | Mimic exposed? | descriptor parity | prototype parity | capability/value parity | semantics / capability-only / unsupported |
|---|---|---|---|---|---|---|
'''+ '\n'.join(rows)+'''

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

- [Headful Chrome capture](../compatibility/captures/navigator-chrome152.json), [mode-specific headless capture](../compatibility/captures/navigator-chrome152-headless.json), [исходный HEAD](../compatibility/captures/navigator-mimic-baseline.json), [Mimic после изменений](../compatibility/captures/navigator-mimic-current.json).
- [Oracle policy](oracle-policy.md) и [отчёт аудита headless-ожиданий](oracle-headless-audit.md).
- [Точные расхождения операций](navigator-capability-differences.json), [происхождение и условия каждого члена](navigator-capability-provenance.json).
- [Общий сценарий захвата](../compatibility/navigator_capabilities.py), [пробы](../compatibility/probes-navigator-capabilities.json), [регрессионные тесты](../internal/browser/capabilities_test.go).
- [109 закреплённых IDL-исходников](../chrome/152/generated/navigator-idl-sources.json), [их воспроизводимое восстановление](../tools/refresh_navigator_idl.py), [тесты парсера](../tools/test_generate_compat.py).

Матрица не означает завершённую реализацию всех методов этих интерфейсов. `capability-only` и `unsupported by design` остаются явными границами, даже когда shape полностью совпадает.
'''
    (ROOT/'docs/navigator-capabilities.md').write_text(report,encoding='utf-8')
    for name,value in [('navigator-capability-differences.json',{'secure':differences,'opaque':opaque_differences}),('navigator-capability-provenance.json',provenance)]:
        (ROOT/'docs'/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
    print(f'Shape {shapes(mimic)}/{len(names)*len(contexts)}; operations {len(operations)-len(differences)}/{len(operations)}; baseline shape {shapes(baseline)}')

if __name__=='__main__':main()
