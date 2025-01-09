import {renderMermaid} from './mermaid.ts';
import {renderMath} from './math.ts';
import {renderCodeCopy} from './codecopy.ts';
import {renderAsciicast} from './asciicast.ts';
import {initMarkupTasklist} from './tasklist.ts';

// code that runs for all markup content
export function initMarkupContent(): void {
  renderMermaid();
  renderMath();
  renderCodeCopy();
  renderAsciicast();
}

// code that only runs for comments
export function initCommentContent(): void {
  initMarkupTasklist();
}
