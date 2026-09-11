import unittest
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


if __name__ == "__main__":
    unittest.main()
