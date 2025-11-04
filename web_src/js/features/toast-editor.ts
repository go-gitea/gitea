// @ts-expect-error - @toast-ui/editor has type definition issues with package.json exports
import Editor from '@toast-ui/editor';
import '@toast-ui/editor/dist/toastui-editor.css';

export type ToastEditorOptions = {
  height?: string;
  initialEditType?: 'markdown' | 'wysiwyg';
  previewStyle?: 'tab' | 'vertical';
  usageStatistics?: boolean;
  hideModeSwitch?: boolean;
  toolbarItems?: string[][];
};

export async function createToastEditor(
  textarea: HTMLTextAreaElement,
  options: ToastEditorOptions = {},
): Promise<Editor> {
  const {
    height = '500px',
    initialEditType = 'wysiwyg',
    previewStyle = 'vertical',
    usageStatistics = false,
    hideModeSwitch = false,   // must be false to show the tabs
    toolbarItems = [
      ['heading', 'bold', 'italic'],
      ['indent', 'outdent', 'code', 'link'],
      ['ul', 'ol', 'task'],
      ['image', 'table'],
    ],
  } = options;

  // Use the existing container from the template
  let container = document.querySelector<HTMLElement>('#toast-editor-container');
  if (!container) {
    container = document.createElement('div');
    container.id = 'toast-editor-container';
    container.className = 'toast-editor-container';
    container.style.height = height;
    if (!textarea.parentNode) throw new Error('Parent node absent');
    textarea.parentNode.append(container);
  } else {
    container.style.height = height;
  }

  // Initialize Toast UI Editor
  const editor = new Editor({
    el: container,
    height,
    initialEditType,
    previewStyle,
    usageStatistics,
    hideModeSwitch,
    toolbarItems,
    events: {
      change: () => {
        const content = editor.getMarkdown();
        textarea.value = content;
        textarea.dispatchEvent(new Event('change'));
      },
    },
  });

  // Set initial content
  if (textarea.value) {
    editor.setMarkdown(textarea.value);
  }

  // Rename mode switch labels
  const switchEl = container.querySelector('.toastui-editor-mode-switch');
  if (switchEl) {
    for (const el of switchEl.querySelectorAll('.tab-item')) {
      if (el.textContent?.trim() === 'WYSIWYG') el.textContent = 'Visual editor';
      else if (el.textContent?.trim() === 'Markdown') el.textContent = 'Source editor';
    }
  }

  // Hide the original textarea
  textarea.style.display = 'none';

  // Remove loading indicator if present
  const loading = document.querySelector('.editor-loading');
  if (loading) loading.remove();

  return editor;
}

export function destroyToastEditor(editor: Editor): void {
  editor.destroy();
}
