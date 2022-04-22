import {renderMermaid} from './mermaid.js';
import {renderCodeCopy} from './codecopy.js';
import {initMarkupTasklist} from './tasklist.js';

// code that runs for all markup content
export function initMarkupContent() {
  renderMermaid();
  renderCodeCopy();
}

// code that only runs for comments
export function initCommentContent() {
  initMarkupTasklist();
}
