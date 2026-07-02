import {countMatchingFiles, diffTreeStoreSetViewed, filterDiffTree, getDiffTreeExtensionStats, reactiveDiffTreeStore, type DiffTreeEntry} from './diff-file.ts';

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
      FullName: '', DisplayName: '', EntryMode: 'tree', IsViewed: false,
      NameHash: 'root', DiffStatus: '', FileIcon: '', Children: children,
    },
  }, '', '');
}

function visibleNames(root: DiffTreeEntry | null): string[] {
  if (!root) return [];
  const out: string[] = [];
  const visit = (e: DiffTreeEntry) => {
    if (e.EntryMode !== 'tree') out.push(e.FullName);
    for (const c of e.Children ?? []) visit(c);
  };
  visit(root);
  return out;
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

test('filterDiffTree keeps directories that contain matching files', () => {
  const store = makeStore([
    dir('dir1', [file('dir1/test.txt')]),
    file('other.ts'),
  ]);

  store.filenameFilterQuery = 'test';
  expect(visibleNames(filterDiffTree(store))).toEqual(['dir1/test.txt']);

  store.filenameFilterQuery = '';
  store.activeExtensions = ['.ts'];
  expect(visibleNames(filterDiffTree(store))).toEqual(['other.ts']);

  store.activeExtensions = [];
  expect(visibleNames(filterDiffTree(store))).toEqual([]);

  store.activeExtensions = 'all';
  expect(visibleNames(filterDiffTree(store))).toEqual(['dir1/test.txt', 'other.ts']);
});

test('getDiffTreeExtensionStats counts every file in the diff tree', () => {
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

test('countMatchingFiles counts matching file leaves across the whole PR', () => {
  const store = makeStore([
    dir('dir1', [file('dir1/a.ts'), file('dir1/b.css')]),
    file('c.ts'),
  ]);

  expect(countMatchingFiles(store)).toBe(3);

  store.activeExtensions = ['.ts'];
  expect(countMatchingFiles(store)).toBe(2);

  store.activeExtensions = 'all';
  store.filenameFilterQuery = 'b.';
  expect(countMatchingFiles(store)).toBe(1);
});

test('extension filtering is case-insensitive', () => {
  const store = makeStore([
    file('a.ts'),
    file('b.TS'),
    file('c.Ts'),
  ]);
  expect(getDiffTreeExtensionStats(store)).toEqual([
    {ext: '.ts', count: 3},
  ]);

  store.activeExtensions = ['.ts'];
  expect(visibleNames(filterDiffTree(store))).toEqual(['a.ts', 'b.TS', 'c.Ts']);
});
