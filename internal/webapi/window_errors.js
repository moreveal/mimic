// Timer and listener exceptions use the Window error-reporting path before
// reaching diagnostic consumers. Cancellation suppresses uncaught reporting,
// while exceptions in an error listener are reported after the original one.
let reportingWindowException = false;
const deferredWindowExceptions = [];
function reportWindowException(error, source, frames) {
  let details = source?.filename ? source : host.describeException(error);
  if (!details) {
    let message = 'Uncaught';
    try {
      message += ' ' + String(error);
    } catch {}
    details = { message, filename: '', lineno: 0, colno: 0 };
    host.semanticMissingAt('window_errors.js:exception-location', 'Window.error.sourceLocation');
  }
  if (reportingWindowException) {
    deferredWindowExceptions.push({ details, error });
    return;
  }
  if (frames?.length) details.stackFrames = frames;
  const report = (details, value) => {
    let errorObject = false,
      stack = '';
    try {
      errorObject = value instanceof Error;
      if (errorObject && typeof value.stack === 'string') stack = value.stack;
    } catch {}
    host.reportUnhandledException(
      details.message,
      details.filename,
      details.lineno,
      details.colno,
      details.stackFrames || [],
      typeof value,
      errorObject,
      stack,
    );
  };
  reportingWindowException = true;
  try {
    const event = new ErrorEvent('error', { ...details, error, cancelable: true });
    // Preserve thrown undefined and object identity without inspecting stack.
    errorEventSlots.get(event).error = error;
    if (dispatchEventCore(window, event, true, true)) report(details, error);
    for (const deferred of deferredWindowExceptions) report(deferred.details, deferred.error);
  } finally {
    deferredWindowExceptions.length = 0;
    reportingWindowException = false;
  }
}
host.installWindowErrorReporter(reportWindowException);
bootstrapRestoreHooks.push(() => host.installWindowErrorReporter(reportWindowException));
