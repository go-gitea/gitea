import type {EditorView, ViewUpdate} from '@codemirror/view';
import type {CodemirrorModules} from './main.ts';

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
    if (match.index !== undefined && pos >= match.index && pos < match.index + url.length) {
      return url;
    }
  }
  return null;
}

// Lezer syntax tree node names for identifier usages and definitions across grammars
const usageNodes = new Set(['VariableName', 'Identifier', 'TypeIdentifier', 'TypeName', 'FieldIdentifier']);
const definitionNodes = new Set(['VariableDefinition', 'DefName', 'Definition', 'TypeDefinition', 'TypeDef']);

export function goToDefinitionAt(cm: CodemirrorModules, view: EditorView, pos: number): boolean {
  const tree = cm.language.syntaxTree(view.state);
  const node = tree.resolveInner(pos, 1);
  if (!node || !usageNodes.has(node.name)) return false;
  const name = view.state.doc.sliceString(node.from, node.to);
  let target: number | null = null;
  tree.iterate({
    enter(n): false | void {
      if (target !== null) return false;
      if (definitionNodes.has(n.name) && n.from !== node.from && view.state.doc.sliceString(n.from, n.to) === name) {
        target = n.from;
        return false;
      }
    },
  });
  if (target === null) return false;
  view.dispatch({selection: {anchor: target}, scrollIntoView: true});
  return true;
}

/** CodeMirror extension that makes URLs clickable via Ctrl/Cmd+click. */
export function clickableUrls(cm: CodemirrorModules) {
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
      if (!event.metaKey && !event.ctrlKey) return false;
      const pos = view.posAtCoords({x: event.clientX, y: event.clientY});
      if (pos === null) return false;
      const line = view.state.doc.lineAt(pos);
      const url = findUrlAtPosition(line.text, pos - line.from);
      if (url) {
        window.open(url, '_blank', 'noopener');
        event.preventDefault();
        return true;
      }
      // Fall back to go-to-definition: find the symbol at cursor and jump to its definition
      if (goToDefinitionAt(cm, view, pos)) {
        event.preventDefault();
        return true;
      }
      return false;
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
