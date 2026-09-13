import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from prepare import digest, validate_document_links
from publish import CHECKS, require_ci, verified_archive


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

    def test_missing_running_or_failed_ci_rejected(self):
        for runs in ([], [{'status': 'in_progress', 'conclusion': '', 'databaseId': 1}],
                     [{'status': 'completed', 'conclusion': 'failure', 'databaseId': 1}]):
            with self.subTest(runs=runs), patch('publish.run', return_value=json.dumps(runs)):
                with self.assertRaisesRegex(RuntimeError, 'CI must pass'):
                    require_ci('source')

    def test_successful_ci_checks_exact_source(self):
        with patch('publish.run', return_value=json.dumps([
                {'status': 'completed', 'conclusion': 'success', 'databaseId': 42}])) as command:
            self.assertEqual(require_ci('exact-source'), 42)
            self.assertIn('exact-source', command.call_args.args)

    def test_bundled_document_link_cannot_be_missing_or_escape(self):
        doc = self.root / 'README.md'
        doc.write_text('[Results](benchmarks/results.json)')
        with self.assertRaisesRegex(RuntimeError, 'Missing bundled document link'):
            validate_document_links(self.root)
        (self.root / 'benchmarks').mkdir()
        (self.root / 'benchmarks/results.json').write_text('{}')
        validate_document_links(self.root)
        doc.write_text('[Outside](../)')
        with self.assertRaisesRegex(RuntimeError, 'Missing bundled document link'):
            validate_document_links(self.root)


if __name__ == '__main__':
    unittest.main()
