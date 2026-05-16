import {clippie} from 'clippie';
import {createTippy} from '../tippy.ts';
import {keySymbols} from '../../utils.ts';
import {goToDefinitionAt} from './utils.ts';
import type {Instance} from 'tippy.js';
import type {EditorView} from '@codemirror/view';
import type {CodemirrorModules} from './main.ts';

type MenuItem = {
  label: string;
  keys?: string;
  disabled?: boolean;
  run: (view: EditorView) => void | Promise<void>;
} | 'separator';

/** Get the word at cursor, or selected text. Checks adjacent positions when cursor is on a non-word char. */
export function getWordAtPosition(view: EditorView, from: number, to: number): string {
  if (from !== to) return view.state.doc.sliceString(from, to);
  for (const pos of [from, from - 1, from + 1]) {
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

function buildMenuItems(cm: CodemirrorModules, view: EditorView, togglePalette: (view: EditorView) => boolean, goToSymbol: (view: EditorView) => void): MenuItem[] {
  const {from, to} = view.state.selection.main;
  const hasSelection = from !== to;
  // Check if cursor is on a symbol that has a definition
  const tree = cm.language.syntaxTree(view.state);
  const nodeAtCursor = tree.resolveInner(from, 1);
  const hasDefinition = nodeAtCursor?.name === 'VariableName';
  const hasWord = Boolean(getWordAtPosition(view, from, to));
  return [
    {label: 'Go to Definition', keys: 'F12', disabled: !hasDefinition, run: (v) => { goToDefinitionAt(cm, v, v.state.selection.main.from) }},
    {label: 'Go to Symbol…', keys: 'Mod+Shift+O', run: goToSymbol},
    {label: 'Change All Occurrences', keys: 'Mod+F2', disabled: !hasWord, run: (v) => selectAllOccurrences(cm, v)},
    'separator',
    {label: 'Cut', keys: 'Mod+X', disabled: !hasSelection, run: async (v) => {
      const {from, to} = v.state.selection.main;
      if (await clippie(v.state.doc.sliceString(from, to))) {
        v.dispatch({changes: {from, to}});
      }
    }},
    {label: 'Copy', keys: 'Mod+C', disabled: !hasSelection, run: async (v) => {
      const {from, to} = v.state.selection.main;
      await clippie(v.state.doc.sliceString(from, to));
    }},
    {label: 'Paste', keys: 'Mod+V', run: async (view) => {
      try {
        const text = await navigator.clipboard.readText();
        view.dispatch(view.state.replaceSelection(text));
      } catch { /* clipboard permission denied */ }
    }},
    'separator',
    {label: 'Command Palette', keys: 'F1', run: (v) => { togglePalette(v) }},
  ];
}

type MenuResult = {el: HTMLElement; actions: ((() => void) | null)[]};

function createMenuElement(items: MenuItem[], view: EditorView, onAction: () => void): MenuResult {
  const menu = document.createElement('div');
  menu.className = 'cm-context-menu';
  const actions: ((() => void) | null)[] = [];
  for (const item of items) {
    if (item === 'separator') {
      const sep = document.createElement('div');
      sep.className = 'cm-context-menu-separator';
      menu.append(sep);
      continue;
    }
    const row = document.createElement('div');
    row.className = `item${item.disabled ? ' disabled' : ''}`;
    if (item.disabled) row.setAttribute('aria-disabled', 'true');
    const label = document.createElement('span');
    label.className = 'cm-context-menu-label';
    label.textContent = item.label;
    row.append(label);
    if (item.keys) {
      const keysEl = document.createElement('span');
      keysEl.className = 'cm-context-menu-keys';
      for (const key of item.keys.split('+')) {
        const kbd = document.createElement('kbd');
        kbd.textContent = keySymbols[key] || key;
        keysEl.append(kbd);
      }
      row.append(keysEl);
    }
    const execute = item.disabled ? null : () => { onAction(); item.run(view) };
    if (execute) {
      row.addEventListener('mousedown', (e) => { e.preventDefault(); e.stopPropagation(); execute() });
    }
    actions.push(execute);
    menu.append(row);
  }
  return {el: menu, actions};
}

export function contextMenu(cm: CodemirrorModules, togglePalette: (view: EditorView) => boolean, goToSymbol: (view: EditorView) => void) {
  let instance: Instance | null = null;

  function hideMenu() {
    if (instance) {
      instance.destroy();
      instance = null;
    }
  }

  return cm.view.EditorView.domEventHandlers({
    contextmenu(event: MouseEvent, view: EditorView) {
      event.preventDefault();
      hideMenu();

      // Place cursor at right-click position if not inside a selection
      const pos = view.posAtCoords({x: event.clientX, y: event.clientY});
      if (pos !== null) {
        const {from, to} = view.state.selection.main;
        if (pos < from || pos > to) {
          view.dispatch({selection: {anchor: pos}});
        }
      }

      const controller = new AbortController();
      const dismiss = () => {
        controller.abort();
        hideMenu();
      };

      const menuItems = buildMenuItems(cm, view, togglePalette, goToSymbol);
      const {el: menuEl, actions} = createMenuElement(menuItems, view, dismiss);

      // Create a virtual anchor at mouse position for tippy
      const anchor = document.createElement('div');
      anchor.style.position = 'fixed';
      anchor.style.left = `${event.clientX}px`;
      anchor.style.top = `${event.clientY}px`;
      document.body.append(anchor);

      instance = createTippy(anchor, {
        content: menuEl,
        theme: 'menu',
        trigger: 'manual',
        placement: 'bottom-start',
        interactive: true,
        arrow: false,
        offset: [0, 0],
        showOnCreate: true,
        onHidden: () => {
          anchor.remove();
          instance = null;
        },
      });
      const rows = menuEl.querySelectorAll<HTMLElement>('.item');
      let focusIndex = -1;
      const setFocus = (idx: number) => {
        focusIndex = idx;
        for (const [rowIdx, el] of rows.entries()) {
          el.classList.toggle('active', rowIdx === focusIndex);
        }
      };
      const nextEnabled = (from: number, dir: number) => {
        for (let step = 1; step <= actions.length; step++) {
          const idx = (from + dir * step + actions.length) % actions.length;
          if (actions[idx]) return idx;
        }
        return from;
      };

      document.addEventListener('mousedown', (e: MouseEvent) => {
        if (!menuEl.contains(e.target as Element)) dismiss();
      }, {signal: controller.signal});
      document.addEventListener('keydown', (e: KeyboardEvent) => {
        e.stopPropagation();
        e.preventDefault();
        if (e.key === 'Escape') {
          dismiss(); view.focus();
        } else if (e.key === 'ArrowDown') {
          setFocus(nextEnabled(focusIndex, 1));
        } else if (e.key === 'ArrowUp') {
          setFocus(nextEnabled(focusIndex, -1));
        } else if (e.key === 'Enter' && focusIndex >= 0 && actions[focusIndex]) {
          actions[focusIndex]!();
        }
      }, {signal: controller.signal, capture: true});
      view.scrollDOM.addEventListener('scroll', dismiss, {signal: controller.signal, once: true});
      document.addEventListener('scroll', dismiss, {signal: controller.signal, once: true});
      window.addEventListener('blur', dismiss, {signal: controller.signal});
      window.addEventListener('resize', dismiss, {signal: controller.signal});
    },
  });
}
