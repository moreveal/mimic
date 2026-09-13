import unittest

from windows_tests import partition


class ShardCoverageTests(unittest.TestCase):
    def test_every_root_test_appears_in_exactly_one_shard(self):
        names = ['Test' + str(index) for index in range(661)] + ['ExamplePage', 'FuzzParser']
        shards = [partition(names, index, 2) for index in range(2)]
        self.assertFalse(set(shards[0]) & set(shards[1]))
        self.assertCountEqual(shards[0] + shards[1], names)
        self.assertEqual(partition(list(reversed(names)), 0, 2), shards[0])

    def test_bad_shard_and_duplicate_discovery_fail(self):
        for names, shard, count in [(['TestA'], 2, 2), (['TestA'], 0, 0),
                                     (['TestA', 'TestA'], 0, 2)]:
            with self.subTest(names=names, shard=shard, count=count), self.assertRaises(ValueError):
                partition(names, shard, count)


if __name__ == '__main__':
    unittest.main()
