"""Unit tests plus opt-in real Chrome normalization checks.

COMPAT_FUZZ_CHROME_CDP=http://127.0.0.1:19333 enables browser tests.
"""
import os
import unittest

from fuzz_probes import ROOTS, generate_probes, probe, render_probe, surface_probes, discovery_expression


class GenerationTests(unittest.TestCase):
    def test_bounded_surface_covers_every_root_before_properties(self):
        discovery = {root: ['z', 'a'] for root in ROOTS}
        cases = surface_probes(discovery)
        self.assertEqual([p['id'] for p in cases[:7]], ['surface.' + root for root in ROOTS])
        self.assertEqual([p['id'] for p in cases[7:14]], ['descriptor.' + root + '.a' for root in ROOTS])
        self.assertEqual([p['id'] for p in cases[14:21]], ['property.' + root + '.a' for root in ROOTS])

    def test_unique_ids_and_required_corpora(self):
        cases = generate_probes()
        ids = [p['id'] for p in cases]
        self.assertEqual(len(ids), len(set(ids)))
        self.assertIn('legacy.document-all', ids)
        self.assertIn('realm.cross-origin', ids)
        self.assertIn('realm.detached-iframe', ids)
        self.assertIn('timing.mutation-observer', ids)
        self.assertTrue(all(isinstance(p['setup'], list) for p in cases))

    def test_volatile_clock_values_excluded(self):
        ids = [p['id'] for p in surface_probes({'performance': ['timeOrigin', 'now']})]
        self.assertNotIn('property.performance.timeOrigin', ids)
        self.assertIn('property.performance.now', ids)


@unittest.skipUnless(os.getenv('COMPAT_FUZZ_CHROME_CDP'), 'set COMPAT_FUZZ_CHROME_CDP for real Chrome checks')
class ChromeNormalizerTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        from pyppeteer import connect
        from oracle import PINNED_PRODUCT, product
        endpoint = os.environ['COMPAT_FUZZ_CHROME_CDP']
        self.assertEqual(product(endpoint), PINNED_PRODUCT)
        self.browser = await connect(browserURL=endpoint, defaultViewport=None)
        self.page = await self.browser.newPage()
        await self.page.goto('about:blank')

    async def asyncTearDown(self):
        if getattr(self, 'page', None):
            await self.page.close()
        if getattr(self, 'browser', None):
            await self.browser.disconnect()

    async def evaluate(self, expression):
        result = await self.page.evaluate(render_probe(probe('test', 'test', expression)), force_expr=True)
        self.assertEqual(result['status'], 'ok', result)
        return result['value']

    async def test_html_dda_is_not_normalized_as_undefined(self):
        value = await self.evaluate('({missing:inspect(undefined),all:inspect(document.all)})')
        self.assertEqual(value['all']['type'], 'undefined')
        self.assertFalse(value['all']['strictUndefined'])
        self.assertTrue(value['all']['looseUndefined'])
        self.assertTrue(value['all']['looseNull'])
        self.assertFalse(value['all']['boolean'])
        self.assertEqual(value['all']['primitive'], {'type': 'undefined', 'tag': '[object HTMLAllCollection]'})
        self.assertEqual(value['missing']['primitive'], {'type': 'undefined'})

    async def test_special_primitives_remain_distinct_json(self):
        value = await self.evaluate('[undefined,null,NaN,Infinity,-Infinity,-0,0,1n,Symbol("x")].map(scalar)')
        self.assertEqual(value, [
            {'type': 'undefined'}, {'type': 'object', 'value': None},
            {'type': 'number', 'value': 'NaN'}, {'type': 'number', 'value': 'Infinity'},
            {'type': 'number', 'value': '-Infinity'}, {'type': 'number', 'value': '-0'},
            {'type': 'number', 'value': 0}, {'type': 'bigint', 'value': '1'},
            {'type': 'symbol', 'value': 'Symbol(x)'},
        ])

    async def test_exception_categories_and_descriptor(self):
        value = await self.evaluate('({brand:attempt(()=>Document.prototype.getElementById.call({},"x")),descriptor:descriptor(document,"all")})')
        self.assertEqual(value['brand'], {'exception': 'TypeError', 'class': '[object Error]', 'messageCategory': 'illegal-receiver'})
        self.assertEqual(value['descriptor']['kind'], 'accessor')
        self.assertEqual(value['descriptor']['get']['type'], 'function')

    async def test_discovery_and_realm_observation(self):
        discovery = await self.page.evaluate(discovery_expression(), force_expr=True)
        self.assertEqual(set(discovery), set(ROOTS))
        self.assertIn('all', discovery['document'])
        case = next(p for p in generate_probes() if p['id'] == 'realm.same-origin')
        result = await self.page.evaluate(render_probe(case), force_expr=True)
        self.assertEqual(result['status'], 'ok', result)
        self.assertTrue(result['value']['differentObject'])
        self.assertTrue(result['value']['instance'])
        self.assertFalse(result['value']['notMainInstance'])

    async def test_job_and_mutation_ordering(self):
        for name, expected in [('timing.jobs', ['microtask', 'promise', 'timeout']), ('timing.mutation-observer', ['promise-before', 'observer', 'microtask-after'])]:
            case = next(p for p in generate_probes() if p['id'] == name)
            result = await self.page.evaluate(render_probe(case), force_expr=True)
            self.assertEqual(result, {'status': 'ok', 'value': expected})


if __name__ == '__main__':
    unittest.main()
