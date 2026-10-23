import {countMatchingFiles, diffTreeStoreSetViewed, extensionFilterFromUrl, extensionFilterToUrl, filterDiffTree, getDiffTreeExtensionStats, reactiveDiffTreeStore, type DiffTreeEntry} from './diff-file.ts';

function file(name: string, oldName: string = ''): DiffTreeEntry {
  return {
    FullName: name,
    OldFullName: oldName,
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
    OldFullName: '',
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
      FullName: '', OldFullName: '', DisplayName: '', EntryMode: 'tree', IsViewed: false,
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

test('filterDiffTree', () => {
  const store = makeStore([
    dir('dir1', [file('dir1/test.txt')]),
    file('other.ts'),
    file('other.TS'),
  ]);

  store.filenameFilterQuery = 'TesT';
  expect(visibleNames(filterDiffTree(store))).toEqual(['dir1/test.txt']);

  store.filenameFilterQuery = '';
  store.activeExtensions = ['.ts'];
  expect(visibleNames(filterDiffTree(store))).toEqual(['other.ts', 'other.TS']);

  store.activeExtensions = [];
  expect(visibleNames(filterDiffTree(store))).toEqual([]);

  store.activeExtensions = 'all';
  expect(visibleNames(filterDiffTree(store))).toEqual(['dir1/test.txt', 'other.ts', 'other.TS']);
});

test('getDiffTreeExtensionStats', () => {
  const store = makeStore([
    dir('dir1', [file('dir1/test.txt'), file('dir1/Makefile'), file('dir1/.gitignore')]),
    file('.eslintrc.json'), // a dotfile with an extension keeps that extension
    file('other.ts'),
    file('other.TXT'), // case-insensitive
  ]);
  expect(getDiffTreeExtensionStats(store)).toEqual([
    {ext: '.json', count: 1},
    {ext: '.ts', count: 1},
    {ext: '.txt', count: 2},
    {ext: 'dotfile', count: 1},
    {ext: '', count: 1},
  ]);
});

test('countMatchingFiles', () => {
  const store = makeStore([
    dir('dir1', [file('dir1/new-name.md', 'dir1/old-name.txt')]),
    file('other.ts'),
  ]);

  expect(countMatchingFiles(store)).toBe(2);

  // search query also matches the pre-rename path
  store.filenameFilterQuery = 'old-name';
  expect(visibleNames(filterDiffTree(store))).toEqual(['dir1/new-name.md']);
  expect(countMatchingFiles(store)).toBe(1);

  // extension filter only applies to new name
  store.filenameFilterQuery = '';
  store.activeExtensions = ['.txt'];
  expect(visibleNames(filterDiffTree(store))).toEqual([]);
  expect(countMatchingFiles(store)).toBe(0);
  store.activeExtensions = ['.md'];
  expect(visibleNames(filterDiffTree(store))).toEqual(['dir1/new-name.md']);

  store.activeExtensions = ['.md', '.ts'];
  expect(countMatchingFiles(store)).toBe(2);
});

test('extensionFilter url round-trip', () => {
  const known = ['.go', '.ts', 'dotfile', ''];
  const url = 'http://localhost/owner/repo/pulls/1/files?style=split';
  const roundTrip = (filter: Parameters<typeof extensionFilterToUrl>[0]) =>
    extensionFilterFromUrl(new URL(extensionFilterToUrl(filter, url)).search, known);

  expect(extensionFilterToUrl('all', url)).toEqual(url);
  expect(roundTrip('all')).toEqual('all');
  expect(roundTrip(['.go', ''])).toEqual(['.go', '']);
  expect(roundTrip([])).toEqual([]);

  // other query parameters survive, "no extension" gets a stable token
  expect(extensionFilterToUrl(['.go', ''], url)).toEqual(`${url}&file-filters%5B%5D=.go&file-filters%5B%5D=noextension`);

  // unknown extensions are dropped, selecting every known one is the same as no filter
  expect(extensionFilterFromUrl('?file-filters[]=.go&file-filters[]=.nope', known)).toEqual(['.go']);
  expect(extensionFilterFromUrl('?file-filters[]=.go&file-filters[]=.ts&file-filters[]=dotfile&file-filters[]=noextension', known)).toEqual('all');
});
