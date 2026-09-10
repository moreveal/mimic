import unittest
from types import SimpleNamespace

from differential import cookie_metadata, differences, probe_errors, network_sequence, run_case


class DifferentialTest(unittest.TestCase):
    def test_missing_is_not_null(self):
        self.assertEqual(differences({}, {'x': None})[0]['missing'], 'chrome')
        self.assertEqual(differences({'x': None}, {})[0]['missing'], 'mimic')

    def test_json_numbers_and_booleans(self):
        self.assertEqual(differences(1, 1.0), [])
        self.assertTrue(differences(True, 1))
        self.assertTrue(differences(False, None))

    def test_order_and_pointer_escaping(self):
        self.assertEqual(differences({'a/b~': [1, 2]}, {'a/b~': [1, 3]})[0]['path'], '/a~1b~0/1')
        self.assertTrue(differences([1, 2], [2, 1]))
        self.assertTrue(differences([], [None]))

    def test_partition_is_observable(self):
        a = {'name': 'a', 'value': '1', 'expires': 10, 'partitionKey': {'topLevelSite': 'https://a.test', 'hasCrossSiteAncestor': False}}
        b = dict(a, expires=20)
        self.assertEqual(cookie_metadata([a]), cookie_metadata([b]))
        b['partitionKey'] = dict(a['partitionKey'], hasCrossSiteAncestor=True)
        self.assertTrue(differences(cookie_metadata([a]), cookie_metadata([b])))

    def test_probe_exception_is_incomplete(self):
        self.assertEqual(probe_errors({'observations': [{'exception': {'name': 'Error'}}, {'value': {'exception': 'expected result'}}]}), [0])

    def test_transport_reuse_and_redirect_order(self):
        base = {'method': 'POST', 'body': 'x', 'protocol': 'HTTP/1.1'}
        records = [dict(base, path='/redirect/307', connection=45), dict(base, path='/echo', connection=45)]
        self.assertEqual([r['connection'] for r in network_sequence(records)], [1, 1])
        records[1]['connection'] = 99
        self.assertEqual([r['connection'] for r in network_sequence(records)], [1, 2])


class CleanupTest(unittest.IsolatedAsyncioTestCase):
    async def test_setup_and_cleanup_failures_are_reported(self):
        class Client:
            async def call(self, method, params=None, session=None):
                if method == 'Target.createBrowserContext':
                    return {'browserContextId': 'owned-context'}
                raise RuntimeError(method + ' failed')
        result = await run_case(Client(), True, {}, SimpleNamespace(server_port=1))
        self.assertIn('Target.createTarget failed', result['harnessError'])
        self.assertIn('Target.disposeBrowserContext failed', result['harnessError'])
        self.assertEqual(result['partialObservations'], [])


if __name__ == '__main__':
    unittest.main()
