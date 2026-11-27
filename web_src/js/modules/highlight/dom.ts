import {registerGlobalInitFunc} from '../observer.ts';
import {queryElems} from '../../utils/dom.ts';
import {highlightLines, loadHighlighter, type Segment} from './highlight.ts';

type Token = [length: number, classes: string];

// the tokens of the lines, their line breaks included, lengths are in UTF-16 units like the DOM's text
function lineTokens(lines: string[], segments: Segment[][]): Token[] {
  const tokens: Token[] = [];
  for (const [idx, line] of lines.entries()) {
    let length = 0;
    for (const [text, classes] of segments[idx]) {
      tokens.push([text.length, classes]);
      length += text.length;
    }
    tokens.push([line.length - length + 1, '']); // the line break, and a carriage return the highlighter has dropped
  }
  return tokens;
}

// Highlights server-rendered code in place. The elements are the consecutive parts of one document, like the lines of a file.
// The tokens are wrapped around the existing text, so markup inside of it, like unicode escapes and search matches, is kept.
export async function highlightCodeElements(els: HTMLElement[], language: string): Promise<void> {
  const highlighter = await loadHighlighter(language);
  if (!highlighter) return;
  const textNodes: Text[] = [];
  for (const el of els) {
    const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
    while (walker.nextNode()) textNodes.push(walker.currentNode as Text);
  }
  const lines = textNodes.map((node) => node.data).join('').split('\n');
  const segments = highlightLines(highlighter, lines);
  if (segments.length !== lines.length) return;
  const tokens = lineTokens(lines, segments);
  let tokenIdx = 0;
  let tokenLeft = tokens[0][0];
  for (const node of textNodes) {
    const text = node.data;
    const parts: Array<string | HTMLElement> = [];
    for (let pos = 0; pos < text.length;) {
      while (!tokenLeft) tokenLeft = tokens[++tokenIdx][0];
      const length = Math.min(tokenLeft, text.length - pos);
      const classes = tokens[tokenIdx][1];
      const part = text.slice(pos, pos + length);
      if (classes) {
        const span = document.createElement('span');
        span.className = classes;
        span.textContent = part;
        parts.push(span);
      } else {
        parts.push(part);
      }
      pos += length;
      tokenLeft -= length;
    }
    if (parts.some((part) => typeof part !== 'string')) node.replaceWith(...parts);
  }
}

// code rendered by templates, with the lines of a file as ".code-inner" elements
function highlightCodeLines(el: HTMLElement): void {
  highlightCodeElements(Array.from(el.querySelectorAll<HTMLElement>('.code-inner')), el.getAttribute('data-highlight-language')!);
}

export function initCodeHighlight(): void {
  registerGlobalInitFunc('initCodeHighlight', highlightCodeLines);
}

// the code blocks and code previews of rendered markup, mermaid and math blocks are replaced by their own renderers
export function initMarkupCodeHighlight(elMarkup: HTMLElement): void {
  queryElems<HTMLElement>(elMarkup, '.code-preview-container table[data-highlight-language]', highlightCodeLines); // the sanitizer drops "data-global-init"
  queryElems<HTMLElement>(elMarkup, 'pre.code-block > code', (el) => {
    const language = el.getAttribute('data-code-language') ?? /(?:^|\s)language-(\S+)/.exec(el.className)?.[1];
    if (!language || language === 'mermaid' || language === 'math') return;
    el.classList.add('code-highlight');
    highlightCodeElements([el], language);
  });
}
