(async () => {
  const rows = [];
  for (const status of [0, 301, 302, 303, 307, 308]) {
    const response = await fetch(status ? '/redirect/' + status : '/echo', {
      method: 'POST', headers: {'X-Corpus': 'redirect', 'Content-Type': 'text/plain'}, body: 'body=a&b'
    });
    rows.push({status, redirected: response.redirected, url: response.url, received: await response.json()});
  }
  return rows;
})()
