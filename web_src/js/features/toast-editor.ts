import Editor from '@toast-ui/editor';
import '@toast-ui/editor/dist/toastui-editor.css';

export interface ToastEditorOptions {
  height?: string;
  initialEditType?: 'markdown' | 'wysiwyg';
  previewStyle?: 'tab' | 'vertical';
  usageStatistics?: boolean;
  hideModeSwitch?: boolean;
  toolbarItems?: string[][];
}

export async function createToastEditor(
  textarea: HTMLTextAreaElement, 
  options: ToastEditorOptions = {}
): Promise<Editor> {
  const {
    height = '500px',
    initialEditType = 'wysiwyg',
    previewStyle = 'vertical',
    usageStatistics = false,
    hideModeSwitch = true,
    toolbarItems = [
      ['heading', 'bold', 'italic', 'strike'],
      ['hr', 'quote'],
      ['ul', 'ol', 'task', 'indent', 'outdent'],
      ['table', 'image', 'link'],
      ['code', 'codeblock'],
      ['scrollSync']
    ]
  } = options;

  // Use the existing container from the template
  let container = document.getElementById('toast-editor-container');
  if (!container) {
    // Fallback: create container if not found
    container = document.createElement('div');
    container.id = 'toast-editor-container';
    container.className = 'toast-editor-container';
    container.style.height = height;
    
    if (!textarea.parentNode) {
      throw new Error('Parent node absent');
    }
    
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
        // Sync editor content with textarea
        const content = editor.getMarkdown();
        textarea.value = content;
        textarea.dispatchEvent(new Event('change'));
      }
    }
  });

  // Set initial content
  if (textarea.value) {
    editor.setMarkdown(textarea.value);
  }

  // Hide the original textarea
  textarea.style.display = 'none';

  // Remove loading indicator
  const loading = document.querySelector('.editor-loading');
  if (loading) {
    loading.remove();
  }

  return editor;
}

export function destroyToastEditor(editor: Editor): void {
  editor.destroy();
}
