import copy
import json
from pathlib import Path
import tempfile
import unittest

from readme_charts import load_results


class ChartInputTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name) / "raw.json"
        self.complete = {
            "finished": "2026-09-13T20:00:00Z", "checkpoint": "2026-09-13",
            "startup": {s: {"cdp_ready_ms": {"n": 10}} for s in ("mimic", "chrome")},
            "concurrency": [{"system": s, "workload": "static", "n": n, "stop": "", "success_rate": 1}
                            for s in ("mimic", "chrome") for n in (1, 5, 10, 25, 50, 100)],
        }

    def write(self, value):
        self.path.write_text(json.dumps(value), encoding="utf-8")

    def test_complete_export_is_accepted(self):
        self.write(self.complete)
        self.assertEqual(load_results(self.path), self.complete)

    def test_incomplete_and_failed_series_are_rejected(self):
        for defect in ("unfinished", "failure", "missing_level", "startup_count"):
            with self.subTest(defect=defect):
                data = copy.deepcopy(self.complete)
                if defect == "unfinished":
                    del data["finished"]
                elif defect == "failure":
                    data["concurrency"][-1]["success_rate"] = .99
                elif defect == "missing_level":
                    data["concurrency"].pop()
                else:
                    data["startup"]["mimic"]["cdp_ready_ms"]["n"] = 1
                self.write(data)
                with self.assertRaises(ValueError):
                    load_results(self.path)

    def test_smoke_and_tampered_raw_are_rejected(self):
        self.write({"finished": "done", "metadata": {"arguments": {"smoke": True}}})
        with self.assertRaisesRegex(ValueError, "Smoke"):
            load_results(self.path)
        self.write({"finished": "done", "metadata": {"arguments": {"smoke": False}}})
        (self.path.parent / "summary.json").write_text("{}")
        (self.path.parent / "manifest.json").write_text(json.dumps({"sha256": {"raw.json": "wrong"}}))
        with self.assertRaisesRegex(ValueError, "hash mismatch"):
            load_results(self.path)


if __name__ == "__main__":
    unittest.main()
