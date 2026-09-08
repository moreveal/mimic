import unittest

from oracle_differential import classify


class ClassificationTests(unittest.TestCase):
    def test_invariant_and_divergence(self):
        self.assertEqual(classify({}, {"value": 1}, {"value": 1}, {"value": 1}), "invariant")
        self.assertEqual(classify({}, {"value": 1}, {"value": 1}, {"value": 2}), "Mimic divergence")

    def test_mode_and_environment_specific(self):
        self.assertEqual(classify({}, {"value": 1}, {"value": 2}, {"value": 1}), "headless-specific")
        self.assertEqual(classify({"oracleScope": "environment"}, {"value": 1}, {"value": 2}, {"value": 1}), "environment-specific")
        self.assertEqual(classify({"oracleScope": "environment"}, {"value": 1}, {"value": 1}, {"value": 2}), "environment-specific")


if __name__ == "__main__":
    unittest.main()
