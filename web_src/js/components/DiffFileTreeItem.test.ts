import DiffFileTreeItem from './DiffFileTreeItem.vue';
import {createApp, h} from 'vue';
import type {DiffStatus, DiffTreeEntry} from '../modules/diff-file.ts';

function renderItem(diffStatus: string): string {
  const item: DiffTreeEntry = {
    FullName: 'a.txt', OldFullName: '', DisplayName: 'a.txt', NameHash: 'hash',
    DiffStatus: diffStatus as DiffStatus, EntryMode: '', IsViewed: false, Children: null, FileIcon: '',
  };
  const root = document.createElement('div');
  createApp({render: () => h(DiffFileTreeItem, {item})}).mount(root);
  return root.innerHTML;
}

test('DiffFileTreeItem diff status icon', () => {
  window.config.pageData.DiffFileTree = {TreeRoot: {
    FullName: '', OldFullName: '', DisplayName: '', NameHash: 'root',
    DiffStatus: '', EntryMode: 'tree', IsViewed: false, Children: [], FileIcon: '',
  }};
  expect(renderItem('typechanged')).toContain('octicon-diff-modified');
  // a status the frontend does not know must fall back instead of failing to render
  expect(renderItem('something-new')).toContain('octicon-blocked');
});
