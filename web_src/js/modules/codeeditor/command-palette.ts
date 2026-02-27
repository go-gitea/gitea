import type {EditorView, ViewUpdate} from '@codemirror/view';
import type {importCodemirror} from './main.ts';
import {trimTrailingWhitespaceFromView} from './utils.ts';

type PaletteCommand = {
  label: string;
  keys?: string;
  run: (view: EditorView) => void;
};

const isMac = /Mac/i.test(navigator.userAgent);
const keySymbols: Record<string, string> = isMac ?
  {Mod: '\u2318', Alt: '\u2325', Shift: '\u21E7', Ctrl: '\u2303', Up: '\u2191', Down: '\u2193', Enter: '\u23CE'} :
  {Mod: 'Ctrl', Shift: 'Shift', Alt: 'Alt', Up: '\u2191', Down: '\u2193', Enter: '\u23CE'};

function formatKeys(keys: string): string[][] {
  return keys.split(' ').map((chord) => chord.split('+').map((k) => keySymbols[k] || k));
}

export function commandPalette(cm: Awaited<ReturnType<typeof importCodemirror>>) {
  const openEffect = cm.state.StateEffect.define();

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

  const plugin = cm.view.ViewPlugin.fromClass(class {
    overlay: HTMLElement | null = null;
    input: HTMLInputElement | null = null;
    list: HTMLElement | null = null;
    filtered: PaletteCommand[] = commands;
    selectedIndex = 0;
    handleClickOutside: (e: MouseEvent) => void;
    view: EditorView;

    constructor(view: EditorView) {
      this.view = view;
      this.handleClickOutside = (e: MouseEvent) => {
        if (this.overlay && !this.overlay.contains(e.target as Node)) {
          this.removeOverlay();
        }
      };
    }

    update(upd: ViewUpdate) {
      for (const tr of upd.transactions) {
        for (const e of tr.effects) {
          if (e.is(openEffect) && !this.overlay) {
            this.show();
          }
        }
      }
    }

    show() {
      const container = this.view.dom.closest('.code-editor-container');
      if (!container) return;

      this.overlay = document.createElement('div');
      this.overlay.className = 'cm-command-palette';

      this.input = document.createElement('input');
      this.input.className = 'cm-command-palette-input';
      this.input.placeholder = 'Type a command...';
      this.input.addEventListener('input', () => this.filter());
      this.input.addEventListener('keydown', (e) => this.handleKey(e));

      this.list = document.createElement('div');
      this.list.className = 'cm-command-palette-list';
      this.list.setAttribute('role', 'listbox');

      this.overlay.append(this.input, this.list);
      container.append(this.overlay);

      this.filtered = commands;
      this.selectedIndex = 0;
      this.renderList();
      this.input.focus();

      requestAnimationFrame(() => {
        document.addEventListener('mousedown', this.handleClickOutside);
      });
    }

    filter() {
      const q = this.input!.value.toLowerCase();
      this.filtered = q ?
        commands.filter((cmd) => cmd.label.toLowerCase().includes(q)) :
        commands;
      this.selectedIndex = 0;
      this.renderList();
    }

    renderList() {
      if (!this.list) return;
      const query = this.input?.value.toLowerCase() || '';
      this.list.textContent = '';

      for (const [index, cmd] of this.filtered.entries()) {
        const item = document.createElement('div');
        item.className = 'cm-command-palette-item';
        item.setAttribute('role', 'option');
        if (index === this.selectedIndex) {
          item.setAttribute('aria-selected', 'true');
        }

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

        if (cmd.keys) {
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
        }

        item.addEventListener('mousedown', (e) => {
          e.preventDefault();
          this.execute(cmd);
        });
        item.addEventListener('pointerenter', () => {
          this.selectedIndex = index;
          this.updateSelected();
        });
        this.list.append(item);
      }
    }

    updateSelected() {
      if (!this.list) return;
      this.list.querySelector('[aria-selected]')?.removeAttribute('aria-selected');
      const el = this.list.children[this.selectedIndex];
      if (el) {
        el.setAttribute('aria-selected', 'true');
        el.scrollIntoView({block: 'nearest'});
      }
    }

    handleKey(e: KeyboardEvent) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        this.selectedIndex = Math.min(this.selectedIndex + 1, this.filtered.length - 1);
        this.updateSelected();
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        this.selectedIndex = Math.max(this.selectedIndex - 1, 0);
        this.updateSelected();
      } else if (e.key === 'Enter') {
        e.preventDefault();
        if (this.filtered[this.selectedIndex]) {
          this.execute(this.filtered[this.selectedIndex]);
        }
      } else if (e.key === 'Escape') {
        e.preventDefault();
        this.removeOverlay();
      }
    }

    execute(cmd: PaletteCommand) {
      this.removeOverlay();
      this.view.focus();
      cmd.run(this.view);
    }

    removeOverlay() {
      document.removeEventListener('mousedown', this.handleClickOutside);
      this.overlay?.remove();
      this.overlay = null;
      this.input = null;
      this.list = null;
    }

    destroy() {
      this.removeOverlay();
    }
  });

  function openPalette(view: EditorView) {
    view.dispatch({effects: openEffect.of(null)});
    return true;
  }

  return [plugin, cm.view.keymap.of([
    {key: 'Mod-Shift-p', run: openPalette, preventDefault: true},
    {key: 'F1', run: openPalette, preventDefault: true},
  ])];
}
