import http from 'node:http';
import { pathToFileURL } from 'node:url';

// A local app so the examples do not depend on a live site's availability.
export async function startFixture(port = 3000) {
  const server = http.createServer((req, res) => {
    if (req.url === '/api/quote') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ price: 42, currency: 'USD' }));
      return;
    }
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    res.end(`<!doctype html><html><head><title>Mimic demo shop</title></head>
      <body><h1>Demo shop</h1><label for="quantity">Quantity</label>
      <input id="quantity" type="text" value="1">
      <button id="calculate">Calculate total</button><output id="total">Ready</output>
      <script>
        document.querySelector('#calculate').addEventListener('click', async () => {
          const quote = await fetch('/api/quote').then(response => response.json());
          document.querySelector('#total').textContent =
            (Number(document.querySelector('#quantity').value) * quote.price) + ' ' + quote.currency;
        });
      </script></body></html>`);
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(port, '127.0.0.1', resolve);
  });
  return { url: `http://127.0.0.1:${server.address().port}`, close: () => new Promise(resolve => server.close(resolve)) };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const fixture = await startFixture(Number(process.env.PORT || 3000));
  console.log(`Demo shop: ${fixture.url}`);
}
