import type {EditorView} from '@codemirror/view';
import type {importCodemirror} from './main.ts';
import {isMac, keySymbols} from '../../utils.ts';
import {trimTrailingWhitespaceFromView} from './utils.ts';

type PaletteCommand = {
  label: string;
  keys: string;
  run: (view: EditorView) => void;
};

function formatKeys(keys: string): string[][] {
  return keys.split(' ').map((chord) => chord.split('+').map((k) => keySymbols[k] || k));
}

export function commandPalette(cm: Awaited<ReturnType<typeof importCodemirror>>) {
  const commands: PaletteCommand[] = [
    {label: 'Undo', keys: 'Mod+Z', run: cm.commands.undo},
    {label: 'Redo', keys: 'Mod+Shift+Z', run: cm.commands.redo},
    {label: 'Find', keys: 'Mod+F', run: cm.search.openSearchPanel},
    {label: 'Go to line', keys: 'Mod+Alt+G', run: cm.search.gotoLine},
    {label: 'Select All', keys: 'Mod+A', run: cm.commands.selectAll},
    {label: 'Delete Line', keys: 'Mod+Shift+K', run: cm.commands.deleteLine},
    {label: 'Move Line Up', keys: 'Alt+Up', run: cm.commands.moveLineUp},
    {label: 'Move Line Down', keys: 'Alt+Down', run: cm.commands.moveLineDown},
    {label: 'Copy Line Up', keys: 'Shift+Alt+Up', run: cm.commands.copyLineUp},
    {label: 'Copy Line Down', keys: 'Shift+Alt+Down', run: cm.commands.copyLineDown},
    {label: 'Toggle Comment', keys: 'Mod+/', run: cm.commands.toggleComment},
    {label: 'Insert Blank Line', keys: 'Mod+Enter', run: cm.commands.insertBlankLine},
    {label: 'Add Cursor Above', keys: isMac ? 'Mod+Alt+Up' : 'Ctrl+Alt+Up', run: cm.commands.addCursorAbove},
    {label: 'Add Cursor Below', keys: isMac ? 'Mod+Alt+Down' : 'Ctrl+Alt+Down', run: cm.commands.addCursorBelow},
    {label: 'Add Next Occurrence', keys: 'Mod+D', run: cm.search.selectNextOccurrence},
    {label: 'Go to Matching Bracket', keys: 'Mod+Shift+\\', run: cm.commands.cursorMatchingBracket},
    {label: 'Indent More', keys: 'Mod+]', run: cm.commands.indentMore},
    {label: 'Indent Less', keys: 'Mod+[', run: cm.commands.indentLess},
    {label: 'Fold Code', keys: isMac ? 'Mod+Alt+[' : 'Ctrl+Shift+[', run: cm.language.foldCode},
    {label: 'Unfold Code', keys: isMac ? 'Mod+Alt+]' : 'Ctrl+Shift+]', run: cm.language.unfoldCode},
    {label: 'Fold All', keys: 'Ctrl+Alt+[', run: cm.language.foldAll},
    {label: 'Unfold All', keys: 'Ctrl+Alt+]', run: cm.language.unfoldAll},
    {label: 'Trigger Autocomplete', keys: 'Ctrl+Space', run: cm.autocomplete.startCompletion},
    {label: 'Trim Trailing Whitespace', keys: 'Mod+K Mod+X', run: trimTrailingWhitespaceFromView},
  ];

  let overlay: HTMLElement | null = null;
  let filtered: PaletteCommand[] = [];
  let selectedIndex = 0;
  let cleanupClickOutside: (() => void) | null = null;

  function hide(view: EditorView) {
    if (!overlay) return;
    cleanupClickOutside?.();
    cleanupClickOutside = null;
    overlay.remove();
    overlay = null;
    view.focus();
  }

  function renderList(list: HTMLElement, query: string) {
    list.textContent = '';
    if (!filtered.length) {
      const empty = document.createElement('div');
      empty.className = 'cm-command-palette-empty';
      empty.textContent = 'No matches';
      list.append(empty);
      return;
    }
    for (const [index, cmd] of filtered.entries()) {
      const item = document.createElement('div');
      item.className = 'cm-command-palette-item';
      item.setAttribute('role', 'option');
      item.setAttribute('data-index', String(index));
      if (index === selectedIndex) item.setAttribute('aria-selected', 'true');

      const label = document.createElement('span');
      label.className = 'cm-command-palette-label';
      const matchIndex = query ? cmd.label.toLowerCase().indexOf(query) : -1;
      if (matchIndex !== -1) {
        label.append(cmd.label.slice(0, matchIndex));
        const mark = document.createElement('mark');
        mark.textContent = cmd.label.slice(matchIndex, matchIndex + query.length);
        label.append(mark, cmd.label.slice(matchIndex + query.length));
      } else {
        label.textContent = cmd.label;
      }
      item.append(label);

      const keysEl = document.createElement('span');
      keysEl.className = 'cm-command-palette-keys';
      for (const [chordIndex, chord] of formatKeys(cmd.keys).entries()) {
        if (chordIndex > 0) keysEl.append('\u2192');
        for (const k of chord) {
          const kbd = document.createElement('kbd');
          kbd.textContent = k;
          keysEl.append(kbd);
        }
      }
      item.append(keysEl);
      list.append(item);
    }
  }

  function show(view: EditorView) {
    const container = view.dom.closest('.code-editor-container')!;
    overlay = document.createElement('div');
    overlay.className = 'cm-command-palette';

    const input = document.createElement('input');
    input.className = 'cm-command-palette-input';
    input.placeholder = 'Type a command...';

    const list = document.createElement('div');
    list.className = 'cm-command-palette-list';
    list.setAttribute('role', 'listbox');

    filtered = commands;
    selectedIndex = 0;

    const updateSelected = () => {
      list.querySelector('[aria-selected]')?.removeAttribute('aria-selected');
      const el = list.children[selectedIndex];
      if (el) {
        el.setAttribute('aria-selected', 'true');
        el.scrollIntoView({block: 'nearest'});
      }
    };

    const execute = (cmd: PaletteCommand) => {
      hide(view);
      cmd.run(view);
    };

    list.addEventListener('pointerover', (e) => {
      const item = (e.target as Element).closest<HTMLElement>('.cm-command-palette-item');
      if (!item) return;
      selectedIndex = Number(item.getAttribute('data-index'));
      updateSelected();
    });

    list.addEventListener('mousedown', (e) => {
      const item = (e.target as Element).closest<HTMLElement>('.cm-command-palette-item');
      if (!item) return;
      e.preventDefault();
      const cmd = filtered[Number(item.getAttribute('data-index'))];
      if (cmd) execute(cmd);
    });

    input.addEventListener('input', () => {
      const q = input.value.toLowerCase();
      filtered = q ? commands.filter((cmd) => cmd.label.toLowerCase().includes(q)) : commands;
      selectedIndex = 0;
      renderList(list, q);
    });

    input.addEventListener('keydown', (e) => {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        selectedIndex = Math.min(selectedIndex + 1, filtered.length - 1);
        updateSelected();
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        selectedIndex = Math.max(selectedIndex - 1, 0);
        updateSelected();
      } else if (e.key === 'Enter') {
        e.preventDefault();
        if (filtered[selectedIndex]) execute(filtered[selectedIndex]);
      } else if (e.key === 'Escape') {
        e.preventDefault();
        hide(view);
      }
    });

    overlay.append(input, list);
    container.append(overlay);
    renderList(list, '');
    input.focus();

    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as Element;
      if (overlay && !overlay.contains(target) && !target.closest('.js-code-command-palette')) {
        hide(view);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    cleanupClickOutside = () => document.removeEventListener('mousedown', handleClickOutside);
  }

  function togglePalette(view: EditorView) {
    if (overlay) {
      hide(view);
    } else {
      show(view);
    }
    return true;
  }

  return {
    extensions: cm.view.keymap.of([
      {key: 'Mod-Shift-p', run: togglePalette, preventDefault: true},
      {key: 'F1', run: togglePalette, preventDefault: true},
    ]),
    togglePalette,
  };
}
