import DiffFileTreeItem from './DiffFileTreeItem.vue';
import {createApp, h} from 'vue';
import type {DiffStatus, DiffTreeEntry} from '../modules/diff-file.ts';

function renderItem(diffStatus: string): string {
  const item: DiffTreeEntry = {Name: 'a.txt', DiffStatus: diffStatus as DiffStatus, Icon: '', IconClass: ''};
  const root = document.createElement('div');
  createApp({render: () => h(DiffFileTreeItem, {item, path: item.Name})}).mount(root);
  return root.innerHTML;
}

test('DiffFileTreeItem diff status icon', () => {
  document.body.innerHTML = `<script type="application/json" id="diff-file-tree-data">{"TreeRoot":{"Children":[]}}</script>`;
  expect(renderItem('typechanged')).toContain('octicon-diff-modified');
  // a status the frontend does not know must fall back instead of failing to render
  expect(renderItem('something-new')).toContain('octicon-blocked');
});
