import {basename, extname} from '../utils.js';

const languagesByFilename = {};
const languagesByExt = {};

function getEditorconfig(input) {
  try {
    return JSON.parse(input.dataset.editorconfig);
  } catch (_err) {
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

function updateEditor(monaco, editor, filenameInput) {
  const newFilename = filenameInput.value;
  editor.updateOptions(getOptions(filenameInput));
  const model = editor.getModel();
  const language = model.getModeId();
  const newLanguage = getLanguage(newFilename);
  if (language === newLanguage) return;
  monaco.editor.setModelLanguage(model, newLanguage);
}

export async function createCodeEditor(textarea, filenameInput, previewFileModes) {
  const filename = basename(filenameInput.value);
  const previewLink = document.querySelector('a[data-tab=preview]');
  const markdownExts = (textarea.dataset.markdownFileExts || '').split(',').filter((v) => !!v);
  const lineWrapExts = (textarea.dataset.lineWrapExtensions || '').split(',').filter((v) => !!v);
  const isMarkdown = markdownExts.includes(extname(filename));

  if (isMarkdown && (previewFileModes || []).includes('markdown')) {
    previewLink.dataset.url = previewLink.dataset.url.replace(/(.*)\/.*/i, `$1/markdown`);
    previewLink.style.display = '';
  } else {
    previewLink.style.display = 'none';
  }

  const monaco = await import(/* webpackChunkName: "monaco" */'monaco-editor');
  initLanguages(monaco);

  const container = document.createElement('div');
  container.className = 'monaco-editor-container';
  textarea.parentNode.appendChild(container);

  const editor = monaco.editor.create(container, {
    value: textarea.value,
    language: getLanguage(filename),
    ...await getOptions(filenameInput, lineWrapExts),
  });

  const model = editor.getModel();
  model.onDidChangeContent(() => {
    textarea.value = editor.getValue();
    textarea.dispatchEvent(new Event('change')); // seems to be needed for jquery-are-you-sure
  });

  window.addEventListener('resize', () => {
    editor.layout();
  });

  filenameInput.addEventListener('keyup', () => {
    updateEditor(monaco, editor, filenameInput);
  });

  const loading = document.querySelector('.editor-loading');
  if (loading) loading.remove();

  return editor;
}

async function getOptions(filenameInput, lineWrapExts) {
  const ec = getEditorconfig(filenameInput);
  const detectIndentation = !ec || !ec.indent_style || !ec.indent_size;
  const indentSize = !detectIndentation && ('indent_size' in ec) ? Number(ec.indent_size) : undefined;
  const tabSize = !detectIndentation && ('tab_width' in ec) ? Number(ec.tab_width) : indentSize;

  return {
    detectIndentation,
    insertSpaces: detectIndentation ? undefined : ec.indent_style === 'space',
    tabSize: detectIndentation ? undefined : (tabSize || indentSize),
    theme: document.documentElement.classList.contains('theme-arc-green') ? 'vs-dark' : 'vs',
    useTabStops: ec.indent_style === 'tab',
    wordWrap: lineWrapExts.includes(extname(filenameInput.value)) ? 'on' : 'off',
  };
}
