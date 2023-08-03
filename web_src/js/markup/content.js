import {renderMermaid} from './mermaid.js';
import {renderMath} from './math.js';
import {renderCodeCopy} from './codecopy.js';
import {renderAsciicast} from './asciicast.js';
import {initMarkupTasklist} from './tasklist.js';

// code that runs for all markup content
export function initMarkupContent() {
  renderMermaid();
  renderMath();
  renderCodeCopy();
  renderAsciicast();
}

// code that only runs for comments
export function initCommentContent() {
  initMarkupTasklist();
}
