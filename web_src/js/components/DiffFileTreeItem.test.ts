import DiffFileTreeItem from './DiffFileTreeItem.vue';
import {createApp, h} from 'vue';
import type {DiffStatus, DiffTreeEntry} from '../modules/diff-file.ts';

function renderItem(diffStatus: string): string {
  const item: DiffTreeEntry = {Name: 'a.txt', NameHash: 'hash', DiffStatus: diffStatus as DiffStatus};
  const root = document.createElement('div');
  createApp({render: () => h(DiffFileTreeItem, {item})}).mount(root);
  return root.innerHTML;
}

test('DiffFileTreeItem diff status icons and links to unloaded files', () => {
  document.body.innerHTML = `<script type="application/json" id="diff-file-tree-data">{"TreeRoot":{"Children":[]}}</script>`;
  expect(renderItem('typechanged')).toContain('octicon-diff-modified');
  expect(renderItem('typechanged')).toContain('href="#diff-hash"');
  expect(renderItem('something-new')).toContain('octicon-blocked');
});
