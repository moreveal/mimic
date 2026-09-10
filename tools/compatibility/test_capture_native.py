import asyncio
import importlib.util
import io
import json
import pathlib
import tempfile
import unittest
from unittest.mock import patch


class CaptureTests(unittest.IsolatedAsyncioTestCase):
    def setUp(self):
        path = pathlib.Path(__file__).with_name('capture_native.py')
        spec = importlib.util.spec_from_file_location('native_capture_test', path)
        self.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.module)

    async def test_initialization_readiness_waits_for_last_response(self):
        m = self.module
        m.main_target = 'owned'
        m.main_ready = asyncio.get_running_loop().create_future()
        resume = asyncio.Event()

        async def call(method, params, session):
            if method == 'Runtime.runIfWaitingForDebugger':
                await resume.wait()
            return {'result': {}}

        m.call = call
        task = asyncio.create_task(m.initialize({
            'sessionId': 'session', 'targetInfo': {'targetId': 'owned', 'type': 'page'}}))
        await asyncio.sleep(0)
        self.assertFalse(m.main_ready.done())
        resume.set()
        await task
        self.assertEqual(await m.main_ready, 'session')

    async def test_protocol_errors_are_indexed(self):
        m = self.module

        class Socket:
            async def send(self, text):
                request = json.loads(text)
                m.pending[request['id']].set_result({'id': request['id'], 'error': {
                    'code': -32000, 'message': 'missing body'}})

        m.ws = Socket()
        response = await m.call('Network.getResponseBody', {}, 'session')
        self.assertIn('error', response)
        self.assertEqual(m.errors[0]['method'], 'Network.getResponseBody')
        self.assertEqual(m.pending, {})

    async def test_preload_runs_in_frames_before_resume_and_not_workers(self):
        m = self.module
        m.preload_source = "/* diagnostic */"
        calls = []

        async def call(method, params, session):
            calls.append((session, method, params))
            return {'result': {}}

        m.call = call
        for kind in ('iframe', 'worker'):
            await m.initialize({'sessionId': kind, 'targetInfo': {'targetId': kind, 'type': kind}})
        frame = [item for item in calls if item[0] == 'iframe']
        self.assertEqual(frame[-2][1], 'Page.addScriptToEvaluateOnNewDocument')
        self.assertTrue(frame[-2][2]['runImmediately'])
        self.assertEqual(frame[-1][1], 'Runtime.runIfWaitingForDebugger')
        self.assertFalse(any(item[1].startswith('Page.') for item in calls if item[0] == 'worker'))

    async def test_preload_failure_still_resumes_owned_target(self):
        m = self.module
        m.preload_source = "/* diagnostic */"
        calls = []

        async def call(method, params, session):
            calls.append(method)
            return {} if method == 'Page.addScriptToEvaluateOnNewDocument' else {'result': {}}

        m.call = call
        with self.assertRaises(RuntimeError):
            await m.initialize({'sessionId': 'frame', 'targetInfo': {'targetId': 'frame', 'type': 'iframe'}})
        self.assertEqual(calls[-1], 'Runtime.runIfWaitingForDebugger')

    async def test_reused_script_id_preserves_both_navigation_sources(self):
        m = self.module
        source = 'first document'

        async def call(method, params, session):
            return {'result': {'scriptSource': source}}

        m.call = call
        with tempfile.TemporaryDirectory() as directory:
            m.OUT = pathlib.Path(directory)
            await m.script('session', {'scriptId': '3', 'executionContextId': 1,
                                      'hash': 'first', 'url': 'https://example.test/'})
            source = 'second document'
            await m.script('session', {'scriptId': '3', 'executionContextId': 2,
                                      'hash': 'second', 'url': 'https://example.test/'})
            files = [entry['file'] for entry in m.scripts]
            self.assertNotEqual(files[0], files[1])
            self.assertEqual([json.loads((m.OUT / file).read_text())['result']['scriptSource']
                              for file in files], ['first document', 'second document'])

    async def test_drain_includes_tasks_spawned_during_wait(self):
        m = self.module
        done = []

        async def second():
            await asyncio.sleep(0)
            done.append('second')

        async def first():
            await asyncio.sleep(0)
            m.spawn(second())

        m.spawn(first())
        await m.drain_tasks(1)
        self.assertEqual(done, ['second'])
        self.assertFalse(m.tasks)

    async def test_partial_setup_failure_disposes_only_owned_context(self):
        m = self.module
        calls = []

        async def call(method, params=None, session=None):
            calls.append((method, params))
            if method == 'Target.createBrowserContext':
                return {'result': {'browserContextId': 'owned'}}
            if method == 'Target.createTarget':
                return {'error': {'message': 'failed'}}
            return {'result': {}}

        class Connection:
            async def __aenter__(self):
                return self

            async def __aexit__(self, *args):
                pass

        async def reader():
            await asyncio.Future()

        m.call, m.reader = call, reader
        with tempfile.TemporaryDirectory() as directory:
            m.OUT, m.PORT, m.URL = pathlib.Path(directory), 1234, 'https://example.test'
            m.events = (m.OUT / 'events.jsonl').open('w')
            with patch.object(m.urllib.request, 'urlopen', return_value=io.StringIO(
                    '{"webSocketDebuggerUrl":"ws://localhost/mock"}')), \
                    patch.object(m.websockets, 'connect', return_value=Connection()):
                await m.main()
            self.assertIn(('Target.disposeBrowserContext', {'browserContextId': 'owned'}), calls)
            self.assertTrue(m.events.closed)
            manifest = json.loads((m.OUT / 'manifest.json').read_text())
            self.assertTrue(manifest['errors'])
            self.assertTrue(manifest['limitations'])


if __name__ == '__main__':
    unittest.main()
