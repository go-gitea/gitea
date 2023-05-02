import $ from 'jquery';
import {initMarkupContent} from '../markup/content.js';
import {validateTextareaNonEmpty, initComboMarkdownEditor} from './comp/ComboMarkdownEditor.js';

const {csrfToken} = window.config;

async function initRepoWikiFormEditor() {
  const $editArea = $('.repository.wiki .combo-markdown-editor textarea');
  if (!$editArea.length) return;

  const $form = $('.repository.wiki.new .ui.form');
  const $editorContainer = $form.find('.combo-markdown-editor');
  let editor;

  let renderRequesting = false;
  let lastContent;
  const renderEasyMDEPreview = function () {
    if (renderRequesting) return;

    const $previewFull = $editorContainer.find('.EasyMDEContainer .editor-preview-active');
    const $previewSide = $editorContainer.find('.EasyMDEContainer .editor-preview-active-side');
    const $previewTarget = $previewSide.length ? $previewSide : $previewFull;
    const newContent = $editArea.val();
    if (editor && $previewTarget.length && lastContent !== newContent) {
      renderRequesting = true;
      $.post(editor.previewUrl, {
        _csrf: csrfToken,
        mode: editor.previewMode,
        context: editor.previewContext,
        text: newContent,
        wiki: editor.previewWiki,
      }).done((data) => {
        lastContent = newContent;
        $previewTarget.html(`<div class="markup ui segment">${data}</div>`);
        initMarkupContent();
      }).always(() => {
        renderRequesting = false;
        setTimeout(renderEasyMDEPreview, 1000);
      });
    } else {
      setTimeout(renderEasyMDEPreview, 1000);
    }
  };
  renderEasyMDEPreview();

  editor = await initComboMarkdownEditor($editorContainer, {
    useScene: 'wiki',
    // EasyMDE has some problems of height definition, it has inline style height 300px by default, so we also use inline styles to override it.
    // And another benefit is that we only need to write the style once for both editors.
    // TODO: Move height style to CSS after EasyMDE removal.
    editorHeights: {minHeight: '300px', height: 'calc(100vh - 600px)'},
    previewMode: 'gfm',
    previewWiki: true,
    easyMDEOptions: {
      previewRender: (_content, previewTarget) => previewTarget.innerHTML, // disable builtin preview render
      toolbar: ['bold', 'italic', 'strikethrough', '|',
        'heading-1', 'heading-2', 'heading-3', 'heading-bigger', 'heading-smaller', '|',
        'gitea-code-inline', 'code', 'quote', '|', 'gitea-checkbox-empty', 'gitea-checkbox-checked', '|',
        'unordered-list', 'ordered-list', '|',
        'link', 'image', 'table', 'horizontal-rule', '|',
        'clean-block', 'preview', 'fullscreen', 'side-by-side', '|', 'gitea-switch-to-textarea'
      ],
    },
  });

  $form.on('submit', () => {
    if (!validateTextareaNonEmpty($editArea)) {
      return false;
    }
  });
}

export function initRepoWikiForm() {
  initRepoWikiFormEditor();
}
