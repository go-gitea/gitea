import type {EditorView, ViewUpdate} from '@codemirror/view';
import type {importCodemirror} from './main.ts';

/** Remove trailing whitespace from all lines in the editor. */
export function trimTrailingWhitespaceFromView(view: EditorView): void {
  const changes = [];
  const doc = view.state.doc;
  for (let i = 1; i <= doc.lines; i++) {
    const line = doc.line(i);
    const trimmed = line.text.replace(/\s+$/, '');
    if (trimmed.length < line.text.length) {
      changes.push({from: line.from + trimmed.length, to: line.to});
    }
  }
  if (changes.length) view.dispatch({changes});
}

/** Matches URLs, excluding characters that are never valid unencoded in URLs per RFC 3986. */
export const urlRawRegex = /\bhttps?:\/\/[^\s<>[\]]+/gi;

/** Strip trailing punctuation that is likely not part of the URL. */
export function trimUrlPunctuation(url: string): string {
  url = url.replace(/[.,;:'"]+$/, '');
  // Strip trailing closing parens only if unbalanced (not part of the URL like Wikipedia links)
  while (url.endsWith(')') && (url.match(/\(/g) || []).length < (url.match(/\)/g) || []).length) {
    url = url.slice(0, -1);
  }
  return url;
}

/** Find the URL at the given character position in a document string, or null if none. */
export function findUrlAtPosition(doc: string, pos: number): string | null {
  for (const match of doc.matchAll(urlRawRegex)) {
    const url = trimUrlPunctuation(match[0]);
    if (match.index !== undefined && pos >= match.index && pos <= match.index + url.length) {
      return url;
    }
  }
  return null;
}

/** CodeMirror extension that makes URLs clickable via Ctrl/Cmd+click. */
export function clickableUrls(cm: Awaited<ReturnType<typeof importCodemirror>>) {
  const urlMark = cm.view.Decoration.mark({class: 'cm-url'});
  const urlDecorator = new cm.view.MatchDecorator({
    regexp: urlRawRegex,
    decorate: (add, from, _to, match) => {
      const trimmed = trimUrlPunctuation(match[0]);
      add(from, from + trimmed.length, urlMark);
    },
  });

  const plugin = cm.view.ViewPlugin.fromClass(class {
    decorations: ReturnType<typeof urlDecorator.createDeco>;
    constructor(view: EditorView) {
      this.decorations = urlDecorator.createDeco(view);
    }
    update(update: ViewUpdate) {
      this.decorations = urlDecorator.updateDeco(update, this.decorations);
    }
  }, {decorations: (v) => v.decorations});

  const handler = cm.view.EditorView.domEventHandlers({
    mousedown(event: MouseEvent, view: EditorView) {
      if (!(event.metaKey || event.ctrlKey)) return false;
      const pos = view.posAtCoords({x: event.clientX, y: event.clientY});
      if (pos === null) return false;
      const url = findUrlAtPosition(view.state.doc.toString(), pos);
      if (!url) return false;
      window.open(url);
      event.preventDefault();
      return true;
    },
  });

  const modClass = cm.view.ViewPlugin.fromClass(class {
    container: Element;
    handleKeyDown: (e: KeyboardEvent) => void;
    handleKeyUp: (e: KeyboardEvent) => void;
    handleBlur: () => void;
    constructor(view: EditorView) {
      this.container = view.dom.closest('.code-editor-container')!;
      this.handleKeyDown = (e) => { if (e.key === 'Meta' || e.key === 'Control') this.container.classList.add('cm-mod-held'); };
      this.handleKeyUp = (e) => { if (e.key === 'Meta' || e.key === 'Control') this.container.classList.remove('cm-mod-held'); };
      this.handleBlur = () => this.container.classList.remove('cm-mod-held');
      document.addEventListener('keydown', this.handleKeyDown);
      document.addEventListener('keyup', this.handleKeyUp);
      window.addEventListener('blur', this.handleBlur);
    }
    destroy() {
      document.removeEventListener('keydown', this.handleKeyDown);
      document.removeEventListener('keyup', this.handleKeyUp);
      window.removeEventListener('blur', this.handleBlur);
      this.container.classList.remove('cm-mod-held');
    }
  });

  return [plugin, handler, modClass];
}
