import json
from pathlib import Path
import tempfile
import unittest

from summarize_trace import read_events, summarize


class TraceSummaryTests(unittest.TestCase):
    def test_cdp_projection_and_unsupported_boundaries(self):
        result = summarize([
            {"sequence": 1, "kind": "cdp", "name": "event", "data": {
                "method": "Network.responseReceived", "params": {"type": "Document", "response": {
                    "url": "https://test/", "status": 403, "headers": {"Cf-Mitigated": "challenge"}}}}},
            {"kind": "semantic-missing", "name": "WebGL.compileShader", "data": {}},
        ])
        self.assertEqual(result["documentResponses"][0]["cfMitigated"], "challenge")
        self.assertEqual(result["unsupported"], {"WebGL.compileShader": 1})

    def test_caught_console_exception_is_not_lost(self):
        result = summarize([
            {"sequence": 1, "kind": "console", "name": "log", "data": {"args": ["normal"]}},
            {"sequence": 2, "kind": "console", "name": "error", "data": {"args": ["TypeError: undefined.call"]}},
            {"sequence": 3, "kind": "error", "name": "script", "data": {"message": "uncaught"}},
        ])
        self.assertEqual([d["sequence"] for d in result["diagnostics"]], [2, 3])
        self.assertEqual(result["eventCount"], 3)

    def test_native_redirect_and_application_404_do_not_imply_failure_or_success(self):
        result = summarize([
            {"method": "Network.requestWillBeSent", "sessionId": "page", "params": {
                "requestId": "1", "type": "Document", "request": {"url": "https://test/final"},
                "redirectResponse": {"url": "https://test/start", "status": 301}}},
            {"method": "Network.responseReceived", "sessionId": "page", "params": {
                "requestId": "1", "response": {"url": "https://test/final", "status": 404, "headers": {}}}},
            {"method": "Network.responseReceived", "sessionId": "worker", "params": {
                "requestId": "1", "response": {"url": "https://test/resource", "status": 403}}},
            {"method": "Runtime.consoleAPICalled", "params": {
                "type": "error", "args": [{"type": "object", "description": "TypeError: caught"}]}},
        ])
        self.assertEqual(len(result["documentResponses"]), 1)
        self.assertEqual(result["documentResponses"][0]["status"], 404)
        self.assertEqual(result["redirects"][0]["status"], 301)
        self.assertEqual(result["diagnostics"][0]["args"], ["TypeError: caught"])
        self.assertNotIn("success", result)

    def test_utf8_array_and_jsonl(self):
        records = [{"kind": "console", "name": "error", "data": {"args": ["ошибка"]}}]
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "trace.json"
            for content in (json.dumps(records, ensure_ascii=False),
                            json.dumps({"result": {"events": records}}, ensure_ascii=False),
                            json.dumps({"events": records}, ensure_ascii=False),
                            "\n".join(json.dumps(r, ensure_ascii=False) for r in records)):
                path.write_text(content, encoding="utf-8")
                self.assertEqual(read_events(path), records)


if __name__ == "__main__":
    unittest.main()
