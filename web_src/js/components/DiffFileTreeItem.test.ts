import DiffFileTreeItem from './DiffFileTreeItem.vue';
import {createApp, h} from 'vue';
import type {DiffStatus, DiffTreeEntry} from '../modules/diff-file.ts';

function renderItem(diffStatus: string): string {
  const item: DiffTreeEntry = {FullName: 'a.txt', DisplayName: 'a.txt', NameHash: 'hash', DiffStatus: diffStatus as DiffStatus, Icon: 0};
  const root = document.createElement('div');
  createApp({render: () => h(DiffFileTreeItem, {item})}).mount(root);
  return root.innerHTML;
}

test('DiffFileTreeItem diff status icon', () => {
  const data = {TreeRoot: {FullName: '', DisplayName: '', EntryMode: 'tree', Icon: 0, Children: []}, Icons: [''], FolderIcon: '', FolderOpenIcon: ''};
  document.body.innerHTML = `<script type="application/json" id="diff-file-tree-data">${JSON.stringify(data)}</script>`;
  expect(renderItem('typechanged')).toContain('octicon-diff-modified');
  // a status the frontend does not know must fall back instead of failing to render
  expect(renderItem('something-new')).toContain('octicon-blocked');
});
