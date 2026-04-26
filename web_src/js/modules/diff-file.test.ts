import {diffTreeStoreSetViewed, getDiffTreeExtensionStats, isDiffTreeEntryVisible, reactiveDiffTreeStore} from './diff-file.ts';

test('diff-tree', () => {
  const store = reactiveDiffTreeStore({
    'TreeRoot': {
      'FullName': '',
      'DisplayName': '',
      'EntryMode': '',
      'IsViewed': false,
      'NameHash': '....',
      'DiffStatus': '',
      'FileIcon': '',
      'Children': [
        {
          'FullName': 'dir1',
          'DisplayName': 'dir1',
          'EntryMode': 'tree',
          'IsViewed': false,
          'NameHash': '....',
          'DiffStatus': '',
          'FileIcon': '',
          'Children': [
            {
              'FullName': 'dir1/test.txt',
              'DisplayName': 'test.txt',
              'DiffStatus': 'added',
              'NameHash': '....',
              'EntryMode': '',
              'IsViewed': false,
              'FileIcon': '',
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
          'FileIcon': '',
          'Children': null,
        },
      ],
    },
  }, '', '');
  diffTreeStoreSetViewed(store, 'dir1/test.txt', true);
  expect(store.fullNameMap['dir1/test.txt'].IsViewed).toBe(true);
  expect(store.fullNameMap['dir1'].IsViewed).toBe(true);
});

test('diff-tree visibility keeps directories for matching files', () => {
  const store = reactiveDiffTreeStore({
    TreeRoot: {
      FullName: '',
      DisplayName: '',
      EntryMode: '',
      IsViewed: false,
      NameHash: 'root',
      DiffStatus: '',
      FileIcon: '',
      Children: [
        {
          FullName: 'dir1',
          DisplayName: 'dir1',
          EntryMode: 'tree',
          IsViewed: false,
          NameHash: 'dir1',
          DiffStatus: '',
          FileIcon: '',
          Children: [
            {
              FullName: 'dir1/test.txt',
              DisplayName: 'test.txt',
              DiffStatus: 'added',
              NameHash: 'file1',
              EntryMode: '',
              IsViewed: false,
              FileIcon: '',
              Children: null,
            },
          ],
        },
        {
          FullName: 'other.ts',
          DisplayName: 'other.ts',
          NameHash: 'file2',
          DiffStatus: 'added',
          EntryMode: '',
          IsViewed: false,
          FileIcon: '',
          Children: null,
        },
      ],
    },
  }, '', '');

  store.filenameFilterQuery = 'test';
  expect(isDiffTreeEntryVisible(store, store.fullNameMap.dir1)).toBe(true);
  expect(isDiffTreeEntryVisible(store, store.fullNameMap['dir1/test.txt'])).toBe(true);
  expect(isDiffTreeEntryVisible(store, store.fullNameMap['other.ts'])).toBe(false);

  store.filenameFilterQuery = '';
  store.activeExtensions = ['.ts'];
  store.noFileExtensionLabel = '(no extension)';
  expect(isDiffTreeEntryVisible(store, store.fullNameMap.dir1)).toBe(false);
  expect(isDiffTreeEntryVisible(store, store.fullNameMap['other.ts'])).toBe(true);

  store.activeExtensions = [];
  expect(isDiffTreeEntryVisible(store, store.fullNameMap.dir1)).toBe(false);
  expect(isDiffTreeEntryVisible(store, store.fullNameMap['dir1/test.txt'])).toBe(false);
  expect(isDiffTreeEntryVisible(store, store.fullNameMap['other.ts'])).toBe(false);
});

test('diff-tree extension stats include files not yet loaded in diff boxes', () => {
  const store = reactiveDiffTreeStore({
    TreeRoot: {
      FullName: '',
      DisplayName: '',
      EntryMode: '',
      IsViewed: false,
      NameHash: 'root',
      DiffStatus: '',
      FileIcon: '',
      Children: [
        {
          FullName: 'dir1',
          DisplayName: 'dir1',
          EntryMode: 'tree',
          IsViewed: false,
          NameHash: 'dir1',
          DiffStatus: '',
          FileIcon: '',
          Children: [
            {
              FullName: 'dir1/test.txt',
              DisplayName: 'test.txt',
              DiffStatus: 'added',
              NameHash: 'file1',
              EntryMode: '',
              IsViewed: false,
              FileIcon: '',
              Children: null,
            },
            {
              FullName: 'dir1/Makefile',
              DisplayName: 'Makefile',
              DiffStatus: 'added',
              NameHash: 'file2',
              EntryMode: '',
              IsViewed: false,
              FileIcon: '',
              Children: null,
            },
          ],
        },
        {
          FullName: 'other.ts',
          DisplayName: 'other.ts',
          NameHash: 'file3',
          DiffStatus: 'added',
          EntryMode: '',
          IsViewed: false,
          FileIcon: '',
          Children: null,
        },
      ],
    },
  }, '', '');

  store.noFileExtensionLabel = '(no extension)';

  expect(getDiffTreeExtensionStats(store)).toEqual([
    {ext: '.txt', count: 1},
    {ext: '(no extension)', count: 1},
    {ext: '.ts', count: 1},
  ]);
});
