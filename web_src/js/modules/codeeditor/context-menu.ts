import {closestDropdown, initDropdown} from '../dropdown.ts';
import {copyToClipboard} from '../clipboard.ts';
import {createElementFromAttrs} from '../../utils/dom.ts';
import {renderShortcut} from './command-palette.ts';
import {goToDefinitionAt} from './utils.ts';
import type {EditorView} from '@codemirror/view';
import type {CodemirrorModules} from './main.ts';

type MenuItem = {
  label: string;
  keys: string;
  disabled?: boolean;
  run: () => void | Promise<void>;
} | 'separator';

/** Get the word at cursor, or selected text. Checks adjacent positions when cursor is on a non-word char. */
export function getWordAtPosition(view: EditorView, from: number, to: number): string {
  if (from !== to) return view.state.doc.sliceString(from, to);
  for (const pos of [from, from - 1, from + 1]) {
    if (pos < 0 || pos > view.state.doc.length) continue;
    const range = view.state.wordAt(pos);
    if (range) return view.state.doc.sliceString(range.from, range.to);
  }
  return '';
}

/** Select all occurrences of the word at cursor for multi-cursor editing. */
export function selectAllOccurrences(cm: CodemirrorModules, view: EditorView) {
  const {from, to} = view.state.selection.main;
  const word = getWordAtPosition(view, from, to);
  if (!word) return;
  const ranges = [];
  let main = 0;
  const cursor = new cm.search.SearchCursor(view.state.doc, word);
  while (!cursor.done) {
    cursor.next();
    if (cursor.done) break;
    if (cursor.value.from <= from && cursor.value.to >= from) main = ranges.length;
    ranges.push(cm.state.EditorSelection.range(cursor.value.from, cursor.value.to));
  }
  if (ranges.length) {
    view.dispatch({selection: cm.state.EditorSelection.create(ranges, main)});
  }
}

/** Collect symbol definitions from the Lezer syntax tree. */
export function collectSymbols(cm: CodemirrorModules, view: EditorView): {label: string; kind: string; from: number}[] {
  const tree = cm.language.syntaxTree(view.state);
  const symbols: {label: string; kind: string; from: number}[] = [];
  const seen = new Set<number>(); // track by position to avoid O(n²) dedup
  const addSymbol = (label: string, kind: string, from: number) => {
    if (!seen.has(from)) {
      seen.add(from);
      symbols.push({label, kind, from});
    }
  };
  tree.iterate({
    enter(node): false | void {
      if (node.name === 'VariableDefinition' || node.name === 'DefName') {
        addSymbol(view.state.doc.sliceString(node.from, node.to), 'variable', node.from);
      } else if (node.name === 'FunctionDeclaration' || node.name === 'FunctionDecl' || node.name === 'ClassDeclaration') {
        const nameNode = node.node.getChild('VariableDefinition') || node.node.getChild('DefName');
        if (nameNode) {
          const kind = node.name === 'ClassDeclaration' ? 'class' : 'function';
          addSymbol(view.state.doc.sliceString(nameNode.from, nameNode.to), kind, nameNode.from);
        }
        return false;
      } else if (node.name === 'MethodDeclaration' || node.name === 'MethodDecl' || node.name === 'PropertyDefinition') {
        const nameNode = node.node.getChild('PropertyDefinition') || node.node.getChild('PropertyName') || node.node.getChild('DefName');
        if (nameNode) {
          addSymbol(view.state.doc.sliceString(nameNode.from, nameNode.to), node.name === 'PropertyDefinition' ? 'property' : 'method', nameNode.from);
        }
      } else if (node.name === 'TypeDecl' || node.name === 'TypeSpec') {
        const nameNode = node.node.getChild('DefName');
        if (nameNode) {
          addSymbol(view.state.doc.sliceString(nameNode.from, nameNode.to), 'type', nameNode.from);
        }
      }
    },
  });
  return symbols;
}

function createMenuPopup(cm: CodemirrorModules, view: EditorView, togglePalette: (view: EditorView) => boolean, goToSymbol: (view: EditorView) => void): HTMLElement {
  const {from, to} = view.state.selection.main;
  const hasSelection = from !== to;
  const hasDefinition = cm.language.syntaxTree(view.state).resolveInner(from, 1).name === 'VariableName';
  const hasWord = Boolean(getWordAtPosition(view, from, to));
  const items: MenuItem[] = [
    {label: 'Go to Definition', keys: 'F12', disabled: !hasDefinition, run: () => { goToDefinitionAt(cm, view, view.state.selection.main.from) }},
    {label: 'Go to Symbol…', keys: 'Mod+Shift+O', run: () => goToSymbol(view)},
    {label: 'Change All Occurrences', keys: 'Mod+F2', disabled: !hasWord, run: () => selectAllOccurrences(cm, view)},
    'separator',
    {label: 'Cut', keys: 'Mod+X', disabled: !hasSelection, run: async () => {
      const {from, to} = view.state.selection.main;
      if (await copyToClipboard(view.state.doc.sliceString(from, to))) {
        view.dispatch({changes: {from, to}});
      }
    }},
    {label: 'Copy', keys: 'Mod+C', disabled: !hasSelection, run: async () => {
      const {from, to} = view.state.selection.main;
      await copyToClipboard(view.state.doc.sliceString(from, to));
    }},
    {label: 'Paste', keys: 'Mod+V', run: async () => {
      try {
        const text = await navigator.clipboard.readText();
        view.dispatch(view.state.replaceSelection(text));
      } catch { /* clipboard permission denied */ }
    }},
    'separator',
    {label: 'Command Palette', keys: 'F1', run: () => { togglePalette(view) }},
  ];

  const popup = createElementFromAttrs('div', {class: 'dropdown-popup dropdown-popup-flex cm-context-menu'});
  for (const item of items) {
    if (item === 'separator') {
      popup.append(createElementFromAttrs('div', {class: 'dropdown-divider', role: 'separator'}));
      continue;
    }
    const row = createElementFromAttrs('button', {
      type: 'button', class: 'dropdown-item', 'aria-disabled': item.disabled ? 'true' : undefined,
    }, item.label, renderShortcut(item.keys, 'cm-context-menu-keys'));
    row.addEventListener('click', () => { item.run() });
    popup.append(row);
  }
  return popup;
}

export function contextMenu(cm: CodemirrorModules, togglePalette: (view: EditorView) => boolean, goToSymbol: (view: EditorView) => void) {
  return cm.view.EditorView.domEventHandlers({
    contextmenu(event: MouseEvent, view: EditorView) {
      event.preventDefault();
      const previousAnchor = view.dom.querySelector(':scope > .cm-context-menu-anchor');
      if (previousAnchor) closestDropdown(previousAnchor)!.hide();
      const pos = view.posAtCoords({x: event.clientX, y: event.clientY});
      const {from, to} = view.state.selection.main;
      if (pos !== null && (pos < from || pos > to)) view.dispatch({selection: {anchor: pos}});

      const trigger = createElementFromAttrs('button', {
        type: 'button', class: 'dropdown-trigger', tabindex: -1, 'aria-label': 'Editor context menu',
      });
      const container = createElementFromAttrs('div', {class: 'dropdown-container cm-context-menu-anchor'},
        trigger, createMenuPopup(cm, view, togglePalette, goToSymbol),
      );
      container.style.left = `${event.clientX}px`;
      container.style.top = `${event.clientY}px`;
      view.dom.append(container);

      const dropdown = initDropdown(container);
      const controller = new AbortController();
      container.addEventListener('ce-dropdown-hide', () => {
        controller.abort();
        if (document.activeElement === trigger) view.focus();
        container.remove();
      }, {once: true});
      const dismiss = () => dropdown.hide();
      const options = {signal: controller.signal};
      view.scrollDOM.addEventListener('scroll', dismiss, options);
      document.addEventListener('scroll', dismiss, options);
      window.addEventListener('blur', dismiss, options);
      window.addEventListener('resize', dismiss, options);
      dropdown.show();
    },
  });
}
