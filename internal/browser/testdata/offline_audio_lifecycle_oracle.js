(async () => {
  const context = new OfflineAudioContext(1, 512, 8000);
  const log = [];
  let rendered;
  context.onstatechange = (event) =>
    log.push(['state', context.state, context.currentTime, event.isTrusted]);
  context.oncomplete = (event) =>
    log.push([
      'complete',
      context.state,
      context.currentTime,
      event.isTrusted,
      event.renderedBuffer === rendered,
    ]);
  const suspension = context
    .suspend(130 / 8000)
    .then(() => log.push(['suspended-promise', context.state, context.currentTime]));
  const source = new ConstantSourceNode(context, { offset: 1 });
  source.connect(context.destination);
  source.start();
  const rendering = context.startRendering().then((value) => {
    rendered = value;
    log.push(['render-promise', context.state, context.currentTime]);
    return value;
  });
  await suspension;
  const frozen = context.currentTime;
  await new Promise((resolve) => setTimeout(resolve, 10));
  log.push(['frozen', context.currentTime === frozen]);
  await context.resume();
  log.push(['resume-promise', context.state, context.currentTime]);
  await rendering;
  const errors = {};
  for (const [name, operation] of Object.entries({
    lateSuspend: () => context.suspend(0),
    lateResume: () => context.resume(),
    secondRender: () => context.startRendering(),
  }))
    errors[name] = await operation().then(
      () => '',
      (error) => error.name,
    );
  return { log, errors };
})();
