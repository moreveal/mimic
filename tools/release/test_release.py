import json
from pathlib import Path
import tempfile
import unittest

from prepare import digest
from publish import CHECKS, verified_archive


class PublicationGateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.archive = self.root / 'mimic-v0.1.0-beta.1-windows-amd64.zip'
        self.archive.write_bytes(b'checked archive bytes')
        self.receipt = {
            'version': 'v0.1.0-beta.1', 'platform': 'windows-amd64',
            'sourceRevision': 'source', 'overviewRevision': 'overview',
            'verified': True, 'checks': list(CHECKS), 'archive': self.archive.name,
            'sha256': digest(self.archive), 'size': self.archive.stat().st_size,
        }

    def check(self):
        (self.root / 'windows-amd64.receipt.json').write_text(json.dumps(self.receipt))
        return verified_archive(self.root, 'v0.1.0-beta.1', 'windows', 'source', 'overview')

    def test_verified_matching_archive(self):
        self.assertEqual(self.check()[0], self.archive)

    def test_modified_archive_rejected(self):
        self.archive.write_bytes(b'changed archive bytes')
        with self.assertRaisesRegex(RuntimeError, 'Archive changed'):
            self.check()

    def test_stale_source_or_examples_and_missing_checks_rejected(self):
        for key, value in [('sourceRevision', 'old'), ('overviewRevision', 'old'),
                           ('verified', False), ('checks', ['runtimecheck:v8'])]:
            with self.subTest(key=key):
                original = self.receipt[key]
                self.receipt[key] = value
                with self.assertRaisesRegex(RuntimeError, 'Stale or unverified'):
                    self.check()
                self.receipt[key] = original

    def test_outside_archive_path_rejected(self):
        self.receipt['archive'] = '../private-source.zip'
        with self.assertRaisesRegex(RuntimeError, 'Unexpected archive name'):
            self.check()


if __name__ == '__main__':
    unittest.main()
