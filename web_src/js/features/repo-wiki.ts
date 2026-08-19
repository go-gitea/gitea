import {validateTextareaNonEmpty, initComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {fomanticMobileScreen} from '../modules/fomantic.ts';
import {POST} from '../modules/fetch.ts';
import type {ComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

async function initRepoWikiForm(form: HTMLFormElement) {
  const editorContainer = form.querySelector<HTMLElement>('.combo-markdown-editor')!;
  const editArea = editorContainer.querySelector<HTMLTextAreaElement>('textarea')!;
  let editor: ComboMarkdownEditor;

  let renderRequesting = false;
  let lastContent: string = '';
  const renderEasyMDEPreview = async function () {
    if (renderRequesting) return;

    const previewFull = editorContainer.querySelector('.EasyMDEContainer .editor-preview-active');
    const previewSide = editorContainer.querySelector('.EasyMDEContainer .editor-preview-active-side');
    const previewTarget = previewSide || previewFull;
    const newContent = editArea.value;
    if (editor && previewTarget && lastContent !== newContent) {
      renderRequesting = true;
      const formData = new FormData();
      formData.append('mode', editor.previewMode);
      formData.append('context', editor.previewContext);
      formData.append('text', newContent);
      try {
        const response = await POST(editor.previewUrl, {data: formData});
        const data = await response.text();
        lastContent = newContent;
        previewTarget.innerHTML = html`<div class="render-content markup ui segment">${htmlRaw(data)}</div>`;
      } catch (error) {
        console.error('Error rendering preview:', error);
      } finally {
        renderRequesting = false;
        setTimeout(renderEasyMDEPreview, 1000);
      }
    } else {
      setTimeout(renderEasyMDEPreview, 1000);
    }
  };
  renderEasyMDEPreview();

  editor = await initComboMarkdownEditor(editorContainer, {
    // EasyMDE has some problems of height definition, it has inline style height 300px by default, so we also use inline styles to override it.
    // And another benefit is that we only need to write the style once for both editors.
    // TODO: Move height style to CSS after EasyMDE removal.
    editorHeights: {minHeight: '300px', height: 'calc(100vh - 600px)'},
    easyMDEOptions: {
      previewRender: (_content, previewTarget) => previewTarget.innerHTML, // disable builtin preview render
      toolbar: ['bold', 'italic', 'strikethrough', '|',
        'heading-1', 'heading-2', 'heading-3', 'heading-bigger', 'heading-smaller', '|',
        'gitea-code-inline', 'code', 'quote', '|', 'gitea-checkbox-empty', 'gitea-checkbox-checked', '|',
        'unordered-list', 'ordered-list', '|',
        'link', 'image', 'table', 'horizontal-rule', '|',
        'preview', 'fullscreen', 'side-by-side', '|', 'gitea-switch-to-textarea',
      ] as any, // to use custom toolbar buttons
    },
  });

  form.addEventListener('submit', (e) => {
    if (!validateTextareaNonEmpty(editArea)) {
      e.preventDefault();
      e.stopPropagation();
    }
  });
}

export function initRepoWiki() {
  registerGlobalInitFunc('initRepoWikiSidebarToc', (el) => {
    const collapseWikiTocForMobile = (collapse: boolean) => {
      if (collapse) el.querySelector('details')?.removeAttribute('open');
    };
    fomanticMobileScreen.addEventListener('change', (e) => collapseWikiTocForMobile(e.matches));
    collapseWikiTocForMobile(fomanticMobileScreen.matches);
  });
  registerGlobalInitFunc('initRepoWikiForm', initRepoWikiForm);
}
