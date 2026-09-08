"""Compare Mimic's effective Window own-property surface with a pinned capture."""

import argparse
import asyncio
import json
from pathlib import Path

from pyppeteer import connect


EXPRESSION = """() => Object.getOwnPropertyNames(window).sort().map(name => {
  const descriptor = Object.getOwnPropertyDescriptor(window, name);
  let valueType = 'accessor', functionName = null, functionLength = null;
  if ('value' in descriptor) {
    valueType = typeof descriptor.value;
    if (valueType === 'function') {
      functionName = descriptor.value.name;
      functionLength = descriptor.value.length;
    }
  }
  return {
    name,
    enumerable: !!descriptor.enumerable,
    configurable: !!descriptor.configurable,
    writable: 'writable' in descriptor ? !!descriptor.writable : null,
    getter: typeof descriptor.get === 'function',
    setter: typeof descriptor.set === 'function',
    valueType,
    functionName,
    functionLength
  };
})"""


async def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", default="http://127.0.0.1:19222")
    parser.add_argument("--url", default="https://example.com/")
    parser.add_argument("--expected", default="chrome/152/generated/window-secure.json")
    args = parser.parse_args()
    expected_capture = json.loads(Path(args.expected).read_text(encoding="utf-8"))
    expected = {item["name"]: item for item in expected_capture["properties"]}
    browser = await connect(browserURL=args.endpoint, defaultViewport=None)
    page = await browser.newPage()
    try:
        await page.goto(args.url, {"waitUntil": "load", "timeout": 30_000})
        actual = {item["name"]: item for item in await page.evaluate(EXPRESSION)}
    finally:
        await page.close()
        await browser.disconnect()
    mismatches = []
    for name in sorted(expected.keys() | actual.keys()):
        if expected.get(name) != actual.get(name):
            mismatches.append({"name": name, "expected": expected.get(name), "actual": actual.get(name)})
    print(json.dumps({
        "expectedCount": len(expected),
        "actualCount": len(actual),
        "matched": len(expected) - len(mismatches),
        "mismatchCount": len(mismatches),
        "mismatches": mismatches,
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    asyncio.run(main())
