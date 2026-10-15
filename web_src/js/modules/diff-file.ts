import {reactive} from 'vue';
import type {Reactive} from 'vue';
import {toggleElem} from '../utils/dom.ts';
import {trString} from './i18n.ts';
import {basename, extname} from '../utils.ts';

const {pageData} = window.config;

// matches statusFromLetter in services/gitdiff/git_diff_tree.go
export type DiffStatus = '' | 'added' | 'modified' | 'deleted' | 'renamed' | 'copied' | 'typechanged' | 'unmerged' | 'unknown';

export type DiffTreeEntry = {
  FullName: string,
  OldFullName: string,
  DisplayName: string,
  NameHash: string,
  DiffStatus: DiffStatus,
  EntryMode: string,
  IsViewed: boolean,
  Children: DiffTreeEntry[] | null,
  FileIcon: string,
  ParentEntry?: DiffTreeEntry,
};

export type DiffFileTreeData = {
  TreeRoot: DiffTreeEntry,
};

// activeExtensions: 'all' = no filter (every extension passes); string[] = exact set of extensions allowed (empty = nothing passes).
type ExtensionFilter = 'all' | string[];

type DiffFileTree = {
  folderIcon: string;
  folderOpenIcon: string;
  diffFileTree: DiffFileTreeData;
  fullNameMap: Record<string, DiffTreeEntry>
  fileTreeIsVisible: boolean;
  selectedItem: string;
  filenameFilterQuery: string;
  activeExtensions: ExtensionFilter;
};

type DiffExtensionStats = {
  ext: string,
  count: number,
};

export type DiffExtensionFilterLocale = {
  filterByFileExtension: string,
  fileExtensions: string,
  noFileExtension: string,
  dotfileExtension: string,
  allFileExtensions: string,
};

export type DiffFileTreeLocale = DiffExtensionFilterLocale & {
  filterFiles: string,
  filterFilesClear: string,
};

let diffTreeStoreReactive: Reactive<DiffFileTree>;
export function diffTreeStore() {
  if (!diffTreeStoreReactive) {
    diffTreeStoreReactive = reactiveDiffTreeStore(pageData.DiffFileTree!, pageData.FolderIcon!, pageData.FolderOpenIcon!);
    const knownExtensions = getDiffTreeExtensionStats(diffTreeStoreReactive).map((stat) => stat.ext);
    diffTreeStoreReactive.activeExtensions = extensionFilterFromUrl(window.location.search, knownExtensions);
  }
  return diffTreeStoreReactive;
}

export function diffTreeStoreSetViewed(store: Reactive<DiffFileTree>, fullName: string, viewed: boolean) {
  const entry = store.fullNameMap[fullName];
  if (!entry) return;
  entry.IsViewed = viewed;
  for (let parent = entry.ParentEntry; parent; parent = parent.ParentEntry) {
    parent.IsViewed = isEntryViewed(parent);
  }
}

function fillFullNameMap(map: Record<string, DiffTreeEntry>, entry: DiffTreeEntry) {
  map[entry.FullName] = entry;
  if (!entry.Children) return;
  entry.IsViewed = isEntryViewed(entry);
  for (const child of entry.Children) {
    child.ParentEntry = entry;
    fillFullNameMap(map, child);
  }
}

export function reactiveDiffTreeStore(data: DiffFileTreeData, folderIcon: string, folderOpenIcon: string): Reactive<DiffFileTree> {
  const store = reactive<DiffFileTree>({
    diffFileTree: data,
    folderIcon,
    folderOpenIcon,
    fileTreeIsVisible: false,
    selectedItem: '',
    filenameFilterQuery: '',
    activeExtensions: 'all',
    fullNameMap: {},
  });
  fillFullNameMap(store.fullNameMap, data.TreeRoot);
  return store;
}

export const extDotfile = 'dotfile'; // bucket for ".gitignore" and friends, real extensions always start with a dot

const urlParamFileFilters = 'file-filters[]'; // same parameter GitHub uses, lists the selected extensions
const urlValueNoExtension = 'noextension';

export function extensionFilterFromUrl(search: string, knownExtensions: string[]): ExtensionFilter {
  const params = new URLSearchParams(search);
  if (!params.has(urlParamFileFilters)) return 'all';
  const extensions = params.getAll(urlParamFileFilters)
    .filter(Boolean)
    .map((ext) => ext === urlValueNoExtension ? '' : ext)
    .filter((ext) => knownExtensions.includes(ext));
  return extensions.length === knownExtensions.length ? 'all' : extensions;
}

export function extensionFilterToUrl(filter: ExtensionFilter, url: string): string {
  const parsed = new URL(url);
  parsed.searchParams.delete(urlParamFileFilters);
  if (filter !== 'all') {
    if (!filter.length) parsed.searchParams.append(urlParamFileFilters, '');
    for (const ext of filter) parsed.searchParams.append(urlParamFileFilters, ext || urlValueNoExtension);
  }
  return parsed.href;
}

function getFileExtension(filename: string): string {
  const ext = extname(filename).toLowerCase();
  if (ext) return ext;
  return basename(filename).startsWith('.') ? extDotfile : '';
}

function extensionRank(ext: string): number {
  if (!ext) return 2;
  return ext === extDotfile ? 1 : 0;
}

export function getDiffTreeExtensionStats(store: Reactive<DiffFileTree>): DiffExtensionStats[] {
  const extensionMap = new Map<string, number>();
  for (const entry of Object.values(store.fullNameMap)) {
    if (entry.EntryMode === 'tree' || !entry.FullName) continue;
    const ext = getFileExtension(entry.FullName);
    extensionMap.set(ext, (extensionMap.get(ext) ?? 0) + 1);
  }
  return Array.from(extensionMap, ([ext, count]) => ({ext, count}))
    .sort((a, b) => extensionRank(a.ext) - extensionRank(b.ext) || a.ext.localeCompare(b.ext));
}

function buildFilter(store: Reactive<DiffFileTree>) {
  const query = store.filenameFilterQuery.trim().toLowerCase();
  const exts = store.activeExtensions === 'all' ? null : new Set(store.activeExtensions);
  if (!query && !exts) return null;
  return (newName: string, oldName: string) => {
    if (query && !newName.toLowerCase().includes(query) && !oldName.toLowerCase().includes(query)) return false;
    return !exts || exts.has(getFileExtension(newName));
  };
}

// Children===null marks a file leaf; everything else (incl. the root, which has EntryMode="") is recursed into.
export function filterDiffTree(store: Reactive<DiffFileTree>): DiffTreeEntry | null {
  const matches = buildFilter(store);
  if (!matches) return store.diffFileTree.TreeRoot;
  const visit = (entry: DiffTreeEntry): DiffTreeEntry | null => {
    if (entry.Children === null) return matches(entry.FullName, entry.OldFullName) ? entry : null;
    const children = entry.Children.map(visit).filter((child): child is DiffTreeEntry => child !== null);
    if (!children.length) return null;
    return {...entry, Children: children};
  };
  return visit(store.diffFileTree.TreeRoot);
}

export function countMatchingFiles(store: Reactive<DiffFileTree>): number {
  const matches = buildFilter(store);
  let totalMatchingFilesCount = 0;
  for (const entry of Object.values(store.fullNameMap)) {
    if (entry.EntryMode === 'tree' || !entry.FullName) continue;
    if (!matches || matches(entry.FullName, entry.OldFullName)) totalMatchingFilesCount++;
  }
  return totalMatchingFilesCount;
}

function countEveryFileInDiff(store: Reactive<DiffFileTree>): number {
  let totalFilesCount = 0;
  for (const entry of Object.values(store.fullNameMap)) {
    if (entry.EntryMode !== 'tree' && entry.FullName) totalFilesCount++;
  }
  return totalFilesCount;
}

function updateLoadProgress(loadedFiles: number, totalFiles: number) {
  const el = document.querySelector('#diff-load-progress');
  if (!el) return;
  el.textContent = trString(el.getAttribute('data-text-too-many-files')!, loadedFiles, totalFiles);
}

function updateShowMoreButton(matchingBelow: number) {
  const btn = document.querySelector('#diff-show-more-files');
  if (!btn) return;
  if (matchingBelow > 0) {
    btn.textContent = trString(btn.getAttribute('data-text-matching')!, matchingBelow);
  } else {
    btn.textContent = btn.getAttribute('data-text-default')!;
  }
}

export function applyFiltersToFileBoxes(store: Reactive<DiffFileTree>) {
  const boxes = document.querySelectorAll<HTMLElement>('#diff-file-boxes .diff-file-box[data-new-filename]');
  const matches = buildFilter(store);
  if (!matches) {
    for (const box of boxes) toggleElem(box, true);
    toggleElem('#diff-no-matches', false);
    updateShowMoreButton(0);
    updateLoadProgress(boxes.length, countEveryFileInDiff(store));
    return;
  }
  let visibleCount = 0;
  for (const box of boxes) {
    const matched = matches(box.getAttribute('data-new-filename')!, box.getAttribute('data-old-filename')!);
    if (matched) visibleCount++;
    toggleElem(box, matched);
  }
  const matchingCount = countMatchingFiles(store);
  updateShowMoreButton(matchingCount - visibleCount);
  updateLoadProgress(boxes.length, countEveryFileInDiff(store));
  toggleElem('#diff-no-matches', matchingCount === 0);
}

function isEntryViewed(entry: DiffTreeEntry): boolean {
  if (entry.Children) {
    return entry.Children.every((child) => child.IsViewed);
  }
  return entry.IsViewed;
}
