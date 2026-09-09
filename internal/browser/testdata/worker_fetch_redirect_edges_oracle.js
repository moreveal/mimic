self.onmessage = async event => {
  const base = event.data.base, result = [];
  for (const kind of ['missing', 'empty', 'credentials']) {
    for (const redirect of ['follow', 'manual', 'error']) {
      try {
        const response = await fetch(base + '/edge?kind=' + kind + '&mode=' + redirect, {redirect});
        result.push({kind, redirect, status: response.status, type: response.type, url: response.url, redirected: response.redirected, body: await response.text()});
      } catch (error) { result.push({kind, redirect, error: {name: error.name, message: error.message}}); }
    }
  }
  postMessage(result);
};
