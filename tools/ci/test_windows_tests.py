import unittest

from windows_tests import partition


class ShardCoverageTests(unittest.TestCase):
    def test_every_root_test_appears_in_exactly_one_shard(self):
        names = ['Test' + str(index) for index in range(661)] + ['ExamplePage', 'FuzzParser']
        for count in (2, 4):
            with self.subTest(count=count):
                shards = [partition(names, index, count) for index in range(count)]
                for left in range(count):
                    for right in range(left + 1, count):
                        self.assertFalse(set(shards[left]) & set(shards[right]))
                self.assertCountEqual(sum(shards, []), names)
                self.assertEqual(partition(list(reversed(names)), 0, count), shards[0])

    def test_bad_shard_and_duplicate_discovery_fail(self):
        for names, shard, count in [(['TestA'], 2, 2), (['TestA'], 0, 0),
                                     (['TestA', 'TestA'], 0, 2)]:
            with self.subTest(names=names, shard=shard, count=count), self.assertRaises(ValueError):
                partition(names, shard, count)


if __name__ == '__main__':
    unittest.main()
