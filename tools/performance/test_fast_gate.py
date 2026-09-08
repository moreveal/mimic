import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
import fast_gate


class ProvenanceTests(unittest.TestCase):
    def test_rejects_changed_executable_before_launch(self):
        with tempfile.TemporaryDirectory() as directory:
            binary=Path(directory)/'mimic.exe'
            binary.write_bytes(b'just-built')
            expected=fast_gate.frozen.digest(binary)
            self.assertEqual(fast_gate.verify_binary(binary,expected),expected)
            binary.write_bytes(b'stale-or-replaced')
            with self.assertRaisesRegex(RuntimeError,'SHA-256 mismatch'):
                fast_gate.verify_binary(binary,expected)


if __name__=='__main__':unittest.main()
