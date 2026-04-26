import {reactive} from 'vue';
import type {Reactive} from 'vue';

const {pageData} = window.config;

export type DiffStatus = '' | 'added' | 'modified' | 'deleted' | 'renamed' | 'copied' | 'typechange';

export type DiffTreeEntry = {
  FullName: string,
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

type DiffFileTree = {
  folderIcon: string;
  folderOpenIcon: string;
  diffFileTree: DiffFileTreeData;
  fullNameMap: Record<string, DiffTreeEntry>
  fileTreeIsVisible: boolean;
  selectedItem: string;
  filenameFilterQuery: string;
  activeExtensions: string[] | null;
  noFileExtensionLabel: string;
};

export type DiffExtensionStats = {
  ext: string,
  count: number,
};

let diffTreeStoreReactive: Reactive<DiffFileTree>;
export function diffTreeStore() {
  if (!diffTreeStoreReactive) {
    diffTreeStoreReactive = reactiveDiffTreeStore(pageData.DiffFileTree!, pageData.FolderIcon!, pageData.FolderOpenIcon!);
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
  const store = reactive({
    diffFileTree: data,
    folderIcon,
    folderOpenIcon,
    fileTreeIsVisible: false,
    selectedItem: '',
    filenameFilterQuery: '',
    activeExtensions: null,
    noFileExtensionLabel: '',
    fullNameMap: {},
  });
  fillFullNameMap(store.fullNameMap, data.TreeRoot);
  return store;
}

export function getDiffFileExtension(filename: string, noFileExtensionLabel: string): string {
  const basename = filename.substring(filename.lastIndexOf('/') + 1);
  const lastDot = basename.lastIndexOf('.');
  if (lastDot === -1) {
    return noFileExtensionLabel;
  }
  return basename.substring(lastDot);
}

export function getDiffTreeExtensionStats(store: Reactive<DiffFileTree>): DiffExtensionStats[] {
  const extensionMap = new Map<string, number>();

  for (const entry of Object.values(store.fullNameMap)) {
    if (!entry.FullName || entry.EntryMode === 'tree') continue;

    const ext = getDiffFileExtension(entry.FullName, store.noFileExtensionLabel);
    extensionMap.set(ext, (extensionMap.get(ext) ?? 0) + 1);
  }

  return Array.from(extensionMap.entries(), ([ext, count]) => ({ext, count}))
    .sort((a, b) => b.count - a.count);
}

export function hasActiveDiffTreeFilter(store: Reactive<DiffFileTree>): boolean {
  return Boolean(store.filenameFilterQuery.trim()) || store.activeExtensions !== null;
}

export function isDiffTreeEntryVisible(store: Reactive<DiffFileTree>, entry: DiffTreeEntry): boolean {
  if (entry.EntryMode === 'tree') {
    return Boolean(entry.Children?.some((child) => isDiffTreeEntryVisible(store, child)));
  }

  const query = store.filenameFilterQuery.trim().toLowerCase();
  if (query && !entry.FullName.toLowerCase().includes(query)) {
    return false;
  }

  if (store.activeExtensions === null) {
    return true;
  }

  return store.activeExtensions.includes(getDiffFileExtension(entry.FullName, store.noFileExtensionLabel));
}

export function applyFiltersToFileBoxes(store: Reactive<DiffFileTree>) {
  const fileBoxes = document.querySelectorAll<HTMLElement>('#diff-file-boxes .diff-file-box[data-new-filename]');
  const query = store.filenameFilterQuery.trim().toLowerCase();

  for (const box of fileBoxes) {
    const filename = box.getAttribute('data-new-filename') || '';

    const matchesQuery = !query || filename.toLowerCase().includes(query);
    const ext = getDiffFileExtension(filename, store.noFileExtensionLabel);
    const matchesExtension = store.activeExtensions === null || store.activeExtensions.includes(ext);

    box.classList.toggle('tw-hidden', !matchesQuery || !matchesExtension);
  }
}

function isEntryViewed(entry: DiffTreeEntry): boolean {
  if (entry.Children) {
    return entry.Children.every((child) => child.IsViewed);
  }
  return entry.IsViewed;
}
