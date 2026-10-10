import {reactive} from 'vue';
import type {Reactive} from 'vue';
import {toggleElem} from '../utils/dom.ts';
import {trString} from './i18n.ts';
import {basename, extname, joinPath} from '../utils.ts';

// matches statusFromLetter in services/gitdiff/git_diff_tree.go
export type DiffStatus = '' | 'added' | 'modified' | 'deleted' | 'renamed' | 'copied' | 'typechanged' | 'unmerged' | 'unknown';

export type DiffTreeEntry = {
  OldPath?: string,
  Name: string,
  DiffStatus?: DiffStatus,
  IsViewed?: boolean,
  Children?: DiffTreeEntry[],
  IconID?: string,
  IconClass?: string, // only when it differs from the tree's FileIconClass
  ParentEntry?: DiffTreeEntry,
};

type DiffFileTreeData = {
  TreeRoot: DiffTreeEntry,
  FolderIcon: string,
  FolderOpenIcon: string,
  FileIconClass: string,
};

// activeExtensions: 'all' = no filter (every extension passes); string[] = exact set of extensions allowed (empty = nothing passes).
type ExtensionFilter = 'all' | string[];

type DiffFileTree = DiffFileTreeData & {
  pathMap: Map<string, DiffTreeEntry>;
  boxIdMap: Map<string, string>; // file path to the DOM id of its diff box, only for boxes already rendered
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
    const data = JSON.parse(document.querySelector('#diff-file-tree-data')!.textContent) as DiffFileTreeData;
    diffTreeStoreReactive = reactiveDiffTreeStore(data);
    const knownExtensions = getDiffTreeExtensionStats(diffTreeStoreReactive).map((stat) => stat.ext);
    diffTreeStoreReactive.activeExtensions = extensionFilterFromUrl(window.location.search, knownExtensions);
  }
  return diffTreeStoreReactive;
}

export function diffTreeStoreSetViewed(store: Reactive<DiffFileTree>, path: string, viewed: boolean) {
  const entry = store.pathMap.get(path);
  if (!entry) return;
  entry.IsViewed = viewed;
  for (let parent = entry.ParentEntry; parent; parent = parent.ParentEntry) {
    parent.IsViewed = isEntryViewed(parent);
  }
}

const queryFileBoxes = () => document.querySelectorAll<HTMLElement>('#diff-file-boxes .diff-file-box[data-new-filename]');

// the diff boxes are rendered lazily, so the tree can only link to the ones already in the DOM
function refreshBoxIdMap(store: Reactive<DiffFileTree>, boxes: NodeListOf<HTMLElement>) {
  store.boxIdMap = new Map(Array.from(boxes, (box) => [box.getAttribute('data-new-filename')!, box.id]));
}

function fillPathMap(map: Map<string, DiffTreeEntry>, entry: DiffTreeEntry, path: string) {
  if (!entry.Children) {
    map.set(path, entry);
    return;
  }
  for (const child of entry.Children) {
    child.ParentEntry = entry;
    fillPathMap(map, child, joinPath(path, child.Name));
  }
  entry.IsViewed = isEntryViewed(entry); // after the children, so nested directories roll up
}

export function reactiveDiffTreeStore(data: DiffFileTreeData): Reactive<DiffFileTree> {
  const store = reactive<DiffFileTree>({
    ...data,
    fileTreeIsVisible: false,
    selectedItem: '',
    filenameFilterQuery: '',
    activeExtensions: 'all',
    pathMap: new Map(),
    boxIdMap: new Map(),
  });
  fillPathMap(store.pathMap, store.TreeRoot, '');
  refreshBoxIdMap(store, queryFileBoxes());
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
  for (const path of store.pathMap.keys()) {
    const ext = getFileExtension(path);
    extensionMap.set(ext, (extensionMap.get(ext) ?? 0) + 1);
  }
  return Array.from(extensionMap, ([ext, count]) => ({ext, count}))
    .sort((a, b) => extensionRank(a.ext) - extensionRank(b.ext) || a.ext.localeCompare(b.ext));
}

function buildFilter(store: Reactive<DiffFileTree>) {
  const query = store.filenameFilterQuery.trim().toLowerCase();
  const exts = store.activeExtensions === 'all' ? null : new Set(store.activeExtensions);
  if (!query && !exts) return null;
  return (newName: string, oldName?: string) => {
    if (query && !newName.toLowerCase().includes(query) && !oldName?.toLowerCase().includes(query)) return false;
    return !exts || exts.has(getFileExtension(newName));
  };
}

export function filterDiffTree(store: Reactive<DiffFileTree>): DiffTreeEntry | null {
  const matches = buildFilter(store);
  if (!matches) return store.TreeRoot;
  const visit = (entry: DiffTreeEntry, path: string): DiffTreeEntry | null => {
    if (!entry.Children) return matches(path, entry.OldPath) ? entry : null;
    const children = entry.Children.map((child) => visit(child, joinPath(path, child.Name))).filter((child): child is DiffTreeEntry => child !== null);
    if (!children.length) return null;
    return {...entry, Children: children};
  };
  return visit(store.TreeRoot, '');
}

export function countMatchingFiles(store: Reactive<DiffFileTree>): number {
  const matches = buildFilter(store);
  if (!matches) return store.pathMap.size;
  let totalMatchingFilesCount = 0;
  for (const [path, entry] of store.pathMap) {
    if (matches(path, entry.OldPath)) totalMatchingFilesCount++;
  }
  return totalMatchingFilesCount;
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
  const boxes = queryFileBoxes();
  refreshBoxIdMap(store, boxes); // boxes may have been added by "show more files"
  const matches = buildFilter(store);
  if (!matches) {
    for (const box of boxes) toggleElem(box, true);
    toggleElem('#diff-no-matches', false);
    updateShowMoreButton(0);
    updateLoadProgress(boxes.length, store.pathMap.size);
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
  updateLoadProgress(boxes.length, store.pathMap.size);
  toggleElem('#diff-no-matches', matchingCount === 0);
}

function isEntryViewed(entry: DiffTreeEntry): boolean {
  return entry.Children!.every((child) => child.IsViewed);
}
