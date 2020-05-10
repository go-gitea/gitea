import {basename, extname} from '../utils.js';

let monaco;
const languagesByFilename = {};
const languagesByExt = {};

async function getEditorconfig(input) {
  const res = await fetch(`${input.dataset.ecUrlPrefix}${basename(input.value)}`);
  return res.ok ? await res.json() : null;
}

function initLanguages() {
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
  if (languagesByFilename[filename]) {
    return languagesByFilename[filename];
  }

  const ext = `.${extname(filename)}`;
  if (languagesByExt[ext]) {
    return languagesByExt[ext];
  }

  return 'plaintext';
}

function updateEditor(editor, filenameInput) {
  const newFilename = filenameInput.value;
  editor.updateOptions(getOptions(filenameInput));

  const model = editor.getModel();
  const language = model.getModeId();
  const newLanguage = getLanguage(newFilename);
  if (language === newLanguage) return;
  monaco.editor.setModelLanguage(model, newLanguage);
}

export async function createEditor(textarea, filenameInput, previewFileModes) {
  if (!textarea) return;

  const filename = basename(filenameInput.value);
  const extension = extname(filename);
  const extWithDot = `.${extension}`;
  const previewLink = document.querySelector('a[data-tab=preview]');
  const markdownExts = (textarea.dataset.markdownFileExts || '').split(',').filter((v) => !!v);
  const lineWrapExts = (textarea.dataset.lineWrapExtensions || '').split(',').filter((v) => !!v);
  const isMarkdown = markdownExts.includes(extWithDot);

  // If this file is a Markdown extensions, indicate simplemde needs to be used instead
  if (markdownExts.includes(extWithDot)) {
    return 'simplemde';
  }

  // Continue initializing monaco
  if (isMarkdown && (previewFileModes || []).includes('markdown')) {
    previewLink.dataset.url = previewLink.dataset.url.replace(/(.*)\/.*/i, `$1/markdown`);
    previewLink.style.display = '';
  } else {
    previewLink.style.display = 'none';
  }

  monaco = await import(/* webpackChunkName: "monaco" */'monaco-editor');
  initLanguages();

  const container = document.createElement('div');
  const opts = await getOptions(filenameInput, lineWrapExts);

  container.className = 'monaco-editor-container';
  textarea.parentNode.appendChild(container);

  const editor = monaco.editor.create(container, {
    value: textarea.value,
    language: getLanguage(filename),
    ...opts
  });

  const model = editor.getModel();

  model.onDidChangeContent(() => {
    textarea.value = editor.getValue();
    $(textarea).trigger('change'); // seems to be needed for jquery-are-you-sure
  });

  window.addEventListener('resize', () => {
    editor.layout();
  });

  filenameInput.addEventListener('keyup', () => {
    updateEditor(editor, filenameInput);
  });

  $('.editor-loading').remove();

  return editor;
}

async function getOptions(filenameInput, lineWrapExts) {
  const filename = basename(filenameInput.value);
  const extension = extname(filename);
  const extWithDot = `.${extension}`;
  const ec = await getEditorconfig(filenameInput);
  const detectIndentation = !ec || !ec.indent_style || !ec.indent_size;
  const indentSize = !detectIndentation && ('indent_size' in ec) ? Number(ec.indent_size) : 4;

  const opts = {
    detectIndentation,
    useTabStops: !detectIndentation && ec.indent_style === 'tab',
    insertSpaces: !detectIndentation && ec.indent_style === 'space',
    indentSize,
    tabSize: indentSize,
    theme: document.documentElement.classList.contains('theme-arc-green') ? 'vs-dark' : 'vs',
  };

  if (lineWrapExts) {
    opts.wordWrap = lineWrapExts.includes(extWithDot);
  }

  return opts;
}
