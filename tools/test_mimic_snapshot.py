import unittest
import binascii
from pathlib import PurePosixPath
from mimic_snapshot import NavigationProgress, decode_files


class NavigationProgressTests(unittest.TestCase):
    def test_redirect_and_subframe_status(self):
        progress = NavigationProgress("main")
        def response(frame, status):
            progress.response({"requestId": "navigation", "type": "Document", "frameId": frame, "response": {"status": status}})
        response("main", 301)
        response("main", 200)
        response("child", 401)
        self.assertEqual(progress.status, 200)

    def test_new_main_document_resets_lifecycle(self):
        progress = NavigationProgress("main")
        progress.dom_content_loaded = progress.load_fired = True
        progress.request({"requestId": "new", "type": "Document", "frameId": "main"})
        self.assertFalse(progress.dom_content_loaded)
        self.assertFalse(progress.load_fired)
        progress.dom_content_loaded = True
        progress.request({"requestId": "child", "type": "Document", "frameId": "child"})
        self.assertTrue(progress.dom_content_loaded)

    def test_redirect_request_id_can_finish_again(self):
        progress = NavigationProgress("main")
        for _ in range(2):
            progress.request({"requestId": "same"})
            progress.finished({"requestId": "same", "encodedDataLength": 10})
        self.assertEqual(progress.pending, set())
        self.assertEqual(progress.transferred, 20)

    def test_snapshot_paths_cannot_escape_output(self):
        for path in ["../outside", "/absolute", "C:/drive", "assets\\outside"]:
            with self.subTest(path=path), self.assertRaises(ValueError):
                decode_files({"files": {path: ""}})

    def test_empty_snapshot_assets_from_current_and_older_servers(self):
        files = decode_files({"files": {"empty": "", "legacy-empty": None,
                                        "index.html": "b2s="}})
        self.assertEqual(files, {PurePosixPath("empty"): b"",
                                 PurePosixPath("legacy-empty"): b"",
                                 PurePosixPath("index.html"): b"ok"})

    def test_invalid_asset_data_is_not_treated_as_empty(self):
        for value in [0, False, [], {}, "%%%"]:
            with self.subTest(value=value), self.assertRaises((TypeError, ValueError, binascii.Error)):
                decode_files({"files": {"asset": value}})


if __name__ == "__main__":
    unittest.main()
