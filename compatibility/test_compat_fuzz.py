"""Pure harness regressions: no running browser or oracle is required."""
import copy
import os
import unittest

from compat_fuzz import Endpoint, MISSING, at_path, differences, reduce_setup, same


class TypedDiffTests(unittest.TestCase):
    def test_same_uses_typed_nested_equality(self):
        self.assertFalse(same({'value': [False]}, {'value': [0]}))
        self.assertFalse(same({'value': [True]}, {'value': [1]}))
        self.assertFalse(same({}, {'value': None}))
        self.assertTrue(same({'a': [False, 0], 'b': None},
                             {'b': None, 'a': [False, 0]}))

    def test_missing_is_not_null_in_either_direction(self):
        self.assertEqual(list(differences({}, {'value': None})),
                         [(('value',), MISSING, None)])
        self.assertEqual(list(differences({'value': None}, {})),
                         [(('value',), None, MISSING)])

    def test_boolean_and_number_are_distinct(self):
        self.assertEqual(list(differences({'x': [False]}, {'x': [0]})),
                         [(('x', 0), False, 0)])
        self.assertEqual(list(differences(True, 1)), [((), True, 1)])

    def test_object_insertion_order_is_ignored(self):
        self.assertEqual(list(differences({'a': 1, 'b': None},
                                          {'b': None, 'a': 1})), [])

    def test_array_order_is_observable(self):
        self.assertEqual(list(differences(['a', 'b'], ['b', 'a'])),
                         [((0,), 'a', 'b'), ((1,), 'b', 'a')])

    def test_array_length_difference_projects_whole_array(self):
        self.assertEqual(list(differences({'keys': ['a']}, {'keys': []})),
                         [(('keys',), ['a'], [])])

    def test_paths_are_deterministic_and_projectable(self):
        left = {'z': 0, 'a': {'items': [1, None]}}
        right = {'a': {'items': [1, False]}, 'z': 2}
        result = list(differences(left, right))
        self.assertEqual([path for path, _, _ in result],
                         [('a', 'items', 1), ('z',)])
        for path, expected, actual in result:
            self.assertEqual(at_path(left, path), expected)
            self.assertEqual(at_path(right, path), actual)

    def test_projection_handles_missing_nested_values(self):
        for value, path in [({}, ('absent',)), ([], (0,)),
                            ({'a': None}, ('a', 'b'))]:
            self.assertEqual(at_path(value, path), MISSING)
        self.assertIsNone(at_path({'a': None}, ('a',)))


class ReducerTests(unittest.IsolatedAsyncioTestCase):
    async def test_dependent_setup_preserves_exact_signature_and_is_one_minimal(self):
        # A tiny independent state machine models a receiver and a mutation.
        # Removing its constructor invalidates a later dependent operation;
        # removing the mutation still diverges, but with a different signature.
        def observe(candidate):
            state = {}
            for statement in candidate['setup']:
                if statement == 'construct':
                    state['receiver'] = {'length': 0}
                elif statement == 'append':
                    if 'receiver' not in state:
                        return {'exception': 'ReferenceError'}, {'exception': 'ReferenceError'}
                    state['receiver']['length'] += 1
            if 'receiver' not in state:
                return None, None
            size = state['receiver']['length']
            return {'length': size}, {'length': size + 1}

        probe = {'id': 'dependent', 'setup': ['noise-a', 'construct', 'noise-b', 'append', 'noise-c'],
                 'expression': 'observe receiver', 'category': 'legacy'}
        original = copy.deepcopy(probe)
        signature = observe(probe)

        async def preserves(candidate):
            pair = observe(candidate)
            return all(not list(differences(a, b)) for a, b in zip(pair, signature))

        reduced = await reduce_setup(probe, preserves)
        self.assertEqual(reduced['setup'], ['construct', 'append'])
        self.assertEqual(observe(reduced), signature)
        self.assertEqual(probe, original, 'reduction must not mutate the input')
        self.assertEqual(reduced['expression'], original['expression'])
        for i in range(len(reduced['setup'])):
            candidate = dict(reduced, setup=reduced['setup'][:i] + reduced['setup'][i + 1:])
            self.assertFalse(await preserves(candidate))

    async def test_fixed_point_reconsiders_earlier_statements(self):
        # Nonmonotonic predicate: deleting b first makes a removable. A single
        # forward deletion pass would leave a behind.
        accepted = {('a', 'b', 'c'), ('a', 'c'), ('c',)}

        async def preserves(candidate):
            return tuple(candidate['setup']) in accepted

        reduced = await reduce_setup({'setup': ['a', 'b', 'c']}, preserves)
        self.assertEqual(reduced['setup'], ['c'])

    async def test_all_irrelevant_setup_can_be_deleted(self):
        async def preserves(_):
            return True

        self.assertEqual((await reduce_setup({'setup': ['x', 'y', 'z']}, preserves))['setup'], [])

    async def test_indispensable_setup_and_empty_input(self):
        async def preserves(_):
            return False

        for setup in ([], ['required']):
            self.assertEqual((await reduce_setup({'setup': setup}, preserves))['setup'], setup)


@unittest.skipUnless(os.environ.get('COMPAT_FUZZ_CHROME_CDP'),
                     'set COMPAT_FUZZ_CHROME_CDP to opt into a live Chrome test')
class EndpointLiveTests(unittest.IsolatedAsyncioTestCase):
    async def test_timeout_leaves_endpoint_usable(self):
        # Endpoint creates and closes its own targets; existing application
        # targets are never selected, navigated, or evaluated by this test.
        endpoint = Endpoint(os.environ['COMPAT_FUZZ_CHROME_CDP'], 'about:blank', 2)
        with self.assertRaises(TimeoutError):
            await endpoint.evaluate('new Promise(() => {})')
        self.assertEqual(await endpoint.evaluate('42'), 42)


if __name__ == '__main__':
    unittest.main()
