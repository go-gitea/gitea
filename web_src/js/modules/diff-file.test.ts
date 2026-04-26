import {buildFilterPredicate, diffTreeStoreSetViewed, getDiffTreeExtensionStats, isDiffTreeEntryVisible, reactiveDiffTreeStore, type DiffTreeEntry} from './diff-file.ts';

function file(name: string): DiffTreeEntry {
  return {
    FullName: name,
    DisplayName: name.split('/').pop()!,
    DiffStatus: 'added',
    NameHash: name,
    EntryMode: '',
    IsViewed: false,
    FileIcon: '',
    Children: null,
  };
}

function dir(name: string, children: DiffTreeEntry[]): DiffTreeEntry {
  return {
    FullName: name,
    DisplayName: name.split('/').pop()!,
    EntryMode: 'tree',
    IsViewed: false,
    NameHash: name,
    DiffStatus: '',
    FileIcon: '',
    Children: children,
  };
}

function makeStore(children: DiffTreeEntry[]) {
  return reactiveDiffTreeStore({
    TreeRoot: {
      FullName: '', DisplayName: '', EntryMode: '', IsViewed: false,
      NameHash: 'root', DiffStatus: '', FileIcon: '', Children: children,
    },
  }, '', '');
}

test('diff-tree', () => {
  const store = makeStore([
    dir('dir1', [file('dir1/test.txt')]),
    file('other.txt'),
  ]);
  diffTreeStoreSetViewed(store, 'dir1/test.txt', true);
  expect(store.fullNameMap['dir1/test.txt'].IsViewed).toBe(true);
  expect(store.fullNameMap['dir1'].IsViewed).toBe(true);
});

test('diff-tree visibility keeps directories for matching files', () => {
  const store = makeStore([
    dir('dir1', [file('dir1/test.txt')]),
    file('other.ts'),
  ]);
  const isVisible = (entry: DiffTreeEntry) => isDiffTreeEntryVisible(entry, buildFilterPredicate(store));

  store.filenameFilterQuery = 'test';
  expect(isVisible(store.fullNameMap.dir1)).toBe(true);
  expect(isVisible(store.fullNameMap['dir1/test.txt'])).toBe(true);
  expect(isVisible(store.fullNameMap['other.ts'])).toBe(false);

  store.filenameFilterQuery = '';
  store.activeExtensions = ['.ts'];
  expect(isVisible(store.fullNameMap.dir1)).toBe(false);
  expect(isVisible(store.fullNameMap['other.ts'])).toBe(true);

  store.activeExtensions = [];
  expect(isVisible(store.fullNameMap.dir1)).toBe(false);
  expect(isVisible(store.fullNameMap['dir1/test.txt'])).toBe(false);
  expect(isVisible(store.fullNameMap['other.ts'])).toBe(false);
});

test('diff-tree extension stats include files not yet loaded in diff boxes', () => {
  const store = makeStore([
    dir('dir1', [file('dir1/test.txt'), file('dir1/Makefile')]),
    file('other.ts'),
  ]);
  expect(getDiffTreeExtensionStats(store)).toEqual([
    {ext: '.txt', count: 1},
    {ext: '', count: 1},
    {ext: '.ts', count: 1},
  ]);
});
