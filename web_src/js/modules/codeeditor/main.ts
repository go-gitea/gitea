import {extname} from '../../utils.ts';
import {createElementFromHTML, onInputDebounce, toggleElem} from '../../utils/dom.ts';
import {html, htmlRaw} from '../../utils/html.ts';
import {svg} from '../../svg.ts';
import {commandPalette} from './command-palette.ts';
import {clickableUrls, trimTrailingWhitespaceFromView} from './utils.ts';
import type {LanguageDescription} from '@codemirror/language';
import type {Compartment} from '@codemirror/state';
import type {EditorView, ViewUpdate} from '@codemirror/view';

// CodeEditorConfig is also used by backend, defined in "editor_util.go"
type CodeEditorConfig = {
  is_new_file?: boolean; // whether the editor is creating a new file, only set on the edit page
  previewable_extensions?: string[]; // file extensions that support preview rendering
  indent_style: string; // "space" or "tab", from .editorconfig
  indent_size?: number; // number of spaces per indent level, from .editorconfig
  tab_width?: number; // display width of a tab character, from .editorconfig
  line_wrap_extensions?: string[]; // file extensions that enable line wrapping by default
  line_wrap: boolean; // whether line wrapping is enabled for the current file
  trim_trailing_whitespace: boolean; // whether to trim trailing whitespace on save, from .editorconfig
};

export type CodemirrorEditor = {
  view: EditorView;
  trimTrailingWhitespace: boolean;
  togglePalette: (view: EditorView) => boolean;
  languages: LanguageDescription[];
  compartments: {
    wordWrap: Compartment;
    language: Compartment;
    tabSize: Compartment;
    indentUnit: Compartment;
  };
};

export async function importCodemirror() {
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

async function createCodemirrorEditor(
  textarea: HTMLTextAreaElement,
  filename: string,
  config: CodeEditorConfig,
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
      load: async () => new cm.language.LanguageSupport(cm.language.StreamLanguage.define((await import('@codemirror/legacy-modes/mode/shell')).shell)),
    }),
    cm.language.LanguageDescription.of({
      name: 'JSON5', extensions: ['json5', 'jsonc'],
      load: async () => (await import('@codemirror/lang-json')).json(),
    }),
  ];
  const matchedLang = cm.language.LanguageDescription.matchFilename(languageDescriptions, filename);

  const container = document.createElement('div');
  container.className = 'code-editor-container';
  textarea.parentNode!.append(container);

  const wordWrap = new cm.state.Compartment();
  const language = new cm.state.Compartment();
  const tabSize = new cm.state.Compartment();
  const indentUnitComp = new cm.state.Compartment();
  const palette = commandPalette(cm);

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
      config.trim_trailing_whitespace ? cm.view.highlightTrailingWhitespace() : [],
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
          config.indent_style === 'tab' ? '\t' : ' '.repeat(config.indent_size || 4),
        ),
      ),
      cm.autocomplete.closeBrackets(),
      cm.autocomplete.autocompletion(),
      cm.state.EditorState.languageData.of(() => [{autocomplete: cm.autocomplete.completeAnyWord}]),
      cm.indentMarkers.indentationMarkers({
        colors: {
          light: 'transparent',
          dark: 'transparent',
          activeLight: 'var(--color-secondary-dark-3)',
          activeDark: 'var(--color-secondary-dark-3)',
        },
      }),
      cm.commands.history(),
      palette.extensions,
      clickableUrls(cm),
      tabSize.of(cm.state.EditorState.tabSize.of(config.tab_width || 4)),
      wordWrap.of(config.line_wrap ? cm.view.EditorView.lineWrapping : []),
      language.of(matchedLang ? await matchedLang.load() : []),
      cm.view.EditorView.updateListener.of((update: ViewUpdate) => {
        if (update.docChanged) {
          textarea.value = update.state.doc.toString();
          textarea.dispatchEvent(new Event('change')); // needed for jquery-are-you-sure
        }
      }),
    ],
  });

  const loading = document.querySelector('.editor-loading');
  if (loading) loading.remove();

  return {
    view,
    trimTrailingWhitespace: config.trim_trailing_whitespace,
    togglePalette: palette.togglePalette,
    languages: languageDescriptions,
    compartments: {wordWrap, language, tabSize, indentUnit: indentUnitComp},
  };
}

function setupEditorOptionListeners(textarea: HTMLTextAreaElement, editor: CodemirrorEditor): void {
  const elEditorOptions = textarea.closest('form')!.querySelector('.code-editor-options');
  if (!elEditorOptions) return;

  const {compartments, view} = editor;
  const indentStyleSelect = elEditorOptions.querySelector<HTMLSelectElement>('.js-indent-style-select')!;
  const indentSizeSelect = elEditorOptions.querySelector<HTMLSelectElement>('.js-indent-size-select')!;

  const applyIndentSettings = async (style: string, size: number) => {
    const cm = await importCodemirror();
    view.dispatch({
      effects: [
        compartments.indentUnit.reconfigure(cm.language.indentUnit.of(style === 'tab' ? '\t' : ' '.repeat(size))),
        compartments.tabSize.reconfigure(cm.state.EditorState.tabSize.of(size)),
      ],
    });
  };

  indentStyleSelect.addEventListener('change', () => {
    applyIndentSettings(indentStyleSelect.value, Number(indentSizeSelect.value) || 4);
  });

  indentSizeSelect.addEventListener('change', () => {
    applyIndentSettings(indentStyleSelect.value || 'space', Number(indentSizeSelect.value) || 4);
  });

  elEditorOptions.querySelector('.js-code-find')!.addEventListener('click', async () => {
    const cm = await importCodemirror();
    if (cm.search.searchPanelOpen(view.state)) {
      cm.search.closeSearchPanel(view);
    } else {
      cm.search.openSearchPanel(view);
    }
  });

  elEditorOptions.querySelector('.js-code-command-palette')!.addEventListener('click', () => {
    editor.togglePalette(view);
  });

  elEditorOptions.querySelector<HTMLSelectElement>('.js-line-wrap-select')!.addEventListener('change', async (e) => {
    const target = e.target as HTMLSelectElement;
    const cm = await importCodemirror();
    view.dispatch({
      effects: compartments.wordWrap.reconfigure(target.value === 'on' ? cm.view.EditorView.lineWrapping : []),
    });
  });
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
    writeTab!.click();
  }
}

export async function createCodeEditor(textarea: HTMLTextAreaElement, opts: {filenameInput: HTMLInputElement} | {defaultFilename: string}): Promise<CodemirrorEditor> {
  const filename = 'filenameInput' in opts ? opts.filenameInput.value : opts.defaultFilename;
  const config: CodeEditorConfig = JSON.parse(textarea.getAttribute('data-code-editor-config')!);
  const previewableExts = new Set(config.previewable_extensions || []);
  const lineWrapExts = config.line_wrap_extensions || [];

  const editor = await createCodemirrorEditor(textarea, filename, config);
  setupEditorOptionListeners(textarea, editor);

  // for new files, the autofocus will be on the filename input, for existing files focus the editor
  if (config.is_new_file === false) {
    editor.view.focus();
  }

  const filenameInput = 'filenameInput' in opts ? opts.filenameInput : null;
  if (filenameInput) {
    togglePreviewDisplay(previewableExts.has(extname(filename)));
    filenameInput.addEventListener('input', onInputDebounce(async () => {
      const newFilename = filenameInput.value;
      togglePreviewDisplay(previewableExts.has(extname(newFilename)));
      await updateEditorLanguage(editor, newFilename, lineWrapExts);
    }));
  }

  return editor;
}

async function updateEditorLanguage(editor: CodemirrorEditor, filename: string, lineWrapExts: string[]): Promise<void> {
  const {view: cmView, language: cmLanguage} = await importCodemirror();
  const {compartments, view, languages: editorLanguages} = editor;

  const newLanguage = cmLanguage.LanguageDescription.matchFilename(editorLanguages, filename);
  view.dispatch({
    effects: [
      compartments.wordWrap.reconfigure(
        lineWrapExts.includes(extname(filename)) ? cmView.EditorView.lineWrapping : [],
      ),
      compartments.language.reconfigure(newLanguage ? await newLanguage.load() : []),
    ],
  });
}

export {trimTrailingWhitespaceFromView} from './utils.ts';
