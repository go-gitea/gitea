import {createApp} from 'vue';
import TipTapRichTextEditor from '../components/TipTapRichTextEditor.vue';

export function initTipTapRichTextEditor() {
  const el = document.querySelector('#rich-text-editor');
  if (!el) return;
  const content = el.innerHTML;
  const view = createApp(TipTapRichTextEditor, {content});
  view.mount(el);
}
