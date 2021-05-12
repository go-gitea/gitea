import {renderMermaid} from './mermaid.js';

export async function renderMarkupContent() {
  await renderMermaid(document.querySelectorAll('code.language-mermaid'));
}
