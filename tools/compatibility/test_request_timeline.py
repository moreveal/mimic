import json
import tempfile
import unittest
from pathlib import Path

from tools.compatibility.request_timeline import inventory


class RequestTimelineTests(unittest.TestCase):
    def test_sessions_and_reused_ids_are_not_conflated(self):
        def request(session, url):
            return {"sessionId": session, "method": "Network.requestWillBeSent", "params": {
                "requestId": "1", "timestamp": 10, "request": {"url": url, "method": "POST", "postData": "é"}}}
        first = request("a", "https://a.test/one")
        wrapped = {"method": "Target.receivedMessageFromTarget", "params": {
            "sessionId": "b", "message": json.dumps(request("b", "https://b.test/two"))}}
        extra = {"sessionId": "a", "method": "Network.requestWillBeSentExtraInfo", "params": {
            "requestId": "1", "headers": {"X-Only-A": "yes"}}}
        with tempfile.TemporaryDirectory() as tmp:
            Path(tmp, "events.json").write_text(json.dumps([first, wrapped, first, extra]), encoding="utf-8")
            result = inventory(tmp)["requests"]
        self.assertEqual([row["association"] for row in result], ["ambiguous-reused-request-id", "unique", "ambiguous-reused-request-id"])
        self.assertEqual(result[1]["requestExtraCandidates"], [])
        self.assertEqual(result[0]["requestBody"]["bytes"], 2)
        self.assertFalse(result[0]["browserObservationDependencies"]["known"])

    def test_missing_body_is_not_empty_body(self):
        event = {"method": "Network.requestWillBeSent", "params": {
            "requestId": "1", "request": {"url": "https://a.test/", "method": "POST"}}}
        with tempfile.TemporaryDirectory() as tmp:
            Path(tmp, "events.jsonl").write_text(json.dumps(event) + "\n", encoding="utf-8")
            row = inventory(tmp)["requests"][0]
        self.assertEqual(row["requestBody"], {"missing": True})
        self.assertNotIn("response", row)


if __name__ == "__main__":
    unittest.main()
