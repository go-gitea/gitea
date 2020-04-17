import {highlightBlock} from 'highlight.js';
import {createWindow} from 'domino';

self.onmessage = function ({data}) {
  const window = createWindow();
  self.document = window.document;

  const {index, html} = data;
  document.body.innerHTML = html;
  highlightBlock(document.body.firstChild);
  self.postMessage({index, html: document.body.innerHTML});
};
