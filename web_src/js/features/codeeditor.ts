import tinycolor from 'tinycolor2';
import {extname, isDarkTheme} from '../utils.ts';
import type MonacoNamespace from 'monaco-editor';

type Monaco = typeof MonacoNamespace;
type IStandaloneCodeEditor = MonacoNamespace.editor.IStandaloneCodeEditor;
type IEditorOptions = MonacoNamespace.editor.IEditorOptions;
type IGlobalEditorOptions = MonacoNamespace.editor.IGlobalEditorOptions;
type ITextModelUpdateOptions = MonacoNamespace.editor.ITextModelUpdateOptions;
type MonacoOpts = IEditorOptions & IGlobalEditorOptions & ITextModelUpdateOptions;

const languagesByFilename: Record<string, string> = {};
const languagesByExt: Record<string, string> = {};

function initLanguages(monaco: Monaco): void {
  for (const {filenames, extensions, id} of monaco.languages.getLanguages()) {
    for (const filename of filenames || []) {
      languagesByFilename[filename] = id;
    }
    for (const extension of extensions || []) {
      languagesByExt[extension] = id;
    }
    if (id === 'typescript') {
      monaco.languages.typescript.typescriptDefaults.setCompilerOptions({
        // this is needed to suppress error annotations in tsx regarding missing --jsx flag.
        jsx: monaco.languages.typescript.JsxEmit.Preserve,
      });
    }
  }
}

function getLanguage(filename: string): string {
  return languagesByFilename[filename] || languagesByExt[extname(filename)] || 'plaintext';
}

// export editor for customization - https://github.com/go-gitea/gitea/issues/10409
function exportEditor(editor: IStandaloneCodeEditor): void {
  if (!window.codeEditors) window.codeEditors = [];
  if (!window.codeEditors.includes(editor)) window.codeEditors.push(editor);
}

function updateTheme(monaco: Monaco): void {
  // https://github.com/microsoft/monaco-editor/issues/2427
  // also, monaco can only parse 6-digit hex colors, so we convert the colors to that format
  const styles = window.getComputedStyle(document.documentElement);
  const getColor = (name: string) => tinycolor(styles.getPropertyValue(name).trim()).toString('hex6');

  monaco.editor.defineTheme('gitea', {
    base: isDarkTheme() ? 'vs-dark' : 'vs',
    inherit: true,
    rules: [
      {
        background: getColor('--color-code-bg'),
        token: '',
      },
    ],
    colors: {
      'editor.background': getColor('--color-code-bg'),
      'editor.foreground': getColor('--color-text'),
      'editor.inactiveSelectionBackground': getColor('--color-primary-light-4'),
      'editor.lineHighlightBackground': getColor('--color-editor-line-highlight'),
      'editor.selectionBackground': getColor('--color-primary-light-3'),
      'editor.selectionForeground': getColor('--color-primary-light-3'),
      'editorLineNumber.background': getColor('--color-code-bg'),
      'editorLineNumber.foreground': getColor('--color-secondary-dark-6'),
      'editorWidget.background': getColor('--color-body'),
      'editorWidget.border': getColor('--color-secondary'),
      'input.background': getColor('--color-input-background'),
      'input.border': getColor('--color-input-border'),
      'input.foreground': getColor('--color-input-text'),
      'scrollbar.shadow': getColor('--color-shadow-opaque'),
      'progressBar.background': getColor('--color-primary'),
      'focusBorder': '#0000', // prevent blue border
    },
  });
}

type CreateMonacoOpts = MonacoOpts & {language?: string};

export async function createMonaco(textarea: HTMLTextAreaElement, filename: string, opts: CreateMonacoOpts): Promise<{monaco: Monaco, editor: IStandaloneCodeEditor}> {
  const monaco = await import(/* webpackChunkName: "monaco" */'monaco-editor');

  initLanguages(monaco);
  let {language, ...other} = opts;
  if (!language) language = getLanguage(filename);

  const container = document.createElement('div');
  container.className = 'monaco-editor-container';
  if (!textarea.parentNode) throw new Error('Parent node absent');
  textarea.parentNode.append(container);

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    updateTheme(monaco);
  });
  updateTheme(monaco);

  const model = monaco.editor.createModel(textarea.value, language, monaco.Uri.file(filename));

  const editor = monaco.editor.create(container, {
    model,
    theme: 'gitea',
    ...other,
  });

  monaco.editor.addKeybindingRules([
    {keybinding: monaco.KeyCode.Enter, command: null}, // disable enter from accepting code completion
  ]);

  model.onDidChangeContent(() => {
    textarea.value = editor.getValue({
      preserveBOM: true,
      lineEnding: '',
    });
    textarea.dispatchEvent(new Event('change')); // seems to be needed for jquery-are-you-sure
  });

  exportEditor(editor);

  const loading = document.querySelector('.editor-loading');
  if (loading) loading.remove();

  return {monaco, editor};
}
