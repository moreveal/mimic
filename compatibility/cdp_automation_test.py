"""Regression checks for reporting correctness; browser behavior uses real clients."""
import json
from pathlib import Path
import tempfile
import unittest

from cdp_automation import WireRecorder, compare


class WireRecorderTest(unittest.TestCase):
    def test_nested_and_flattened_sessions_do_not_alias_request_ids(self):
        with tempfile.TemporaryDirectory() as directory:
            recorder = WireRecorder("unused", Path(directory) / "trace.jsonl")
            recorder.observe("send", {"id": 1, "method": "Browser.getVersion"}, 1)
            recorder.observe("send", {"id": 2, "method": "Target.sendMessageToTarget", "params": {
                "sessionId": "legacy", "message": json.dumps({"id": 1, "method": "Runtime.evaluate"})}}, 1)
            recorder.observe("send", {"id": 1, "method": "Runtime.callFunctionOn", "sessionId": "flat"}, 1)
            recorder.observe("receive", {"id": 1, "result": {}}, 1)
            recorder.observe("receive", {"id": 2, "result": {}}, 1)
            recorder.observe("receive", {"method": "Target.receivedMessageFromTarget", "params": {
                "sessionId": "legacy", "message": json.dumps({"id": 1, "result": {}})}}, 1)
            recorder.observe("receive", {"id": 1, "sessionId": "flat", "error": {
                "code": -32000, "message": "Real protocol failure"}}, 1)
            summary = recorder.finish()
            methods = summary["methods"]
            self.assertEqual(methods["Browser.getVersion"]["responses"], 1)
            self.assertEqual(methods["Runtime.evaluate"]["responses"], 1)
            self.assertEqual(methods["Runtime.callFunctionOn"]["errors"][0]["code"], -32000)
            self.assertTrue(all(row["unanswered"] == 0 for row in methods.values()))
            self.assertEqual(summary["events"]["Target.receivedMessageFromTarget"], 1)

    def test_connection_scope_and_unanswered_calls_are_retained(self):
        with tempfile.TemporaryDirectory() as directory:
            recorder = WireRecorder("unused", Path(directory) / "trace.jsonl")
            recorder.observe("send", {"id": 1, "method": "Page.navigate"}, 1)
            recorder.observe("send", {"id": 1, "method": "Page.reload"}, 2)
            recorder.observe("receive", {"id": 1, "result": {}}, 2)
            methods = recorder.finish()["methods"]
            self.assertEqual(methods["Page.navigate"]["unanswered"], 1)
            self.assertEqual(methods["Page.reload"]["responses"], 1)


class ComparisonTest(unittest.TestCase):
    def report(self, *checks):
        return {"clients": {"pyppeteer": {"checks": list(checks)}}}

    def test_shared_failures_are_never_compatible_passes(self):
        failure = {"name": "case", "status": "fail", "error": "same error"}
        result = compare(self.report(failure), self.report(failure))
        self.assertEqual(result["summary"], {"reference-failure": 1})

    def test_blocked_work_and_different_values_remain_divergences(self):
        reference = self.report({"name": "case", "status": "pass", "observed": 42})
        blocked = compare(reference, self.report({"name": "case", "status": "blocked"}))
        changed = compare(reference, self.report({"name": "case", "status": "pass", "observed": 43}))
        self.assertEqual(blocked["summary"], {"divergence": 1})
        self.assertEqual(changed["summary"], {"divergence": 1})


if __name__ == "__main__":
    unittest.main()
