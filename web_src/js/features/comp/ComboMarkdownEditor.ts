import '@github/markdown-toolbar-element';
import '@github/text-expander-element';
import {attachTribute} from '../tribute.ts';
import {hideElem, showElem, autosize, isElemVisible} from '../../utils/dom.ts';
import {
  EventUploadStateChanged,
  initEasyMDEPaste,
  initTextareaEvents,
  triggerUploadStateChanged,
} from './EditorUpload.ts';
import {handleGlobalEnterQuickSubmit} from './QuickSubmit.ts';
import {renderPreviewPanelContent} from '../repo-editor.ts';
import {easyMDEToolbarActions} from './EasyMDEToolbarActions.ts';
import {initTextExpander} from './TextExpander.ts';
import {showErrorToast} from '../../modules/toast.ts';
import {POST} from '../../modules/fetch.ts';
import {
  EventEditorContentChanged,
  initTextareaMarkdown,
  textareaInsertText,
  triggerEditorContentChanged,
} from './EditorMarkdown.ts';
import {DropzoneCustomEventReloadFiles, initDropzone} from '../dropzone.ts';
import {createTippy} from '../../modules/tippy.ts';
import {fomanticQuery} from '../../modules/fomantic/base.ts';
import type EasyMDE from 'easymde';

let elementIdCounter = 0;

/**
 * validate if the given textarea is non-empty.
 * @param {HTMLElement} textarea - The textarea element to be validated.
 * @returns {boolean} returns true if validation succeeded.
 */
export function validateTextareaNonEmpty(textarea) {
  // When using EasyMDE, the original edit area HTML element is hidden, breaking HTML5 input validation.
  // The workaround (https://github.com/sparksuite/simplemde-markdown-editor/issues/324) doesn't work with contenteditable, so we just show an alert.
  if (!textarea.value) {
    if (isElemVisible(textarea)) {
      textarea.required = true;
      const form = textarea.closest('form');
      form?.reportValidity();
    } else {
      // The alert won't hurt users too much, because we are dropping the EasyMDE and the check only occurs in a few places.
      showErrorToast('Require non-empty content');
    }
    return false;
  }
  return true;
}

type ComboMarkdownEditorOptions = {
  editorHeights?: {minHeight?: string, height?: string, maxHeight?: string},
  easyMDEOptions?: EasyMDE.Options,
};

export class ComboMarkdownEditor {
  static EventEditorContentChanged = EventEditorContentChanged;
  static EventUploadStateChanged = EventUploadStateChanged;

  public container : HTMLElement;

  options: ComboMarkdownEditorOptions;

  tabEditor: HTMLElement;
  tabPreviewer: HTMLElement;

  supportEasyMDE: boolean;
  easyMDE: any;
  easyMDEToolbarActions: any;
  easyMDEToolbarDefault: any;

  textarea: HTMLTextAreaElement & {_giteaComboMarkdownEditor: any};
  textareaMarkdownToolbar: HTMLElement;
  textareaAutosize: any;

  dropzone: HTMLElement;
  attachedDropzoneInst: any;

  previewMode: string;
  previewUrl: string;
  previewContext: string;

  constructor(container, options:ComboMarkdownEditorOptions = {}) {
    if (container._giteaComboMarkdownEditor) throw new Error('ComboMarkdownEditor already initialized');
    container._giteaComboMarkdownEditor = this;
    this.options = options;
    this.container = container;
  }

  async init() {
    this.prepareEasyMDEToolbarActions();
    this.setupContainer();
    this.setupTab();
    await this.setupDropzone(); // textarea depends on dropzone
    this.setupTextarea();

    await this.switchToUserPreference();
  }

  applyEditorHeights(el, heights) {
    if (!heights) return;
    if (heights.minHeight) el.style.minHeight = heights.minHeight;
    if (heights.height) el.style.height = heights.height;
    if (heights.maxHeight) el.style.maxHeight = heights.maxHeight;
  }

  setupContainer() {
    this.supportEasyMDE = this.container.getAttribute('data-support-easy-mde') === 'true';
    this.previewMode = this.container.getAttribute('data-content-mode');
    this.previewUrl = this.container.getAttribute('data-preview-url');
    this.previewContext = this.container.getAttribute('data-preview-context');
    initTextExpander(this.container.querySelector('text-expander'));
  }

  setupTextarea() {
    this.textarea = this.container.querySelector('.markdown-text-editor');
    this.textarea._giteaComboMarkdownEditor = this;
    this.textarea.id = `_combo_markdown_editor_${String(elementIdCounter++)}`;
    this.textarea.addEventListener('input', () => triggerEditorContentChanged(this.container));
    this.applyEditorHeights(this.textarea, this.options.editorHeights);

    if (this.textarea.getAttribute('data-disable-autosize') !== 'true') {
      this.textareaAutosize = autosize(this.textarea, {viewportMarginBottom: 130});
    }

    this.textareaMarkdownToolbar = this.container.querySelector('markdown-toolbar');
    this.textareaMarkdownToolbar.setAttribute('for', this.textarea.id);
    for (const el of this.textareaMarkdownToolbar.querySelectorAll('.markdown-toolbar-button')) {
      // upstream bug: The role code is never executed in base MarkdownButtonElement https://github.com/github/markdown-toolbar-element/issues/70
      el.setAttribute('role', 'button');
      // the editor usually is in a form, so the buttons should have "type=button", avoiding conflicting with the form's submit.
      if (el.nodeName === 'BUTTON' && !el.getAttribute('type')) el.setAttribute('type', 'button');
    }

    const monospaceButton = this.container.querySelector('.markdown-switch-monospace');
    const monospaceEnabled = localStorage?.getItem('markdown-editor-monospace') === 'true';
    const monospaceText = monospaceButton.getAttribute(monospaceEnabled ? 'data-disable-text' : 'data-enable-text');
    monospaceButton.setAttribute('data-tooltip-content', monospaceText);
    monospaceButton.setAttribute('aria-checked', String(monospaceEnabled));
    monospaceButton.addEventListener('click', (e) => {
      e.preventDefault();
      const enabled = localStorage?.getItem('markdown-editor-monospace') !== 'true';
      localStorage.setItem('markdown-editor-monospace', String(enabled));
      this.textarea.classList.toggle('tw-font-mono', enabled);
      const text = monospaceButton.getAttribute(enabled ? 'data-disable-text' : 'data-enable-text');
      monospaceButton.setAttribute('data-tooltip-content', text);
      monospaceButton.setAttribute('aria-checked', String(enabled));
    });

    if (this.supportEasyMDE) {
      const easymdeButton = this.container.querySelector('.markdown-switch-easymde');
      easymdeButton.addEventListener('click', async (e) => {
        e.preventDefault();
        this.userPreferredEditor = 'easymde';
        await this.switchToEasyMDE();
      });
    }

    this.initMarkdownButtonTableAdd();

    initTextareaMarkdown(this.textarea);
    initTextareaEvents(this.textarea, this.dropzone);
  }

  async setupDropzone() {
    const dropzoneParentContainer = this.container.getAttribute('data-dropzone-parent-container');
    if (!dropzoneParentContainer) return;
    this.dropzone = this.container.closest(this.container.getAttribute('data-dropzone-parent-container'))?.querySelector('.dropzone');
    if (!this.dropzone) return;

    this.attachedDropzoneInst = await initDropzone(this.dropzone);
    // dropzone events
    // * "processing" means a file is being uploaded
    // * "queuecomplete" means all files have been uploaded
    this.attachedDropzoneInst.on('processing', () => triggerUploadStateChanged(this.container));
    this.attachedDropzoneInst.on('queuecomplete', () => triggerUploadStateChanged(this.container));
  }

  dropzoneGetFiles() {
    if (!this.dropzone) return null;
    return Array.from(this.dropzone.querySelectorAll<HTMLInputElement>('.files [name=files]'), (el) => el.value);
  }

  dropzoneReloadFiles() {
    if (!this.dropzone) return;
    this.attachedDropzoneInst.emit(DropzoneCustomEventReloadFiles);
  }

  dropzoneSubmitReload() {
    if (!this.dropzone) return;
    this.attachedDropzoneInst.emit('submit');
    this.attachedDropzoneInst.emit(DropzoneCustomEventReloadFiles);
  }

  isUploading() {
    if (!this.dropzone) return false;
    return this.attachedDropzoneInst.getQueuedFiles().length || this.attachedDropzoneInst.getUploadingFiles().length;
  }

  setupTab() {
    const tabs = this.container.querySelectorAll<HTMLElement>('.tabular.menu > .item');
    if (!tabs.length) return;

    // Fomantic Tab requires the "data-tab" to be globally unique.
    // So here it uses our defined "data-tab-for" and "data-tab-panel" to generate the "data-tab" attribute for Fomantic.
    this.tabEditor = Array.from(tabs).find((tab) => tab.getAttribute('data-tab-for') === 'markdown-writer');
    this.tabPreviewer = Array.from(tabs).find((tab) => tab.getAttribute('data-tab-for') === 'markdown-previewer');
    this.tabEditor.setAttribute('data-tab', `markdown-writer-${elementIdCounter}`);
    this.tabPreviewer.setAttribute('data-tab', `markdown-previewer-${elementIdCounter}`);

    const panelEditor = this.container.querySelector('.ui.tab[data-tab-panel="markdown-writer"]');
    const panelPreviewer = this.container.querySelector('.ui.tab[data-tab-panel="markdown-previewer"]');
    panelEditor.setAttribute('data-tab', `markdown-writer-${elementIdCounter}`);
    panelPreviewer.setAttribute('data-tab', `markdown-previewer-${elementIdCounter}`);
    elementIdCounter++;

    this.tabEditor.addEventListener('click', () => {
      requestAnimationFrame(() => {
        this.focus();
      });
    });

    fomanticQuery(tabs).tab();

    this.tabPreviewer.addEventListener('click', async () => {
      const formData = new FormData();
      formData.append('mode', this.previewMode);
      formData.append('context', this.previewContext);
      formData.append('text', this.value());
      const response = await POST(this.previewUrl, {data: formData});
      const data = await response.text();
      renderPreviewPanelContent(panelPreviewer, data);
    });
  }

  generateMarkdownTable(rows: number, cols: number): string {
    const tableLines = [];
    tableLines.push(
      `| ${'Header '.repeat(cols).trim().split(' ').join(' | ')} |`,
      `| ${'--- '.repeat(cols).trim().split(' ').join(' | ')} |`,
    );
    for (let i = 0; i < rows; i++) {
      tableLines.push(`| ${'Cell '.repeat(cols).trim().split(' ').join(' | ')} |`);
    }
    return tableLines.join('\n');
  }

  initMarkdownButtonTableAdd() {
    const addTableButton = this.container.querySelector('.markdown-button-table-add');
    const addTablePanel = this.container.querySelector('.markdown-add-table-panel');
    // here the tippy can't attach to the button because the button already owns a tippy for tooltip
    const addTablePanelTippy = createTippy(addTablePanel, {
      content: addTablePanel,
      trigger: 'manual',
      placement: 'bottom',
      hideOnClick: true,
      interactive: true,
      getReferenceClientRect: () => addTableButton.getBoundingClientRect(),
    });
    addTableButton.addEventListener('click', () => addTablePanelTippy.show());

    addTablePanel.querySelector('.ui.button.primary').addEventListener('click', () => {
      let rows = parseInt(addTablePanel.querySelector<HTMLInputElement>('[name=rows]').value);
      let cols = parseInt(addTablePanel.querySelector<HTMLInputElement>('[name=cols]').value);
      rows = Math.max(1, Math.min(100, rows));
      cols = Math.max(1, Math.min(100, cols));
      textareaInsertText(this.textarea, `\n${this.generateMarkdownTable(rows, cols)}\n\n`);
      addTablePanelTippy.hide();
    });
  }

  switchTabToEditor() {
    this.tabEditor.click();
  }

  prepareEasyMDEToolbarActions() {
    this.easyMDEToolbarDefault = [
      'bold', 'italic', 'strikethrough', '|', 'heading-1', 'heading-2', 'heading-3',
      'heading-bigger', 'heading-smaller', '|', 'code', 'quote', '|', 'gitea-checkbox-empty',
      'gitea-checkbox-checked', '|', 'unordered-list', 'ordered-list', '|', 'link', 'image',
      'table', 'horizontal-rule', '|', 'gitea-switch-to-textarea',
    ];
  }

  parseEasyMDEToolbar(easyMde: typeof EasyMDE, actions) {
    this.easyMDEToolbarActions = this.easyMDEToolbarActions || easyMDEToolbarActions(easyMde, this);
    const processed = [];
    for (const action of actions) {
      const actionButton = this.easyMDEToolbarActions[action];
      if (!actionButton) throw new Error(`Unknown EasyMDE toolbar action ${action}`);
      processed.push(actionButton);
    }
    return processed;
  }

  async switchToUserPreference() {
    if (this.userPreferredEditor === 'easymde' && this.supportEasyMDE) {
      await this.switchToEasyMDE();
    } else {
      this.switchToTextarea();
    }
  }

  switchToTextarea() {
    if (!this.easyMDE) return;
    showElem(this.textareaMarkdownToolbar);
    if (this.easyMDE) {
      this.easyMDE.toTextArea();
      this.easyMDE = null;
    }
  }

  async switchToEasyMDE() {
    if (this.easyMDE) return;
    // EasyMDE's CSS should be loaded via webpack config, otherwise our own styles can not overwrite the default styles.
    const {default: EasyMDE} = await import(/* webpackChunkName: "easymde" */'easymde');
    const easyMDEOpt: EasyMDE.Options = {
      autoDownloadFontAwesome: false,
      element: this.textarea,
      forceSync: true,
      renderingConfig: {singleLineBreaks: false},
      indentWithTabs: false,
      tabSize: 4,
      spellChecker: false,
      inputStyle: 'contenteditable', // nativeSpellcheck requires contenteditable
      nativeSpellcheck: true,
      ...this.options.easyMDEOptions,
    };
    easyMDEOpt.toolbar = this.parseEasyMDEToolbar(EasyMDE, easyMDEOpt.toolbar ?? this.easyMDEToolbarDefault);

    this.easyMDE = new EasyMDE(easyMDEOpt);
    this.easyMDE.codemirror.on('change', () => triggerEditorContentChanged(this.container));
    this.easyMDE.codemirror.setOption('extraKeys', {
      'Cmd-Enter': (cm) => handleGlobalEnterQuickSubmit(cm.getTextArea()),
      'Ctrl-Enter': (cm) => handleGlobalEnterQuickSubmit(cm.getTextArea()),
      Enter: (cm) => {
        const tributeContainer = document.querySelector<HTMLElement>('.tribute-container');
        if (!tributeContainer || tributeContainer.style.display === 'none') {
          cm.execCommand('newlineAndIndent');
        }
      },
      Up: (cm) => {
        const tributeContainer = document.querySelector<HTMLElement>('.tribute-container');
        if (!tributeContainer || tributeContainer.style.display === 'none') {
          return cm.execCommand('goLineUp');
        }
      },
      Down: (cm) => {
        const tributeContainer = document.querySelector<HTMLElement>('.tribute-container');
        if (!tributeContainer || tributeContainer.style.display === 'none') {
          return cm.execCommand('goLineDown');
        }
      },
    });
    this.applyEditorHeights(this.container.querySelector('.CodeMirror-scroll'), this.options.editorHeights);
    await attachTribute(this.easyMDE.codemirror.getInputField(), {mentions: true, emoji: true});
    if (this.dropzone) {
      initEasyMDEPaste(this.easyMDE, this.dropzone);
    }
    hideElem(this.textareaMarkdownToolbar);
  }

  value(v = undefined) {
    if (v === undefined) {
      if (this.easyMDE) {
        return this.easyMDE.value();
      }
      return this.textarea.value;
    }

    if (this.easyMDE) {
      this.easyMDE.value(v);
    } else {
      this.textarea.value = v;
    }
    this.textareaAutosize?.resizeToFit();
  }

  focus() {
    if (this.easyMDE) {
      this.easyMDE.codemirror.focus();
    } else {
      this.textarea.focus();
    }
  }

  moveCursorToEnd() {
    this.textarea.focus();
    this.textarea.setSelectionRange(this.textarea.value.length, this.textarea.value.length);
    if (this.easyMDE) {
      this.easyMDE.codemirror.focus();
      this.easyMDE.codemirror.setCursor(this.easyMDE.codemirror.lineCount(), 0);
    }
  }

  get userPreferredEditor() {
    return window.localStorage.getItem(`markdown-editor-${this.previewMode ?? 'default'}`);
  }
  set userPreferredEditor(s) {
    window.localStorage.setItem(`markdown-editor-${this.previewMode ?? 'default'}`, s);
  }
}

export function getComboMarkdownEditor(el) {
  if (!el) return null;
  if (el.length) el = el[0];
  return el._giteaComboMarkdownEditor;
}

export async function initComboMarkdownEditor(container: HTMLElement, options:ComboMarkdownEditorOptions = {}) {
  if (!container) {
    throw new Error('initComboMarkdownEditor: container is null');
  }
  const editor = new ComboMarkdownEditor(container, options);
  await editor.init();
  return editor;
}
