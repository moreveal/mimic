# Frequently asked questions

## What is Mimic?

A browser execution runtime for workflows that need JavaScript and browser state
without rendered pixels. It occupies the space between raw HTTP clients and full browsers.

## Is it headless Chrome?

Mimic uses V8 for JavaScript execution, but does not embed or launch Chromium.
It implements the browser behavior needed by its supported workflows.

## Can I use my existing automation tools?

Mimic exposes a subset of the Chrome DevTools Protocol. Selected workflows have
been exercised with Puppeteer, Pyppeteer, and Playwright tooling. Support depends
on the commands and behavior your workflow uses; it is not a universal drop-in
replacement. Share your workflow when requesting beta access.

## What can I get out of a page?

Evaluated JavaScript values, DOM content, and static DOM snapshots with available
assets. A DOM snapshot is not a screenshot or a complete offline copy of an application.

## Does it render pages?

Mimic does not produce page pixels or require a GPU. Screenshots, rendered PDFs,
video playback, full CSS layout, and Canvas/WebGL pixel rendering are outside the
current scope. Use a full browser when the visual output is the result you need.

## Does every website work?

Compatibility is still evolving. JavaScript execution alone does not establish
browser compatibility: supported APIs, loading behavior, and interactions all
matter. The local benchmark fixtures cover a limited set of behaviors and are
not a website compatibility score.

## Is it faster or smaller than Chrome?

Mimic is actively being developed toward direct HTTP lightness with browser
compatibility. The current measurements already show a memory advantage in
supported workloads. Latency still has substantial work ahead: slower cases
are optimization targets, not the performance level we're aiming to stop at.

See the [dated benchmark results](BENCHMARKS.md). Startup, task completion,
concurrency, CPU, and memory measure different things. Some workloads can favor
Chrome. We do not claim a universal speedup or infer production operating costs
from synthetic fixtures.

## Which platforms are supported?

Current builds and validation target Windows x64. Linux and additional Chrome
behavioral targets are development directions. There is no announced release date.

## Is it open source? Where can I download it?

The runtime is private. This repository contains product information and benchmark
results, with no source distribution or public binaries. [Request private beta access](BETA.md).

## Can I use it to run untrusted code?

Mimic is not a security sandbox. Its CDP endpoint is intended for trusted local
clients, and independent pages are not a security boundary for hostile tenants.

## Does it guarantee access to a website?

Recorded development sessions have reached the success page on a Cloudflare
challenge laboratory, received a BrowserScan Normal verdict, and captured Amazon
storefront content. These are encouraging compatibility checkpoints, not a
measured success rate across anti-bot products. The cause of admission is not
isolated by those observations.

Site policies, authentication, rate limits, and server decisions still apply.
Compatibility with browser behavior does not guarantee acceptance by any service.
