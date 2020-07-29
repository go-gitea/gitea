import {renderMermaid} from './mermaid.js';

export default async function renderMarkdownContent() {
  await renderMermaid(document.querySelectorAll('.language-mermaid'));
}
