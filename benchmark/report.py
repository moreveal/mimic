"""All tables, conclusions and figures are derived from raw observations."""
import csv
import hashlib
import json
import math
from pathlib import Path
import statistics as st
import sys
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt

MIB=2**20
def quantile(values,p):
    a=sorted(values);x=(len(a)-1)*p;lo=math.floor(x);hi=math.ceil(x);return a[lo]+(a[hi]-a[lo])*(x-lo)
def stats(values):
    a=[float(x) for x in values if x is not None]
    if not a:return dict(n=0,median=None,p50=None,p95=None,p99=None,min=None,max=None,sd=None,cv=None)
    mean=st.mean(a);sd=st.stdev(a) if len(a)>1 else 0
    return dict(n=len(a),median=st.median(a),p50=st.median(a),p95=quantile(a,.95) if len(a)>=10 else None,p99=quantile(a,.99) if len(a)>=100 else None,min=min(a),max=max(a),sd=sd,cv=sd/mean if mean else None)
def f(x):return '—' if x is None else f'{x:.2f}'
def table(headers,rows):return '\n'.join(['| '+' | '.join(headers)+' |','|'+'|'.join(['---']*len(headers))+'|']+['| '+' | '.join(str(x).replace('|','/').replace('\n',' ') for x in row)+' |' for row in rows])
def med(rows,key):return stats([r.get(key) for r in rows])['median']
def yaml_lines(value,indent=0):
    prefix=' '*indent;lines=[]
    if isinstance(value,dict):
        for key,item in value.items():
            if isinstance(item,(dict,list)) and item:lines.append(prefix+key+':');lines+=yaml_lines(item,indent+2)
            else:lines.append(prefix+key+': '+json.dumps(item,ensure_ascii=False))
    elif isinstance(value,list):
        for item in value:
            if isinstance(item,(dict,list)):lines.append(prefix+'-');lines+=yaml_lines(item,indent+2)
            else:lines.append(prefix+'- '+json.dumps(item,ensure_ascii=False))
    return lines

def generate(path):
    d=json.loads(path.read_text(encoding='utf-8'));out=path.parent;m=d['metadata'];measured=[r for r in d['rows'] if not r.get('excluded') and r.get('mode') in ('cold','warm')]
    # Recompute derived CPU peaks: adjacent control snapshots can be microseconds
    # apart while Windows CPU accounting ticks are much coarser.
    for row in measured:
        samples=sorted((s for s in row.get('samples',[]) if 'cpu_s' in s),key=lambda s:s['t'])
        peaks=[100*max(0,b['cpu_s']-a['cpu_s'])/(b['t']-a['t']) for a,b in zip(samples,samples[1:]) if b['t']-a['t']>=.025]
        row['peak_cpu_percent']=max(peaks,default=0)
        if row.get('mode')=='cold' and row.get('final_accounting'):
            for k in ['cpu_s','user_s','kernel_s']:row['cold_'+k]=row['final_accounting'][k]
    groups={}
    for r in measured:groups.setdefault((r['system'],r['workload'],r['mode']),[]).append(r)
    metrics=['process_start_ms','http_ready_ms','cdp_ready_ms','runtime_initialization_ms','session_create_ms','navigate_ack_ms','navigation_ms','execution_ms','completion_ms','total_cold_ms','teardown_ms','shutdown_ms','cold_cpu_s','cold_user_s','cold_kernel_s','session_cpu_s','session_user_s','session_kernel_s','workload_cpu_s','workload_user_s','workload_kernel_s','average_cpu_percent','peak_cpu_percent','peak_rss','peak_private','lifecycle_peak_rss']
    summary=[]
    for key,rows in groups.items():
        for metric in metrics:summary.append(dict(system=key[0],workload=key[1],mode=key[2],metric=metric,**stats([r.get(metric) for r in rows])))
    with (out/'summary.csv').open('w',newline='',encoding='utf-8') as file:
        w=csv.DictWriter(file,fieldnames=list(summary[0]) if summary else ['system']);w.writeheader();w.writerows(summary)
    flat=[]
    for r in measured:flat.append({k:v for k,v in r.items() if v is None or isinstance(v,(str,int,float,bool))})
    with (out/'iterations.csv').open('w',newline='',encoding='utf-8') as file:
        w=csv.DictWriter(file,fieldnames=sorted({k for r in flat for k in r}));w.writeheader();w.writerows(flat)
    scaling=[]
    for entry in d['concurrency']:
        waves=[x for x in entry['waves'] if not x['excluded']];diagnostic=False
        if not waves and entry.get('stop_reason') and entry['waves']:waves=entry['waves'][-1:];diagnostic=True
        if not waves:continue
        rows=[r for wave in waves for r in wave['rows']];successful=sum(r['status']=='VALID' for r in rows)
        lat=stats([r['session_latency_ms'] for r in rows]);rss=st.median(x['active_memory']['rss'] for x in waves)/MIB
        scaling.append(dict(system=entry['system'],workload=entry['workload'],n=entry['n'],waves=0 if diagnostic else len(waves),diagnostic_warmup_failure=diagnostic,attempts=len(rows),success_rate=successful/len(rows),rss_mib=rss,rss_per_session_mib=rss/entry['n'],peak_rss_mib=max(max(x['peak_rss'],x['active_memory']['rss'],x['after_teardown']['rss']) for x in waves)/MIB,private_mib=st.median(x['active_memory']['private'] for x in waves)/MIB,
          cpu_s=sum(x['cpu_s'] for x in waves),cpu_per_success_s=sum(x['cpu_s'] for x in waves)/successful if successful else None,cpu_percent=100*sum(x['cpu_s'] for x in waves)/sum(x['elapsed_s'] for x in waves),throughput=successful/sum(x['elapsed_s'] for x in waves),
          p50=lat['p50'],p95=lat['p95'],p99=lat['p99'],min=lat['min'],max=lat['max'],sd=lat['sd'],cv=lat['cv'],recovery_mib=st.median(x.get('after_recovery',x['after_teardown'])['rss'] for x in waves)/MIB,
          active_increment_per_session_mib=st.median((x['active_memory']['rss']-x.get('before_wave',entry['fixed_memory'])['rss'])/entry['n']/MIB for x in waves),
          fixed_ready_mib=entry['fixed_memory']['rss']/MIB,stop=entry.get('stop_reason','')))
    if scaling:
        with (out/'concurrency.csv').open('w',newline='',encoding='utf-8') as file:w=csv.DictWriter(file,fieldnames=list(scaling[0]));w.writeheader();w.writerows(scaling)
    fits=[];marginal=[]
    for work in sorted({x['workload'] for x in scaling}):
        for system in ['chrome','mimic']:
            a=sorted([x for x in scaling if x['system']==system and x['workload']==work and x['success_rate']==1 and not x['stop']],key=lambda x:x['n'])
            for lo,hi in zip(a,a[1:]):marginal.append(dict(system=system,workload=work,lo=lo['n'],hi=hi['n'],delta_mib=hi['rss_mib']-lo['rss_mib'],per_session_mib=(hi['rss_mib']-lo['rss_mib'])/(hi['n']-lo['n'])))
            if len(a)>=2:
                slope,intercept=st.linear_regression([x['n'] for x in a],[x['rss_mib'] for x in a]);res=sum((x['rss_mib']-(intercept+slope*x['n']))**2 for x in a);total=sum((x['rss_mib']-st.mean(x['rss_mib'] for x in a))**2 for x in a)
                fits.append(dict(system=system,workload=work,intercept_mib=intercept,marginal_mib=slope,r2=1-res/total if total else None,levels=len(a),max_n=max(x['n'] for x in a)))
    (out/'summary.json').write_text(json.dumps(dict(single_session=summary,concurrency=scaling,marginal=marginal,fits=fits),indent=2),encoding='utf-8')
    sections=['# Mimic V8 и Chrome 152: baseline Windows x64',
      'Это '+('пробный запуск, недостаточный для выводов.' if m['arguments']['smoke'] else 'измерение после исправления ошибок семантики, разрешённого пользователем. Оптимизации runtime ради производительности не выполнялись.')+' Проверка исходной версии сохранена отдельно в `pre-fix/raw.json`.',
      '## Окружение',table(['Параметр','Значение'],[
        ['Дата начала / конца',m['date']+' / '+d.get('finished','НЕ ЗАВЕРШЕНО')],['Chrome',m.get('chrome_version',{})],['Chromium',str(m['target']['main_branch_revision'])+' / '+m['target']['chromium_commit']],['Mimic commit',m['mimic_commit']],['Mimic V8',m.get('mimic_v8','unknown')],['OS',m['os']],['CPU',m['cpu']],['CPU physical / logical',f"{m['physical_cpus']} / {m['logical_cpus']}"],['RAM GiB',f(m['total_ram']/2**30)],['План питания',m['power_mode']],['Power overlay',m['power_overlay']],['Антивирус',m['antivirus']],['Chrome SHA-256',m['binaries']['chrome']['sha256']],['Mimic SHA-256',m['binaries']['mimic']['sha256']]]),
      'Рабочая станция не была эксклюзивно выделена тесту: фоновые приложения и антивирус включены. Список процессов, версии инструментов, аргументы, SHA-256 фикстур и бинарников находятся в raw.json.',
      '## Методика и корректность',
      'В обеих системах создаётся новая страница и уникальный loopback-origin. HTTP cache отключён, ответы no-store; cookie не используются. Контексты, транспорт и процесс в warm сохраняются, состояние страницы не используется повторно. Это изоляция для данного контролируемого корпуса, а не проверка tenant/security isolation. Cold создаёт новый процесс и профиль. Кэш файлов Windows, DLL и DNS ОС не очищается; «cold» означает новый процесс, а не холодный диск. Вариант warm HTTP cache не измерялся.',
      'Навигация заканчивается только при точном URL, document.readyState === "complete" и наличии __benchRun. Затем одинаковый явный запуск __benchRun; завершение — done и точное совпадение детерминированного результата. Settle = 0. Paint/networkidle не ожидаются. Chrome headless=new; результаты не переносятся автоматически на headful.',
      'Внешние часы perf_counter/QPC; polling 5 ms плюс задержка CDP/планировщика ОС. navigation_ms включает разбор HTML и загрузку скриптов, execution_ms — запуск/выполнение приложения и обнаружение маркера. Они не являются изолированным временем JIT или чистого JavaScript. js_ms страницы диагностический: в Mimic виртуальное время часто равно нулю.',
      'Процесс запускается приостановленным, включается в Windows Job Object, затем возобновляется. process_start_ms — вызов создания процесса; CDP readiness отсчитывается от начала создания до ответа протокола. runtime_initialization_ms — остаток между ними; внутренние фазы V8 отдельно не инструментированы. Cold total включает создание страницы, выполнение, teardown и завершение процесса; удаление временного профиля и обслуживание локального сервера не включены.',
      'CPU — user+kernel всего Job Object, включая завершившихся потомков. Working set/private bytes — сумма всех текущих участников Job Object каждые 50 ms и в контрольных точках. Кратковременные пики памяти могут быть пропущены, общие страницы DLL могут учитываться несколько раз. CPU % задан относительно одного логического ядра; 100% машины = '+str(m['logical_cpus']*100)+'%. Пики CPU чувствительны к дискретности счётчиков Windows.',
      'Одна проверка и по одному первоначальному warmup на серию исключены явно. Основные серии: 10 cold и 20 warm, без удаления медленных наблюдений. p95 публикуется при n≥10, p99 при n≥100; используется линейная интерполяция эмпирических квантилей, хвосты при n=10–20 особенно неустойчивы. SD и CV, min/max всех серий доступны в summary.csv. Ускорение не агрегируется в одно отношение.',
      table(['Система / workload','Correctness gate'],sorted(d['gates'].items()))]
    startup=[r for r in d.get('startup_calibration',{}).get('rows',[]) if not r['excluded']]
    if startup:
        sections+=['### CDP readiness: отдельная серия с одинаковой пробой',
          'В финальной серии обе системы отвечают на Target.getTargets после WebSocket handshake. По 10 новых процессов, порядок систем чередуется; warmup исключён. Дата: '+d['startup_calibration']['date']+'. '+('В первоначальной серии ниже Mimic readiness фиксировался после handshake, Chrome — после Browser.getVersion. Эти исходные значения сохраняются для аудита; вывод о сравнительном readiness основан только на новой общей пробе.' if not m.get('readiness_probe') else 'Основная серия использует ту же общую пробу.'),
          table(['Система','n','CDP p50 ms','p95','Min','Max','SD','Ready RSS MiB'],[[s,z['n'],f(z['p50']),f(z['p95']),f(z['min']),f(z['max']),f(z['sd']),f(st.median(r['ready_memory']['rss'] for r in startup if r['system']==s)/MIB)] for s in ['mimic','chrome'] for z in [stats([r['cdp_ready_ms'] for r in startup if r['system']==s])]])]
    interrupted={}
    for row in d['rows']:
        if row.get('exclusion_reason'):interrupted.setdefault((row['system'],row['workload'],row['mode']),[]).append(row)
    if interrupted:
        sections+=['### Наблюдения прерванных серий (не скрыты и не отфильтрованы по скорости)',
          'Ошибка записи checkpoint потребовала перезапуска незавершённой серии. Эти значения не входят в итоговую серию с непрерывной историей процесса; исходные наблюдения сохранены с exclusion_reason. Ниже статистика прерванной части отдельно.',
          table(['Система','Workload','Mode','n','Completion p50 ms','Min','Max','SD'],[[s,w,mode,z['n'],f(z['median']),f(z['min']),f(z['max']),f(z['sd'])] for (s,w,mode),rows in interrupted.items() for z in [stats([r.get('completion_ms') for r in rows])]])]
    sections+=['## Cold startup (медианы, ms)',table(['Система','Workload','n','Process create','CDP ready','Runtime init','Cold total','Shutdown'],[[s,w,len(r),f(med(r,'process_start_ms')),f(med(r,'cdp_ready_ms')),f(med(r,'runtime_initialization_ms')),f(med(r,'total_cold_ms')),f(med(r,'shutdown_ms'))] for (s,w,mode),r in groups.items() if mode=='cold']),
      '## Warm session startup / teardown (медианы)',table(['Система','Workload','n','Create ms','Teardown ms','RSS после teardown MiB'],[[s,w,len(r),f(med(r,'session_create_ms')),f(med(r,'teardown_ms')),f(st.median(x['after_teardown']['rss'] for x in r)/MIB)] for (s,w,mode),r in groups.items() if mode=='warm']),
      '## Single-session workload latency (ms)',table(['Система','Workload','Mode','n','Nav p50','Execution p50','Completion p50','p95','Min','Max','SD','CV'],[[s,w,mode,len(r),f(med(r,'navigation_ms')),f(med(r,'execution_ms')),f(z['median']),f(z['p95']),f(z['min']),f(z['max']),f(z['sd']),f(z['cv'])] for (s,w,mode),r in groups.items() for z in [stats([x.get('completion_ms') for x in r])]]),
      '## Single-session память / CPU (медианы)',table(['Система','Workload','Mode','До страницы MiB','После create MiB','Peak RSS MiB','Peak private MiB','CPU/session ms','CPU/workload ms'],[[s,w,mode,f(st.median(x['before_session']['rss'] for x in r)/MIB),f(st.median(x['after_session']['rss'] for x in r)/MIB),f(med(r,'peak_rss')/MIB),f(med(r,'peak_private')/MIB),f(med(r,'session_cpu_s')*1000),f(med(r,'workload_cpu_s')*1000)] for (s,w,mode),r in groups.items()]),
      '## Concurrency / density',
      'Каждый уровень — отдельный процесс; один исключённый warmup и max(5, ceil(20/N)) измеряемых волн. Страницы создаются параллельно; после барьера все начинают навигацию. Завершившиеся страницы удерживаются до окончания волны для одновременного RSS. Throughput = успешные сессии / время create→последний teardown, включая барьер и измерения, но без HTTP-server setup/cleanup. Latency = create→completion, включая ожидание барьера. Это batch throughput, не оптимизированный постоянный поток запросов. Удержание не добавляется в latency, но входит в throughput. CPU на сессию = CPU всей волны / число успехов; пересекающиеся интервалы CPU отдельных страниц не суммируются.',
      'Пределы остановки: любая ошибка, <15% либо <2 GiB доступной RAM, >1024 pages input/s в течение 3 s, timeout 180 s. Это защита рабочей машины; максимум прошедшего уровня — нижняя граница поддержанной здесь ёмкости, не доказательство абсолютного максимума.',
      'Mimic сериализует команды CDP общим mutex. Измерение включает это поведение; профилирование причин затрат не проводилось. В строках с остановкой RSS может быть снят раньше завершения всех страниц, а число волн 0 означает остановку на исключаемом warmup. Такие строки диагностические и не входят в fit/графики стабильных уровней. Между волнами дополнительно выполняются 250 ms recovery, очистка серверов и запись checkpoint вне batch throughput.',
      table(['Система','Workload','N','Волн','Успех %','RSS MiB','RSS/N MiB','Peak MiB','CPU/session ms','Сессий/s','p50 ms','p95 ms','p99 ms','Стоп'],[[x['system'],x['workload'],x['n'],x['waves'],f(x['success_rate']*100),f(x['rss_mib']),f(x['rss_per_session_mib']),f(x['peak_rss_mib']),f(x['cpu_per_success_s']*1000 if x['cpu_per_success_s'] is not None else None),f(x['throughput']),f(x['p50']),f(x['p95']),f(x['p99']),x['stop']] for x in scaling]),
      '## Marginal RAM/session',
      'Измерена конечная разность между соседними протестированными N, делённая на ΔN; это не прямое измерение каждого N→N+1. Линейная модель — описательный OLS fit по медианам только успешных уровней. Пересечение — экстраполяция; реальный startup overhead включает исходную пустую страницу. Общий RSS включает процессы и кэши, оставшиеся от предыдущих волн, а число волн зависит от N. Поэтому fit смешивает стоимость активных страниц с историей процесса. Дополнительно показан прирост RSS относительно начала той же волны: он тоже может включать фоновую активность и GC.',
      table(['Система','Workload','N','(Active RSS − before wave RSS)/N MiB'],[[x['system'],x['workload'],x['n'],f(x['active_increment_per_session_mib'])] for x in scaling]),
      table(['Система','Workload','N low→high','ΔRSS MiB','ΔRSS/ΔN MiB'],[[x['system'],x['workload'],f"{x['lo']}→{x['hi']}",f(x['delta_mib']),f(x['per_session_mib'])] for x in marginal]),
      table(['Система','Workload','Fixed fit MiB','Marginal fit MiB','R²','Max stable N'],[[x['system'],x['workload'],f(x['intercept_mib']),f(x['marginal_mib']),f(x['r2']),x['max_n']] for x in fits]),
      '## Teardown / recovery',table(['Система','Workload','N','Ready RSS MiB','RSS после волн MiB','CPU всей серии s','CPU %'],[[x['system'],x['workload'],x['n'],f(x['fixed_ready_mib']),f(x['recovery_mib']),f(x['cpu_s']),f(x['cpu_percent'])] for x in scaling])]
    sections+=['## Локальный сервер (измеряется независимо)',
      'Время обработчика HTTP — от получения запроса обработчиком до завершения записи ответа; не включает очередь TCP/планировщика сервера или сетевой roundtrip. До основного workload сервер создаётся вне измеряемого интервала. Запросы и bytes каждого ресурса записаны в raw.json.',
      table(['Система','Workload','Mode','Запросов','Server p50 ms','Server p95 ms','Server max ms'],[[s,w,mode,z['n'],f(z['p50']),f(z['p95']),f(z['max'])] for (s,w,mode),rows in groups.items() for z in [stats([q['server_ms'] for row in rows for q in row['server_requests']])]]),
      '## Валидность измеряемых серий',
      table(['Система','Workload','Mode','Успех / попыток','Использование сравнения'],[[s,w,mode,str(sum(x['status']=='VALID' for x in rows))+'/'+str(len(rows)),'VALID' if all(x['status']=='VALID' for x in rows) else 'INVALID — error or semantic mismatch; no speed claim'] for (s,w,mode),rows in groups.items()])]
    worklist=sorted({x['workload'] for x in scaling})
    for metric,title,ylabel in [('rss_mib','total-rss','Total working set (MiB)'),('marginal','marginal-rss','Marginal working set (MiB/session)'),('throughput','throughput','Successful sessions / second'),('latency','latency','Session latency (ms)'),('cpu_percent','cpu','CPU (% of one logical core)')]:
        if not worklist:continue
        fig,axes=plt.subplots(1,len(worklist),figsize=(5*len(worklist),4.3),squeeze=False)
        for ax,work in zip(axes[0],worklist):
            for system in ['chrome','mimic']:
                a=sorted([x for x in scaling if x['system']==system and x['workload']==work and x['success_rate']==1 and not x['stop']],key=lambda x:x['n'])
                if metric=='marginal':
                    a=[x for x in marginal if x['system']==system and x['workload']==work];ax.plot([x['hi'] for x in a],[x['per_session_mib'] for x in a],'-o',label=system)
                elif metric=='latency':
                    for q,style in [('p50','-o'),('p95','--x')]:ax.plot([x['n'] for x in a],[x[q] if x[q] is not None else float('nan') for x in a],style,label=system+' '+q)
                else:ax.plot([x['n'] for x in a],[x[metric] for x in a],'-o',label=system)
            ax.set(title=work,xlabel='Concurrent sessions',ylabel=ylabel);ax.grid(alpha=.25);ax.legend()
        fig.tight_layout();fig.savefig(out/(title+'.png'),dpi=150);plt.close(fig);sections+=['!['+ylabel+']('+title+'.png)']
    faster={'mimic':[],'chrome':[]}
    for work in sorted({r['workload'] for r in measured}):
        a=groups.get(('mimic',work,'warm'),[]);b=groups.get(('chrome',work,'warm'),[])
        if a and b and all(r['status']=='VALID' for r in a+b):
            ma,mb=med(a,'completion_ms'),med(b,'completion_ms');faster['mimic' if ma<mb else 'chrome'].append(f'{work} ({f(ma)} / {f(mb)} ms Mimic/Chrome)')
    sections+=['## Инженерный вывод',
      '1. По медиане warm navigation→completion Mimic быстрее: '+('; '.join(faster['mimic']) or 'ни на одной из измеренных нагрузок')+'. '+('CDP readiness (общая проба, отдельные cold): '+('; '.join(s+' '+f(st.median(r['cdp_ready_ms'] for r in startup if r['system']==s))+' ms' for s in ['mimic','chrome'])) if startup else 'Сопоставимая общая проба readiness не выполнена.'),
      '2. Chrome быстрее по той же метрике: '+('; '.join(faster['chrome']) or 'ни на одной из измеренных нагрузок')+'.',
      '3. Фиксированный overhead процесса (CDP ready, включая исходную страницу): '+('; '.join(s+' '+f(st.median(r['ready_memory']['rss'] for r in measured if r['system']==s)/MIB)+' MiB' for s in ['mimic','chrome']) if measured else 'не измерен')+'. Startup latency и OLS intercept приведены отдельно выше.',
      '4. Оценки marginal RAM/session: '+('; '.join(x['system']+'/'+x['workload']+' '+f(x['marginal_mib'])+' MiB' for x in fits) or 'недостаточно успешных уровней для модели')+'.',
      '5. Максимальный стабильный N по нагрузке: '+('; '.join(s+'/'+w+' '+str(max((x['n'] for x in scaling if x['system']==s and x['workload']==w and x['success_rate']==1 and not x['stop']),default=0)) for w in worklist for s in ['mimic','chrome']) or 'не измерен')+'.',
      '6. Throughput на максимальном общем стабильном уровне:']
    claims=[]
    for work in worklist:
        levels=set(x['n'] for x in scaling if x['system']=='mimic' and x['workload']==work and x['success_rate']==1 and not x['stop'])&set(x['n'] for x in scaling if x['system']=='chrome' and x['workload']==work and x['success_rate']==1 and not x['stop'])
        if levels:
            n=max(levels);a=next(x for x in scaling if x['system']=='mimic' and x['workload']==work and x['n']==n);b=next(x for x in scaling if x['system']=='chrome' and x['workload']==work and x['n']==n)
            claims.append(f"{work}, N={n}: Mimic {f(a['throughput'])} и Chrome {f(b['throughput'])} успешных сессий/s; RSS {f(a['rss_mib'])} и {f(b['rss_mib'])} MiB; CPU/session {f(a['cpu_per_success_s']*1000)} и {f(b['cpu_per_success_s']*1000)} ms.")
    sections+=claims+['7. Невалидные сравнения после исправлений: '+(', '.join(k for k,v in d['gates'].items() if v!='VALID') or 'нет на correctness gate; ошибки отдельных итераций остаются в raw.json')+'. До исправлений: DOM, async, React, WebAssembly (см. pre-fix).',
      '8. Ограничения: одна занятая рабочая станция; headless Chrome; различные сборки V8; HTTP cache отключён; ограниченные размеры и семантические проверки корпуса; React-фикстура синтетическая, не Next.js/production-приложение. Polling и sampler входят в нагрузку системы. RSS — сумма рабочих наборов, не уникальная физическая RAM; нет финансовой модели стоимости сессии. Paging — общесистемный счётчик, не атрибуция hard faults конкретному процессу. Значения виртуальных часов Mimic не используются для сравнений.',
      '9. Наиболее сильная защищаемая формулировка для README: «На Windows x64 локальные контролируемые нагрузки прошли проверку результатов в Mimic V8 и Chrome '+m['target']['chrome_version']+'; опубликованы воспроизводимый harness, исходные наблюдения и отдельные показатели latency, CPU и памяти. '+(' '.join(claims[:1]))+'»',
      '10. Данные НЕ подтверждают утверждение «Mimic в X раз быстрее Chrome вообще», полную совместимость с браузером, безопасность изоляции tenants, преимущество чистого V8/JIT, выигрыш на произвольных сайтах или денежную экономию без модели эксплуатации.']
    if d['notes']:sections+=['## Примечания запуска']+d['notes']
    (out/'report.md').write_text('\n\n'.join(sections)+'\n',encoding='utf-8')
    baseline=dict(benchmark='baseline',commit=m['mimic_commit'],harness_commit=m.get('harness_commit'),harness_sha256=m.get('harness_sha256'),date=m['date'],chrome=m['target']['chrome_version'],concurrency_levels=[1,5,10,25,50,100],units=dict(memory='MiB (summed working sets; private bytes separately)',cpu='seconds',latency='milliseconds',throughput='successful sessions/second'),cold=[],warm=[],concurrency=scaling,common_cdp_startup={s:stats([r['cdp_ready_ms'] for r in startup if r['system']==s]) for s in ['mimic','chrome']})
    for (s,w,mode),rows in groups.items():
        baseline[mode].append(dict(system=s,workload=w,iterations=len(rows),latency_ms=stats([r.get('completion_ms') for r in rows]),total_cold_ms=stats([r.get('total_cold_ms') for r in rows]),session_creation_ms=stats([r.get('session_create_ms') for r in rows]),rss_peak_mib=med(rows,'peak_rss')/MIB,private_bytes_peak_mib=med(rows,'peak_private')/MIB,cpu_per_session_s=med(rows,'session_cpu_s')))
    (out/'baseline.yaml').write_text('\n'.join(yaml_lines(baseline))+'\n',encoding='utf-8')
    artifacts={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in out.iterdir() if p.is_file() and p.name!='manifest.json'}
    (out/'manifest.json').write_text(json.dumps(dict(sha256=artifacts,harness_sha256=m.get('harness_sha256')),indent=2),encoding='utf-8')

if __name__=='__main__':generate(Path(sys.argv[1]))
