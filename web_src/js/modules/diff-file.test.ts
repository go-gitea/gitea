import {countMatchingFiles, diffTreeStoreSetViewed, extensionFilterFromUrl, extensionFilterToUrl, filterDiffTree, getDiffTreeExtensionStats, reactiveDiffTreeStore, type DiffTreeEntry} from './diff-file.ts';

function file(name: string, oldPath?: string): DiffTreeEntry {
  return {Name: name, OldPath: oldPath};
}

function dir(name: string, children: DiffTreeEntry[]): DiffTreeEntry {
  return {Name: name, Children: children};
}

function makeStore(children: DiffTreeEntry[]) {
  return reactiveDiffTreeStore({
    TreeRoot: {Name: '', Children: children},
    FolderIcon: '',
    FolderOpenIcon: '',
    FileIconClass: '',
  });
}

function visiblePaths(root: DiffTreeEntry | null): string[] {
  if (!root) return [];
  const out: string[] = [];
  const visit = (e: DiffTreeEntry, path: string) => {
    if (!e.Children) out.push(path);
    for (const c of e.Children ?? []) visit(c, path ? `${path}/${c.Name}` : c.Name);
  };
  visit(root, '');
  return out;
}

test('diff-tree', () => {
  const store = makeStore([
    dir('dir1', [dir('dir2', [{Name: 'seen.txt', IsViewed: true}])]),
    dir('dir3', [file('test.txt')]),
    file('other.txt'),
  ]);
  // a directory whose only child is a fully viewed directory is viewed too
  expect(store.TreeRoot.Children![0].IsViewed).toBe(true);
  diffTreeStoreSetViewed(store, 'dir3/test.txt', true);
  expect(store.pathMap.get('dir3/test.txt')!.IsViewed).toBe(true);
  expect(store.TreeRoot.Children![1].IsViewed).toBe(true);
});

test('filterDiffTree', () => {
  const store = makeStore([
    dir('dir1', [file('test.txt')]),
    file('other.ts'),
    file('other.TS'),
  ]);

  store.filenameFilterQuery = 'TesT';
  expect(visiblePaths(filterDiffTree(store))).toEqual(['dir1/test.txt']);

  store.filenameFilterQuery = '';
  store.activeExtensions = ['.ts'];
  expect(visiblePaths(filterDiffTree(store))).toEqual(['other.ts', 'other.TS']);

  store.activeExtensions = [];
  expect(visiblePaths(filterDiffTree(store))).toEqual([]);

  store.activeExtensions = 'all';
  expect(visiblePaths(filterDiffTree(store))).toEqual(['dir1/test.txt', 'other.ts', 'other.TS']);
});

test('getDiffTreeExtensionStats', () => {
  const store = makeStore([
    dir('dir1', [file('test.txt'), file('Makefile'), file('.gitignore')]),
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
    dir('dir1', [file('new-name.md', 'dir1/old-name.txt')]),
    file('other.ts'),
  ]);

  expect(countMatchingFiles(store)).toBe(2);

  // search query also matches the pre-rename path
  store.filenameFilterQuery = 'old-name';
  expect(visiblePaths(filterDiffTree(store))).toEqual(['dir1/new-name.md']);
  expect(countMatchingFiles(store)).toBe(1);

  // extension filter only applies to new name
  store.filenameFilterQuery = '';
  store.activeExtensions = ['.txt'];
  expect(visiblePaths(filterDiffTree(store))).toEqual([]);
  expect(countMatchingFiles(store)).toBe(0);
  store.activeExtensions = ['.md'];
  expect(visiblePaths(filterDiffTree(store))).toEqual(['dir1/new-name.md']);

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
