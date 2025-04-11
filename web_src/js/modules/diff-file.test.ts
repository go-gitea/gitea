import {diffTreeStoreSetViewed, reactiveDiffTreeStore} from './diff-file.ts';

test('diff-tree', () => {
  const store = reactiveDiffTreeStore({
    'TreeRoot': {
      'FullName': '',
      'DisplayName': '',
      'EntryMode': '',
      'IsViewed': false,
      'NameHash': '....',
      'DiffStatus': '',
      'Children': [
        {
          'FullName': 'dir1',
          'DisplayName': 'dir1',
          'EntryMode': 'tree',
          'IsViewed': false,
          'NameHash': '....',
          'DiffStatus': '',
          'Children': [
            {
              'FullName': 'dir1/test.txt',
              'DisplayName': 'test.txt',
              'DiffStatus': 'added',
              'NameHash': '....',
              'EntryMode': '',
              'IsViewed': false,
              'Children': null,
            },
          ],
        },
        {
          'FullName': 'other.txt',
          'DisplayName': 'other.txt',
          'NameHash': '........',
          'DiffStatus': 'added',
          'EntryMode': '',
          'IsViewed': false,
          'Children': null,
        },
      ],
    },
  });
  diffTreeStoreSetViewed(store, 'dir1/test.txt', true);
  expect(store.fullNameMap['dir1/test.txt'].IsViewed).toBe(true);
  expect(store.fullNameMap['dir1'].IsViewed).toBe(true);
});
