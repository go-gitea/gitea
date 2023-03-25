import {basename, extname, isObject, isDarkTheme} from '../utils.js';
import {debounce} from 'throttle-debounce';

const languagesByFilename = {};
const languagesByExt = {};

const baseOptions = {
  fontFamily: 'var(--fonts-monospace)',
  fontSize: 14, // https://github.com/microsoft/monaco-editor/issues/2242
  guides: {bracketPairs: false, indentation: false},
  links: false,
  minimap: {enabled: false},
  occurrencesHighlight: false,
  overviewRulerLanes: 0,
  renderLineHighlight: 'all',
  renderLineHighlightOnlyWhenFocus: true,
  renderWhitespace: 'none',
  rulers: false,
  scrollbar: {horizontalScrollbarSize: 6, verticalScrollbarSize: 6},
  scrollBeyondLastLine: false,
  automaticLayout: true,
};

function getEditorconfig(input) {
  try {
    return JSON.parse(input.getAttribute('data-editorconfig'));
  } catch {
    return null;
  }
}

function initLanguages(monaco) {
  for (const {filenames, extensions, id} of monaco.languages.getLanguages()) {
    for (const filename of filenames || []) {
      languagesByFilename[filename] = id;
    }
    for (const extension of extensions || []) {
      languagesByExt[extension] = id;
    }
  }
}

function getLanguage(filename) {
  return languagesByFilename[filename] || languagesByExt[extname(filename)] || 'plaintext';
}

function updateEditor(monaco, editor, filename, lineWrapExts) {
  editor.updateOptions(getFileBasedOptions(filename, lineWrapExts));
  const model = editor.getModel();
  const language = model.getLanguageId();
  const newLanguage = getLanguage(filename);
  if (language !== newLanguage) monaco.editor.setModelLanguage(model, newLanguage);
}

// export editor for customization - https://github.com/go-gitea/gitea/issues/10409
function exportEditor(editor) {
  if (!window.codeEditors) window.codeEditors = [];
  if (!window.codeEditors.includes(editor)) window.codeEditors.push(editor);
}

export async function createMonaco(textarea, filename, editorOpts) {
  const monaco = await import(/* webpackChunkName: "monaco" */'monaco-editor');

  initLanguages(monaco);
  let {language, ...other} = editorOpts;
  if (!language) language = getLanguage(filename);

  const container = document.createElement('div');
  container.className = 'monaco-editor-container';
  textarea.parentNode.appendChild(container);

  // https://github.com/microsoft/monaco-editor/issues/2427
  const styles = window.getComputedStyle(document.documentElement);
  const getProp = (name) => styles.getPropertyValue(name).trim();

  monaco.editor.defineTheme('gitea', {
    base: isDarkTheme() ? 'vs-dark' : 'vs',
    inherit: true,
    rules: [
      {
        background: getProp('--color-code-bg'),
      }
    ],
    colors: {
      'editor.background': getProp('--color-code-bg'),
      'editor.foreground': getProp('--color-text'),
      'editor.inactiveSelectionBackground': getProp('--color-primary-light-4'),
      'editor.lineHighlightBackground': getProp('--color-editor-line-highlight'),
      'editor.selectionBackground': getProp('--color-primary-light-3'),
      'editor.selectionForeground': getProp('--color-primary-light-3'),
      'editorLineNumber.background': getProp('--color-code-bg'),
      'editorLineNumber.foreground': getProp('--color-secondary-dark-6'),
      'editorWidget.background': getProp('--color-body'),
      'editorWidget.border': getProp('--color-secondary'),
      'input.background': getProp('--color-input-background'),
      'input.border': getProp('--color-input-border'),
      'input.foreground': getProp('--color-input-text'),
      'scrollbar.shadow': getProp('--color-shadow'),
      'progressBar.background': getProp('--color-primary'),
    }
  });

  // Quick fix: https://github.com/microsoft/monaco-editor/issues/2962
  monaco.languages.register({id: 'vs.editor.nullLanguage'});
  monaco.languages.setLanguageConfiguration('vs.editor.nullLanguage', {});

  const editor = monaco.editor.create(container, {
    value: textarea.value,
    theme: 'gitea',
    language,
    ...other,
  });

  const model = editor.getModel();
  model.onDidChangeContent(() => {
    textarea.value = editor.getValue();
    textarea.dispatchEvent(new Event('change')); // seems to be needed for jquery-are-you-sure
  });

  exportEditor(editor);

  const loading = document.querySelector('.editor-loading');
  if (loading) loading.remove();

  return {monaco, editor};
}

function getFileBasedOptions(filename, lineWrapExts) {
  return {
    wordWrap: (lineWrapExts || []).includes(extname(filename)) ? 'on' : 'off',
  };
}

function togglePreviewDisplay(previewable) {
  const previewTab = document.querySelector('a[data-tab="preview"]');
  if (!previewTab) return;

  if (previewable) {
    const newUrl = (previewTab.getAttribute('data-url') || '').replace(/(.*)\/.*/i, `$1/markup`);
    previewTab.setAttribute('data-url', newUrl);
    previewTab.style.display = '';
  } else {
    previewTab.style.display = 'none';
    // If the "preview" tab was active, user changes the filename to a non-previewable one,
    // then the "preview" tab becomes inactive (hidden), so the "write" tab should become active
    if (previewTab.classList.contains('active')) {
      const writeTab = document.querySelector('a[data-tab="write"]');
      writeTab.click();
    }
  }
}

export async function createCodeEditor(textarea, filenameInput) {
  const filename = basename(filenameInput.value);
  const previewableExts = new Set((textarea.getAttribute('data-previewable-extensions') || '').split(','));
  const lineWrapExts = (textarea.getAttribute('data-line-wrap-extensions') || '').split(',');
  const previewable = previewableExts.has(extname(filename));
  const editorConfig = getEditorconfig(filenameInput);

  togglePreviewDisplay(previewable);

  const {monaco, editor} = await createMonaco(textarea, filename, {
    ...baseOptions,
    ...getFileBasedOptions(filenameInput.value, lineWrapExts),
    ...getEditorConfigOptions(editorConfig),
  });

  filenameInput.addEventListener('input', debounce(500, () => {
    const filename = filenameInput.value;
    const previewable = previewableExts.has(extname(filename));
    togglePreviewDisplay(previewable);
    updateEditor(monaco, editor, filename, lineWrapExts);
  }));

  return editor;
}

function getEditorConfigOptions(ec) {
  if (!isObject(ec)) return {};

  const opts = {};
  opts.detectIndentation = !('indent_style' in ec) || !('indent_size' in ec);
  if ('indent_size' in ec) opts.indentSize = Number(ec.indent_size);
  if ('tab_width' in ec) opts.tabSize = Number(ec.tab_width) || opts.indentSize;
  if ('max_line_length' in ec) opts.rulers = [Number(ec.max_line_length)];
  opts.trimAutoWhitespace = ec.trim_trailing_whitespace === true;
  opts.insertSpaces = ec.indent_style === 'space';
  opts.useTabStops = ec.indent_style === 'tab';
  return opts;
}
