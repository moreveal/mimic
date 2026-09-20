import tempfile
import unittest
from pathlib import Path

from browser_test_policy import violations


class BrowserTestPolicyTests(unittest.TestCase):
    def check(self, source):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "sample_test.go"
            path.write_text("package browser\nimport \"testing\"\n" + source, encoding="utf-8")
            return violations(Path(directory))

    def test_accepts_explicit_parallel_and_serial_tests(self):
        errors = self.check("""
func TestParallel(t *testing.T) { parallelBrowserTest(t); runPage(t) }
func TestSerial(t *testing.T) { serialBrowserTest(t); t.Setenv("KEY", "value") }
""")
        self.assertEqual(errors, [])

    def test_rejects_missing_or_duplicate_marker(self):
        errors = self.check("""
func TestMissing(t *testing.T) { runPage(t) }
func TestDuplicate(t *testing.T) { serialBrowserTest(t); parallelBrowserTest(t) }
""")
        self.assertEqual(len(errors), 2)

    def test_rejects_process_mutation_in_parallel_test(self):
        errors = self.check("""
func TestUnsafe(t *testing.T) { parallelBrowserTest(t); t.Setenv("KEY", "value") }
""")
        self.assertEqual(len(errors), 1)


if __name__ == "__main__":
    unittest.main()
