import assert from 'node:assert/strict';
import { once } from 'node:events';
import { spawn } from 'node:child_process';
import { resolve } from 'node:path';
import { chromium } from 'playwright-core';

const PLAYGROUND_URL = 'https://playground.ai.cloudflare.com/';
const PLAYGROUND_WS_BASE = 'wss://playground.ai.cloudflare.com/agents/playground/';
const DEFAULT_MODEL = '@cf/zai-org/glm-4.7-flash';
const DEFAULT_PROMPT = 'Reply with exactly: MIMIC_GATE_OK';
const STARTUP_TIMEOUT_MS = 30_000;
const NAVIGATION_TIMEOUT_MS = 45_000;
const CHAT_TIMEOUT_MS = 120_000;

// Protocol shape mirrored from OmniRoute's CloudflarePlaygroundExecutor:
// https://github.com/diegosouzapw/OmniRoute/blob/main/open-sse/executors/cloudflare-playground.ts

function withTimeout(promise, timeoutMs, label) {
  let timer;
  const timeout = new Promise((_, reject) => {
    timer = setTimeout(
      () => reject(new Error(`${label} timed out after ${timeoutMs} ms`)),
      timeoutMs,
    );
  });
  return Promise.race([promise, timeout]).finally(() => clearTimeout(timer));
}

async function startMimic(binary) {
  const child = spawn(resolve(binary), ['--listen', '127.0.0.1:0'], {
    cwd: resolve(import.meta.dirname, '../..'),
    windowsHide: true,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let output = '';
  child.stdout.on('data', (chunk) => {
    output += chunk;
  });
  child.stderr.on('data', (chunk) => {
    output += chunk;
  });
  const endpoint = await withTimeout(
    new Promise((resolveReady, reject) => {
      child.on('error', reject);
      child.on('exit', (code) => reject(new Error(`Mimic exited with ${code}:\n${output}`)));
      const inspect = () => {
        const match = output.match(/Mimic listening on (http:\/\/127\.0\.0\.1:\d+)/);
        if (match) resolveReady(match[1]);
        else setTimeout(inspect, 25);
      };
      inspect();
    }),
    STARTUP_TIMEOUT_MS,
    'Mimic startup',
  );
  return { child, endpoint, output: () => output };
}

async function runGate(endpoint) {
  const browser = await chromium.connectOverCDP(endpoint);
  try {
    const context = browser.contexts()[0];
    assert(context, 'Mimic did not expose a default browser context');
    const page = await context.newPage();
    try {
      const response = await page.goto(PLAYGROUND_URL, {
        waitUntil: 'domcontentloaded',
        timeout: NAVIGATION_TIMEOUT_MS,
      });
      assert(response, 'Cloudflare Playground navigation returned no response');
      assert.equal(response.ok(), true, `Cloudflare Playground returned HTTP ${response.status()}`);
      assert.doesNotMatch(
        await page.title(),
        /Attention Required/i,
        'Cloudflare blocked the session',
      );

      const result = await withTimeout(
        page.evaluate(
          ({ chatId, model, prompt, wsBase }) =>
            new Promise((resolveChat, rejectChat) => {
              const frames = [];
              let text = '';
              const pk = crypto.randomUUID();
              const room = `playground-${crypto.randomUUID().replace(/-/g, '').slice(0, 25)}`;
              let socket;
              try {
                socket = new WebSocket(`${wsBase}${room}?_pk=${pk}`);
              } catch (error) {
                rejectChat(
                  new Error(
                    `WebSocket constructor is not usable: ${error instanceof Error ? error.message : String(error)}`,
                  ),
                );
                return;
              }
              socket.onopen = () => {
                socket.send(JSON.stringify({ type: 'cf_agent_stream_resume_request' }));
                socket.send(
                  JSON.stringify({
                    type: 'rpc',
                    id: 'mimic-gate-config',
                    method: 'setConfig',
                    args: [{ model, temperature: 0, stream: true }],
                  }),
                );
                socket.send(
                  JSON.stringify({
                    id: chatId,
                    init: {
                      method: 'POST',
                      body: JSON.stringify({
                        messages: [
                          { role: 'user', parts: [{ type: 'text', text: prompt }], id: 'm1' },
                        ],
                        trigger: 'submit-message',
                      }),
                    },
                    type: 'cf_agent_use_chat_request',
                  }),
                );
              };
              socket.onerror = () =>
                rejectChat(new Error('Cloudflare Playground WebSocket failed'));
              socket.onmessage = (event) => {
                const raw = String(event.data);
                frames.push(raw.slice(0, 500));
                let message;
                try {
                  message = JSON.parse(raw);
                } catch {
                  return;
                }
                if (message.type !== 'cf_agent_use_chat_response' || message.id !== chatId) return;
                if (message.error) {
                  socket.close();
                  rejectChat(new Error(`Cloudflare error: ${String(message.body)}`));
                  return;
                }
                if (message.done) {
                  socket.close();
                  resolveChat({ text, frames });
                  return;
                }
                try {
                  const body =
                    typeof message.body === 'string' ? JSON.parse(message.body) : message.body;
                  if (body?.type === 'text-delta' && typeof body.delta === 'string')
                    text += body.delta;
                } catch {}
              };
            }),
          {
            chatId: `mimic-gate-${crypto.randomUUID()}`,
            model: process.env.CLOUDFLARE_PLAYGROUND_MODEL || DEFAULT_MODEL,
            prompt: process.env.CLOUDFLARE_PLAYGROUND_PROMPT || DEFAULT_PROMPT,
            wsBase: PLAYGROUND_WS_BASE,
          },
        ),
        CHAT_TIMEOUT_MS,
        'Cloudflare Playground chat',
      );
      assert(result.frames.length > 0, 'Cloudflare returned no cf_agent frames');
      assert(result.text.trim(), 'Cloudflare returned an empty assistant response');
      return result;
    } finally {
      await page.close();
    }
  } finally {
    await browser.close();
  }
}

const explicitEndpoint = process.env.MIMIC_URL;
const binary = process.argv[2] || process.env.MIMIC_BINARY;
let runtime;
try {
  if (!explicitEndpoint && !binary) {
    throw new Error('Set MIMIC_URL or pass a Mimic binary path as the first argument');
  }
  runtime = explicitEndpoint ? null : await startMimic(binary);
  const endpoint = explicitEndpoint || runtime.endpoint;
  const result = await runGate(endpoint);
  console.log(`PASS Cloudflare Playground via Mimic: ${JSON.stringify(result.text)}`);
} catch (error) {
  if (runtime) console.error(runtime.output());
  console.error(`FAIL Cloudflare Playground via Mimic: ${error.stack || error}`);
  process.exitCode = 1;
} finally {
  if (runtime?.child.exitCode === null && runtime.child.signalCode === null) {
    runtime.child.kill('SIGTERM');
    await once(runtime.child, 'exit');
  }
}
