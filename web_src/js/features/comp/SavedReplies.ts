import { GET } from '../../modules/fetch.ts';
import { svg } from '../../svg.ts';
import { triggerEditorContentChanged } from './EditorMarkdown.ts';
import { createElementFromAttrs, createElementFromHTML } from '../../utils/dom.ts';
import type { ComboMarkdownEditor } from './ComboMarkdownEditor.ts';

const { appSubUrl } = window.config;

type SavedReply = {
  id: number;
  title: string;
  content: string;
};

type DialogLocale = {
  dialogTitle: string;
  searchPlaceholder: string;
  noResults: string;
  close: string;
  createNew: string;
  errorLoading: string;
};

const dialogTitleId = 'saved-replies-dialog-title';

let sharedDialog: HTMLDialogElement | null = null;
let sharedSearchInputController: AbortController | null = null;
let sharedOpenerButton: HTMLElement | null = null;

export function insertSavedReply(value: string, cursorPos: number, content: string): { value: string; pos: number } {
  if (!value) {
    return { value: content, pos: content.length };
  }

  const safeCursorPos = Math.max(0, Math.min(cursorPos, value.length));
  const lineEnd = value.indexOf('\n', safeCursorPos);
  const insertAt = lineEnd === -1 ? value.length : lineEnd;
  const insertText = `\n${content}`;
  return {
    value: `${value.slice(0, insertAt)}${insertText}${value.slice(insertAt)}`,
    pos: insertAt + insertText.length,
  };
}

function renderSavedReplies(listEl: Element, replies: SavedReply[], editor: ComboMarkdownEditor, noResultsText: string) {
  listEl.replaceChildren();
  if (replies.length === 0) {
    listEl.append(createElementFromAttrs('div', { class: 'saved-replies-empty' }, noResultsText));
    return;
  }
  for (const reply of replies) {
    const titleEl = createElementFromAttrs('div', { class: 'saved-replies-item-title' }, reply.title);
    const bodyEl = createElementFromAttrs('div', { class: 'saved-replies-item-body' },
      reply.content.length > 100 ? `${reply.content.substring(0, 100)}…` : reply.content,
    );

    const itemEl = createElementFromAttrs<HTMLButtonElement>('button', {
      type: 'button',
      class: 'saved-replies-item',
    }, titleEl, bodyEl);

    itemEl.addEventListener('click', () => {
      sharedDialog?.close();
      if (editor.easyMDE) {
        const cm = editor.easyMDE.codemirror;
        const cursorPos = cm.indexFromPos(cm.getCursor('from'));
        const result = insertSavedReply(editor.value(), cursorPos, reply.content);
        editor.value(result.value);
        cm.focus();
        cm.setCursor(cm.posFromIndex(result.pos));
      } else {
        const result = insertSavedReply(editor.textarea.value, editor.textarea.selectionStart, reply.content);
        editor.value(result.value);
        editor.textarea.focus();
        editor.textarea.setSelectionRange(result.pos, result.pos);
      }
      triggerEditorContentChanged(editor.container);
    });
    listEl.append(itemEl);
  }
}

function createDialog(): HTMLDialogElement {
  const dialog = createElementFromAttrs<HTMLDialogElement>('dialog', {
    class: 'saved-replies-dialog',
    'aria-labelledby': dialogTitleId,
  });

  // Header
  const headerTitle = createElementFromAttrs('h4', { class: 'saved-replies-dialog-title', id: dialogTitleId });
  const header = createElementFromAttrs('div', { class: 'saved-replies-dialog-header' }, headerTitle);

  // Search
  const searchInput = createElementFromAttrs<HTMLInputElement>('input', {
    type: 'text',
    class: 'saved-replies-search-input',
    autofocus: true,
  });
  const searchSection = createElementFromAttrs('div', { class: 'saved-replies-dialog-search' }, searchInput);

  // List
  const listEl = createElementFromAttrs('div', { class: 'saved-replies-dialog-list' });

  // Footer
  const cancelBtn = createElementFromAttrs<HTMLButtonElement>('button', {
    class: 'ui cancel button saved-replies-dialog-cancel',
    type: 'button',
  });
  const createLink = createElementFromAttrs<HTMLAnchorElement>('a', {
    class: 'ui primary button saved-replies-create-link',
    href: `${appSubUrl}/user/settings/saved_replies`,
  });
  const footer = createElementFromAttrs('div', {
    class: 'actions saved-replies-dialog-footer',
  }, cancelBtn, createLink);

  dialog.append(header, searchSection, listEl, footer);

  cancelBtn.addEventListener('click', () => dialog.close());
  dialog.addEventListener('click', (event) => {
    if (event.target === dialog) dialog.close();
  });
  dialog.addEventListener('close', () => {
    document.body.classList.remove('tw-overflow-hidden');
    sharedOpenerButton?.focus({ preventScroll: true });
  });

  document.body.append(dialog);
  return dialog;
}

function updateDialogLocale(dialog: HTMLDialogElement, locale: DialogLocale): void {
  dialog.querySelector('.saved-replies-dialog-title')!.textContent = locale.dialogTitle;

  const searchInput = dialog.querySelector<HTMLInputElement>('.saved-replies-search-input')!;
  searchInput.placeholder = locale.searchPlaceholder;
  searchInput.setAttribute('aria-label', locale.searchPlaceholder);

  const cancelBtn = dialog.querySelector<HTMLElement>('.saved-replies-dialog-cancel')!;
  cancelBtn.replaceChildren();
  cancelBtn.append(createElementFromHTML<SVGElement>(svg('octicon-x')), ` ${locale.close}`);

  const createLink = dialog.querySelector<HTMLElement>('.saved-replies-create-link')!;
  createLink.replaceChildren();
  createLink.append(createElementFromHTML<SVGElement>(svg('octicon-plus')), ` ${locale.createNew}`);
}

async function openSavedRepliesDialog(btn: HTMLElement, editor: ComboMarkdownEditor): Promise<void> {
  sharedOpenerButton = btn;

  const locale: DialogLocale = {
    dialogTitle: btn.getAttribute('data-dialog-title')!,
    searchPlaceholder: btn.getAttribute('data-search-placeholder')!,
    noResults: btn.getAttribute('data-no-results')!,
    close: btn.getAttribute('data-close')!,
    createNew: btn.getAttribute('data-create-new')!,
    errorLoading: btn.getAttribute('data-error-loading')!,
  };

  if (!sharedDialog) {
    sharedDialog = createDialog();
  }
  const dialog = sharedDialog;

  updateDialogLocale(dialog, locale);

  const searchInput = dialog.querySelector<HTMLInputElement>('.saved-replies-search-input')!;
  const listEl = dialog.querySelector('.saved-replies-dialog-list')!;

  // show loading
  listEl.replaceChildren();
  listEl.append(createElementFromAttrs('div', { class: 'saved-replies-loading' }, '…'));
  searchInput.value = '';
  document.body.classList.add('tw-overflow-hidden');
  dialog.showModal();

  try {
    const resp = await GET(`${appSubUrl}/user/settings/saved_replies/json`);
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const allReplies: SavedReply[] = await resp.json();

    renderSavedReplies(listEl, allReplies, editor, locale.noResults);
    sharedSearchInputController?.abort();
    sharedSearchInputController = new AbortController();
    searchInput.addEventListener('input', () => {
      const query = searchInput.value.toLowerCase().trim();
      const filtered = query ? allReplies.filter((r) => r.title.toLowerCase().includes(query)) : allReplies;
      renderSavedReplies(listEl, filtered, editor, locale.noResults);
    }, { signal: sharedSearchInputController.signal });
    searchInput.focus({ preventScroll: true });
  } catch {
    listEl.replaceChildren();
    listEl.append(createElementFromAttrs('div', { class: 'saved-replies-empty' }, locale.errorLoading));
  }
}

export function initSavedRepliesButton(editor: ComboMarkdownEditor): void {
  const btn = editor.container.querySelector<HTMLElement>('.markdown-button-saved-replies');
  if (!btn) return;

  btn.addEventListener('click', (e) => {
    e.preventDefault();
    openSavedRepliesDialog(btn, editor);
  });
}
