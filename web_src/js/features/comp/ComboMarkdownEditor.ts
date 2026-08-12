import '@github/markdown-toolbar-element';
import '@github/text-expander-element';
import {autosize, generateElemId} from '../../utils/dom.ts';
import {EventUploadStateChanged, initTextareaEvents, triggerUploadStateChanged} from './EditorUpload.ts';
import {renderPreviewPanelContent} from '../repo-editor.ts';
import {toggleTasklistCheckbox} from '../../markup/tasklist.ts';
import {initTextExpander} from './TextExpander.ts';
import {POST} from '../../modules/fetch.ts';
import {
  EventEditorContentChanged,
  initTextareaMarkdown,
  replaceTextareaSelection,
  triggerEditorContentChanged,
} from './EditorMarkdown.ts';
import {DropzoneCustomEventReloadFiles, initDropzone} from '../dropzone.ts';
import {createTippy} from '../../modules/tippy.ts';
import {initTabSwitcher} from '../../modules/fomantic/tab.ts';
import {localUserSettings} from '../../modules/user-settings.ts';

export function validateTextareaNonEmpty(textarea: HTMLTextAreaElement) {
  if (!textarea.value) {
    textarea.required = true;
    textarea.closest('form')?.reportValidity();
    return false;
  }
  return true;
}

type ComboMarkdownEditorOptions = {
  editorHeights?: {
    minHeight?: string,
    height?: string,
    maxHeight?: string,
  },
};

type ComboMarkdownEditorTextarea = HTMLTextAreaElement & {_giteaComboMarkdownEditor: any};
type ComboMarkdownEditorContainer = HTMLElement & {_giteaComboMarkdownEditor?: any};

export class ComboMarkdownEditor {
  static EventEditorContentChanged = EventEditorContentChanged;
  static EventUploadStateChanged = EventUploadStateChanged;

  public container: HTMLElement;

  options: ComboMarkdownEditorOptions;

  tabEditor?: HTMLElement;
  tabPreviewer?: HTMLElement;

  textarea!: ComboMarkdownEditorTextarea;
  textareaMarkdownToolbar!: HTMLElement;
  textareaAutosize: any;

  buttonMonospace!: HTMLButtonElement;

  dropzone: HTMLElement | null = null;
  attachedDropzoneInst: any;

  previewMode!: string;
  previewUrl!: string;
  previewContext!: string;

  constructor(container: ComboMarkdownEditorContainer, options: ComboMarkdownEditorOptions = {}) {
    if (container._giteaComboMarkdownEditor) throw new Error('ComboMarkdownEditor already initialized');
    container._giteaComboMarkdownEditor = this;
    this.options = options;
    this.container = container;
  }

  async init() {
    this.setupContainer();
    this.setupTab();
    await this.setupDropzone(); // textarea depends on dropzone
    this.setupTextarea();
  }

  applyEditorHeights(el: HTMLElement, heights: ComboMarkdownEditorOptions['editorHeights']) {
    if (!heights) return;
    if (heights.minHeight) el.style.minHeight = heights.minHeight;
    if (heights.height) el.style.height = heights.height;
    if (heights.maxHeight) el.style.maxHeight = heights.maxHeight;
  }

  setupContainer() {
    this.previewMode = this.container.getAttribute('data-content-mode')!;
    this.previewUrl = this.container.getAttribute('data-preview-url')!;
    this.previewContext = this.container.getAttribute('data-preview-context')!;
    initTextExpander(this.container.querySelector('text-expander')!);
  }

  setupTextarea() {
    this.textarea = this.container.querySelector('.markdown-text-editor')!;
    this.textarea._giteaComboMarkdownEditor = this;
    this.textarea.id = generateElemId(`_combo_markdown_editor_`);
    this.textarea.addEventListener('input', () => triggerEditorContentChanged(this.container));
    this.applyEditorHeights(this.textarea, this.options.editorHeights);

    if (this.textarea.getAttribute('data-disable-autosize') !== 'true') {
      this.textareaAutosize = autosize(this.textarea, {viewportMarginBottom: 130});
    }

    this.textareaMarkdownToolbar = this.container.querySelector('markdown-toolbar')!;
    this.textareaMarkdownToolbar.setAttribute('for', this.textarea.id);
    for (const el of this.textareaMarkdownToolbar.querySelectorAll('.markdown-toolbar-button')) {
      // upstream bug: The role code is never executed in base MarkdownButtonElement https://github.com/github/markdown-toolbar-element/issues/70
      el.setAttribute('role', 'button');
      // the editor usually is in a form, so the buttons should have "type=button", avoiding conflicting with the form's submit.
      if (el.nodeName === 'BUTTON' && !el.getAttribute('type')) el.setAttribute('type', 'button');
    }

    this.buttonMonospace = this.container.querySelector('.markdown-switch-monospace')!;
    this.applyMonospace();
    this.buttonMonospace.addEventListener('click', (e) => {
      e.preventDefault();
      const enabled = !localUserSettings.getBoolean('markdown-editor-monospace');
      localUserSettings.setBoolean('markdown-editor-monospace', enabled);
      applyMonospaceToAllEditors();
    });

    this.initMarkdownButtonTableAdd();

    initTextareaMarkdown(this.textarea);
    initTextareaEvents(this.textarea, this.dropzone);
  }

  async setupDropzone() {
    const dropzoneParentContainer = this.container.getAttribute('data-dropzone-parent-container');
    if (!dropzoneParentContainer) return;
    this.dropzone = this.container.closest(this.container.getAttribute('data-dropzone-parent-container')!)?.querySelector('.dropzone') ?? null;
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
    const elTabular = this.container.querySelector('.ui.tabular');
    if (!elTabular) return;
    this.tabEditor = this.container.querySelector('[data-tab-for="markdown-writer"]')!;
    this.tabPreviewer = this.container.querySelector('[data-tab-for="markdown-previewer"]')!;
    const panelEditor = this.container.querySelector('.ui.tab[data-tab-panel="markdown-writer"]')!;
    const panelPreviewer = this.container.querySelector('.ui.tab[data-tab-panel="markdown-previewer"]')!;

    // Fomantic Tab requires the "data-tab" to be globally unique.
    // So here it uses our defined "data-tab-for" and "data-tab-panel" to generate the "data-tab" attribute for Fomantic.
    const tabIdSuffix = generateElemId();
    this.tabEditor.setAttribute('data-tab', `markdown-writer-${tabIdSuffix}`);
    this.tabPreviewer.setAttribute('data-tab', `markdown-previewer-${tabIdSuffix}`);
    panelEditor.setAttribute('data-tab', `markdown-writer-${tabIdSuffix}`);
    panelPreviewer.setAttribute('data-tab', `markdown-previewer-${tabIdSuffix}`);
    initTabSwitcher(elTabular);

    this.tabEditor.addEventListener('click', () => {
      requestAnimationFrame(() => {
        this.focus();
      });
    });

    this.tabPreviewer.addEventListener('click', async () => {
      const formData = new FormData();
      formData.append('mode', this.previewMode);
      formData.append('context', this.previewContext);
      formData.append('text', this.value());
      const response = await POST(this.previewUrl, {data: formData});
      const data = await response.text();
      renderPreviewPanelContent(panelPreviewer, data);
      // enable task list checkboxes in preview and sync state back to the editor
      for (const checkbox of panelPreviewer.querySelectorAll<HTMLInputElement>('.task-list-item input[type=checkbox]')) {
        checkbox.disabled = false;
        checkbox.addEventListener('input', () => {
          const position = parseInt(checkbox.getAttribute('data-source-position')!) + 1;
          const newContent = toggleTasklistCheckbox(this.value(), position, checkbox.checked);
          if (newContent === null) {
            checkbox.checked = !checkbox.checked;
            return;
          }
          this.value(newContent);
          triggerEditorContentChanged(this.container);
        });
      }
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
    const addTableButton = this.container.querySelector('.markdown-button-table-add')!;
    const addTablePanel = this.container.querySelector('.markdown-add-table-panel')!;
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

    addTablePanel.querySelector('.ui.button.primary')!.addEventListener('click', () => {
      let rows = parseInt(addTablePanel.querySelector<HTMLInputElement>('.add-table-rows')!.value);
      let cols = parseInt(addTablePanel.querySelector<HTMLInputElement>('.add-table-cols')!.value);
      rows = Math.max(1, Math.min(100, rows));
      cols = Math.max(1, Math.min(100, cols));
      replaceTextareaSelection(this.textarea, `\n${this.generateMarkdownTable(rows, cols)}\n\n`);
      addTablePanelTippy.hide();
    });
  }

  switchTabToEditor() {
    this.tabEditor!.click(); // when this function is called, the tab must exist
  }

  value(): string;
  value(v: string): void;
  value(v?: string): string | void {
    if (v === undefined) {
      return this.textarea.value;
    }
    this.textarea.value = v;
    this.textareaAutosize?.resizeToFit();
  }

  focus() {
    this.textarea.focus();
  }

  moveCursorToEnd() {
    this.textarea.focus();
    this.textarea.setSelectionRange(this.textarea.value.length, this.textarea.value.length);
  }

  applyMonospace() {
    const enabled = localUserSettings.getBoolean('markdown-editor-monospace');
    const text = this.buttonMonospace.getAttribute(enabled ? 'data-disable-text' : 'data-enable-text')!;
    this.textarea.classList.toggle('tw-font-mono', enabled);
    this.buttonMonospace.setAttribute('data-tooltip-content', text);
    this.buttonMonospace.setAttribute('aria-checked', String(enabled));
  }
}

function applyMonospaceToAllEditors() {
  const editors = document.querySelectorAll<ComboMarkdownEditorContainer>('.combo-markdown-editor');
  for (const editorContainer of editors) {
    const editor = getComboMarkdownEditor(editorContainer);
    if (editor) editor.applyMonospace();
  }
}

export function getComboMarkdownEditor(el: any): ComboMarkdownEditor | null {
  if (!el) return null;
  if (el.length) el = el[0];
  return el._giteaComboMarkdownEditor;
}

export async function initComboMarkdownEditor(container: HTMLElement, options: ComboMarkdownEditorOptions = {}) {
  if (!container) {
    throw new Error('initComboMarkdownEditor: container is null');
  }
  const editor = new ComboMarkdownEditor(container, options);
  await editor.init();
  return editor;
}
