import {basename, extname, isObject} from '../utils.ts';
import {createElementFromHTML, onInputDebounce, toggleElem} from '../utils/dom.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {cleanUrl, findUrlAt, urlRawRegex} from '../utils/url.ts';
import {svg} from '../svg.ts';
import type {LanguageDescription} from '@codemirror/language';
import type {Compartment} from '@codemirror/state';
import type {EditorView, ViewUpdate} from '@codemirror/view';

type CodeEditorConfig = {
  indent_style?: 'tab' | 'space',
  indent_size?: number,
  tab_width?: string | number, // backend emits this as string
  trim_trailing_whitespace?: boolean,
};

type EditorOptions = {
  indentStyle: string;
  indentSize?: number;
  tabSize?: number;
  wordWrap: boolean;
  trimTrailingWhitespace: boolean;
};

export type CodemirrorEditor = {
  view: EditorView;
  trimTrailingWhitespace: boolean;
  languages: LanguageDescription[];
  compartments: {
    wordWrap: Compartment;
    language: Compartment;
    tabSize: Compartment;
    indentUnit: Compartment;
  };
};

function getCodeEditorConfig(input: HTMLInputElement): CodeEditorConfig | null {
  const json = input.getAttribute('data-code-editor-config');
  if (!json) return null;
  try {
    return JSON.parse(json);
  } catch {
    return null;
  }
}

// export editor for customization - https://github.com/go-gitea/gitea/issues/10409
function exportEditor(editor: EditorView): void {
  if (!window.codeEditors) window.codeEditors = [];
  if (!window.codeEditors.includes(editor)) window.codeEditors.push(editor);
}

async function importCodemirror() {
  const [view, state, search, language, commands, autocomplete, languageData, highlight, indentMarkers, vscodeKeymap] = await Promise.all([
    import(/* webpackChunkName: "codemirror" */ '@codemirror/view'),
    import(/* webpackChunkName: "codemirror" */ '@codemirror/state'),
    import(/* webpackChunkName: "codemirror" */ '@codemirror/search'),
    import(/* webpackChunkName: "codemirror" */ '@codemirror/language'),
    import(/* webpackChunkName: "codemirror" */ '@codemirror/commands'),
    import(/* webpackChunkName: "codemirror" */ '@codemirror/autocomplete'),
    import(/* webpackChunkName: "codemirror" */ '@codemirror/language-data'),
    import(/* webpackChunkName: "codemirror" */ '@lezer/highlight'),
    import(/* webpackChunkName: "codemirror" */ '@replit/codemirror-indentation-markers'),
    import(/* webpackChunkName: "codemirror" */ '@replit/codemirror-vscode-keymap'),
  ]);
  return {view, state, search, language, commands, autocomplete, languageData, highlight, indentMarkers, vscodeKeymap};
}

type PaletteCommand = {
  label: string;
  keys?: string;
  run: (view: EditorView) => void;
};

const isMac = /Mac/i.test(navigator.userAgent);
const keySymbols: Record<string, string> = isMac ?
  {Mod: '\u2318', Alt: '\u2325', Shift: '\u21E7', Up: '\u2191', Down: '\u2193', Enter: '\u23CE'} :
  {Mod: 'Ctrl', Up: '\u2191', Down: '\u2193', Enter: '\u23CE'};

function formatKeys(keys: string): string[] {
  return keys.split(/[+ ]/).map((k) => keySymbols[k] || k);
}

function commandPalette(cm: Awaited<ReturnType<typeof importCodemirror>>) {
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
    {label: 'Add Cursor Above', run: cm.commands.addCursorAbove},
    {label: 'Add Cursor Below', run: cm.commands.addCursorBelow},
    {label: 'Add Next Occurrence', keys: 'Mod+D', run: cm.search.selectNextOccurrence},
    {label: 'Go to Matching Bracket', run: cm.commands.cursorMatchingBracket},
    {label: 'Indent More', run: cm.commands.indentMore},
    {label: 'Indent Less', run: cm.commands.indentLess},
    {label: 'Fold Code', run: cm.language.foldCode},
    {label: 'Unfold Code', run: cm.language.unfoldCode},
    {label: 'Fold All', run: cm.language.foldAll},
    {label: 'Unfold All', run: cm.language.unfoldAll},
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
        const translated = cmd.label;
        const item = document.createElement('div');
        item.className = 'cm-command-palette-item';
        item.setAttribute('role', 'option');
        if (index === this.selectedIndex) {
          item.setAttribute('aria-selected', 'true');
        }

        const label = document.createElement('span');
        label.className = 'cm-command-palette-label';
        const matchIndex = query ? translated.toLowerCase().indexOf(query) : -1;
        if (matchIndex !== -1) {
          label.append(translated.slice(0, matchIndex));
          const mark = document.createElement('mark');
          mark.textContent = translated.slice(matchIndex, matchIndex + query.length);
          label.append(mark, translated.slice(matchIndex + query.length));
        } else {
          label.textContent = translated;
        }
        item.append(label);

        if (cmd.keys) {
          const keysEl = document.createElement('span');
          keysEl.className = 'cm-command-palette-keys';
          for (const k of formatKeys(cmd.keys)) {
            const kbd = document.createElement('kbd');
            kbd.textContent = k;
            keysEl.append(kbd);
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

function clickableLinks(cm: Awaited<ReturnType<typeof importCodemirror>>) {
  const urlMark = cm.view.Decoration.mark({class: 'cm-url'});
  const urlDecorator = new cm.view.MatchDecorator({
    regexp: urlRawRegex,
    decorate: (add, from, _to, match) => {
      const cleaned = cleanUrl(match[0]);
      add(from, from + cleaned.length, urlMark);
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
      const url = findUrlAt(view.state.doc.toString(), pos);
      if (!url) return false;
      window.open(url, '_blank', 'noopener,noreferrer');
      event.preventDefault();
      return true;
    },
  });

  const modClass = cm.view.ViewPlugin.fromClass(class {
    container: Element | null;
    handleKeyDown: (e: KeyboardEvent) => void;
    handleKeyUp: (e: KeyboardEvent) => void;
    constructor(view: EditorView) {
      this.container = view.dom.closest('.code-editor-container');
      this.handleKeyDown = (e) => { if (e.key === 'Meta' || e.key === 'Control') this.container?.classList.add('cm-mod-held'); };
      this.handleKeyUp = (e) => { if (e.key === 'Meta' || e.key === 'Control') this.container?.classList.remove('cm-mod-held'); };
      document.addEventListener('keydown', this.handleKeyDown);
      document.addEventListener('keyup', this.handleKeyUp);
    }
    destroy() {
      document.removeEventListener('keydown', this.handleKeyDown);
      document.removeEventListener('keyup', this.handleKeyUp);
      this.container?.classList.remove('cm-mod-held');
    }
  });

  return [plugin, handler, modClass];
}

async function createCodemirrorEditor(
  textarea: HTMLTextAreaElement,
  filename: string,
  editorOpts: EditorOptions,
): Promise<CodemirrorEditor> {
  const cm = await importCodemirror();
  const languageDescriptions = [
    ...cm.languageData.languages,
    cm.language.LanguageDescription.of({
      name: 'Elixir', extensions: ['ex', 'exs'],
      load: async () => (await import('codemirror-lang-elixir')).elixir(),
    }),
    cm.language.LanguageDescription.of({
      name: 'Nix', extensions: ['nix'],
      load: async () => (await import('@replit/codemirror-lang-nix')).nix(),
    }),
    cm.language.LanguageDescription.of({
      name: 'Svelte', extensions: ['svelte'],
      load: async () => (await import('@replit/codemirror-lang-svelte')).svelte(),
    }),
    cm.language.LanguageDescription.of({
      name: 'Makefile', filename: /^(GNUm|M|m)akefile$/,
      load: async () => new cm.language.LanguageSupport(cm.language.StreamLanguage.define((await import('@codemirror/legacy-modes/mode/cmake')).cmake)),
    }),
    cm.language.LanguageDescription.of({
      name: 'JSON5', extensions: ['json5', 'jsonc'],
      load: async () => (await import('@codemirror/lang-json')).json(),
    }),
  ];
  const matchedLang = cm.language.LanguageDescription.matchFilename(languageDescriptions, filename);

  const container = document.createElement('div');
  container.className = 'code-editor-container';
  if (!textarea.parentNode) throw new Error('Parent node absent');
  textarea.parentNode.append(container);

  const wordWrap = new cm.state.Compartment();
  const language = new cm.state.Compartment();
  const tabSize = new cm.state.Compartment();
  const indentUnitComp = new cm.state.Compartment();

  const view = new cm.view.EditorView({
    doc: textarea.value,
    parent: container,
    extensions: [
      cm.view.lineNumbers(),
      cm.language.codeFolding({
        placeholderDOM(_view: EditorView, onclick: (event: Event) => void) {
          const el = createElementFromHTML(html`<span class="cm-foldPlaceholder">${htmlRaw(svg('octicon-kebab-horizontal', 16))}</span>`);
          el.addEventListener('click', onclick);
          return el as unknown as HTMLElement;
        },
      }),
      cm.language.foldGutter({
        markerDOM(open: boolean) {
          return createElementFromHTML(svg(open ? 'octicon-chevron-down' : 'octicon-chevron-right', 16));
        },
      }),
      cm.view.highlightActiveLineGutter(),
      cm.view.highlightSpecialChars(),
      cm.view.highlightActiveLine(),
      cm.view.drawSelection(),
      cm.view.dropCursor(),
      cm.view.rectangularSelection(),
      cm.view.crosshairCursor(),
      textarea.getAttribute('data-placeholder') ? cm.view.placeholder(textarea.getAttribute('data-placeholder')!) : [],
      editorOpts.trimTrailingWhitespace ? cm.view.highlightTrailingWhitespace() : [],
      cm.search.search({top: true}),
      cm.search.highlightSelectionMatches(),
      cm.view.keymap.of([
        ...cm.vscodeKeymap.vscodeKeymap,
        ...cm.search.searchKeymap,
        cm.commands.indentWithTab,
        {key: 'Mod-k Mod-x', run: (view) => { trimTrailingWhitespaceFromView(view); return true }, preventDefault: true},
        {key: 'Mod-Enter', run: cm.commands.insertBlankLine, preventDefault: true},
        {key: 'Mod-k Mod-k', run: cm.commands.deleteToLineEnd, preventDefault: true},
        {key: 'Mod-k Mod-Backspace', run: cm.commands.deleteToLineStart, preventDefault: true},
      ]),
      cm.state.EditorState.allowMultipleSelections.of(true),
      cm.language.indentOnInput(),
      cm.language.syntaxHighlighting(cm.highlight.classHighlighter),
      cm.language.bracketMatching(),
      indentUnitComp.of(
        cm.language.indentUnit.of(
          editorOpts.indentStyle === 'tab' ? '\t' : ' '.repeat(editorOpts.indentSize || 4),
        ),
      ),
      cm.autocomplete.closeBrackets(),
      cm.autocomplete.autocompletion(),
      cm.state.EditorState.languageData.of(() => [{autocomplete: cm.autocomplete.completeAnyWord}]),
      cm.indentMarkers.indentationMarkers({
        colors: {
          light: 'var(--color-secondary)',
          dark: 'var(--color-secondary)',
          activeLight: 'var(--color-secondary-dark-2)',
          activeDark: 'var(--color-secondary-dark-2)',
        },
      }),
      cm.commands.history(),
      commandPalette(cm),
      clickableLinks(cm),
      tabSize.of(cm.state.EditorState.tabSize.of(editorOpts.tabSize || 4)),
      wordWrap.of(editorOpts.wordWrap ? cm.view.EditorView.lineWrapping : []),
      language.of(matchedLang ? await matchedLang.load() : []),
      cm.view.EditorView.updateListener.of((update: ViewUpdate) => {
        if (update.docChanged) {
          textarea.value = update.state.doc.toString();
          textarea.dispatchEvent(new Event('change')); // needed for jquery-are-you-sure
        }
      }),
    ],
  });

  exportEditor(view);

  const loading = document.querySelector('.editor-loading');
  if (loading) loading.remove();

  return {
    view,
    trimTrailingWhitespace: editorOpts.trimTrailingWhitespace,
    languages: languageDescriptions,
    compartments: {wordWrap, language, tabSize, indentUnit: indentUnitComp},
  };
}

function setupEditorOptionListeners(textarea: HTMLTextAreaElement, editor: CodemirrorEditor): void {
  const elEditorOptions = textarea.closest('form')?.querySelector('.code-editor-options');
  if (!elEditorOptions) return;

  const {compartments, view} = editor;
  const indentStyleSelect = elEditorOptions.querySelector<HTMLSelectElement>('.js-indent-style-select');
  const indentSizeSelect = elEditorOptions.querySelector<HTMLSelectElement>('.js-indent-size-select');

  const applyIndentSettings = async (style: string, size: number) => {
    const cm = await importCodemirror();
    view.dispatch({
      effects: [
        compartments.indentUnit.reconfigure(cm.language.indentUnit.of(style === 'tab' ? '\t' : ' '.repeat(size))),
        compartments.tabSize.reconfigure(cm.state.EditorState.tabSize.of(size)),
      ],
    });
  };

  indentStyleSelect?.addEventListener('change', () => {
    applyIndentSettings(indentStyleSelect.value, Number(indentSizeSelect?.value) || 4);
  });

  indentSizeSelect?.addEventListener('change', () => {
    applyIndentSettings(indentStyleSelect?.value || 'space', Number(indentSizeSelect.value) || 4);
  });

  elEditorOptions.querySelector('.js-code-find')?.addEventListener('click', async () => {
    const cm = await importCodemirror();
    if (cm.search.searchPanelOpen(view.state)) {
      cm.search.closeSearchPanel(view);
    } else {
      cm.search.openSearchPanel(view);
    }
  });

  elEditorOptions.querySelector<HTMLSelectElement>('.js-line-wrap-select')?.addEventListener('change', async (e) => {
    const target = e.target as HTMLSelectElement;
    const cm = await importCodemirror();
    view.dispatch({
      effects: compartments.wordWrap.reconfigure(target.value === 'on' ? cm.view.EditorView.lineWrapping : []),
    });
  });
}

function getFileBasedOptions(filename: string, lineWrapExts: string[]): Pick<EditorOptions, 'wordWrap'> {
  return {
    wordWrap: (lineWrapExts || []).includes(extname(filename)),
  };
}

function togglePreviewDisplay(previewable: boolean): void {
  // FIXME: here and below, the selector is too broad, it should only query in the editor related scope
  const previewTab = document.querySelector<HTMLElement>('a[data-tab="preview"]');
  // the "preview tab" exists for "file code editor", but doesn't exist for "git hook editor"
  if (!previewTab) return;

  toggleElem(previewTab, previewable);
  if (previewable) return;

  // If not previewable but the "preview" tab was active (user changes the filename to a non-previewable one),
  // then the "preview" tab becomes inactive (hidden), so the "write" tab should become active
  if (previewTab.classList.contains('active')) {
    const writeTab = document.querySelector<HTMLElement>('a[data-tab="write"]');
    writeTab?.click(); // TODO: it shouldn't need null-safe operator, writeTab must exist
  }
}

export async function createCodeEditor(textarea: HTMLTextAreaElement, filenameInput?: HTMLInputElement | string): Promise<CodemirrorEditor> {
  const filename = basename(typeof filenameInput === 'string' ? filenameInput : filenameInput?.value || '');
  const previewableExts = new Set((textarea.getAttribute('data-previewable-extensions') || '').split(','));
  const lineWrapExts = (textarea.getAttribute('data-line-wrap-extensions') || '').split(',');

  let editorOpts: EditorOptions;
  if (typeof filenameInput === 'object') {
    const editorConfig = getCodeEditorConfig(filenameInput);
    togglePreviewDisplay(previewableExts.has(extname(filename)));
    const configOpts = getEditorConfigOptions(editorConfig);
    editorOpts = {
      ...getFileBasedOptions(filenameInput.value, lineWrapExts),
      indentStyle: configOpts.indentStyle || 'space',
      trimTrailingWhitespace: false,
      ...configOpts,
    };
  } else {
    editorOpts = {indentStyle: 'tab', tabSize: 4, wordWrap: false, trimTrailingWhitespace: false};
  }

  const editor = await createCodemirrorEditor(textarea, filename, editorOpts);

  if (typeof filenameInput === 'object') {
    filenameInput.addEventListener('input', onInputDebounce(async () => {
      const newFilename = filenameInput.value;
      togglePreviewDisplay(previewableExts.has(extname(newFilename)));
      await updateEditorLanguage(editor, newFilename, lineWrapExts);
    }));
  }

  setupEditorOptionListeners(textarea, editor);

  return editor;
}

async function updateEditorLanguage(editor: CodemirrorEditor, filename: string, lineWrapExts: string[]): Promise<void> {
  const {view: cmView, language: cmLanguage} = await importCodemirror();
  const {compartments, view, languages: editorLanguages} = editor;

  const fileOption = getFileBasedOptions(filename, lineWrapExts);
  const newLanguage = cmLanguage.LanguageDescription.matchFilename(editorLanguages, filename);
  view.dispatch({
    effects: [
      compartments.wordWrap.reconfigure(
        fileOption.wordWrap ? cmView.EditorView.lineWrapping : [],
      ),
      compartments.language.reconfigure(newLanguage ? await newLanguage.load() : []),
    ],
  });
}

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

function getEditorConfigOptions(ec: CodeEditorConfig | null): Partial<EditorOptions> {
  if (!ec || !isObject(ec)) return {indentStyle: 'space'};

  const opts: Partial<EditorOptions> = {
    indentStyle: ec.indent_style || 'space',
    trimTrailingWhitespace: ec.trim_trailing_whitespace === true,
  };
  if (ec.indent_size) opts.indentSize = ec.indent_size;
  if (ec.tab_width) opts.tabSize = Number(ec.tab_width) || opts.indentSize;
  return opts;
}
