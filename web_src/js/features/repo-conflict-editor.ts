import {registerGlobalInitFunc} from '../modules/observer.ts';
import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast, showInfoToast} from '../modules/toast.ts';
import {createCodeEditor} from '../modules/codeeditor/main.ts';
import type {CodemirrorEditor} from '../modules/codeeditor/main.ts';

type FileStatus = 'unresolved' | 'resolved';

type FileState = {
  status: FileStatus;
  content: string | null; // null = not yet fetched from server
};

registerGlobalInitFunc('initRepoConflictEditor', async (elRoot: HTMLElement) => {
  const conflictedFiles: string[] = JSON.parse(elRoot.getAttribute('data-conflicted-files')!);
  const fileContentUrl = elRoot.getAttribute('data-file-content-url')!;
  const resolveUrl = elRoot.getAttribute('data-resolve-url')!;
  const initialFile = elRoot.getAttribute('data-initial-file')!;
  const defaultCommitMsg = elRoot.getAttribute('data-default-commit-msg')!;
  const msgHasMarkers = elRoot.getAttribute('data-msg-has-markers')!;
  const msgMarkResolved = elRoot.getAttribute('data-msg-mark-resolved')!;
  const msgCommitMerge = elRoot.getAttribute('data-msg-commit-merge')!;
  const hintHasMarkers = elRoot.getAttribute('data-hint-has-markers')!;
  const hintClean = elRoot.getAttribute('data-hint-clean')!;
  const hintResolved = elRoot.getAttribute('data-hint-resolved')!;

  const elFileList = elRoot.querySelector<HTMLElement>('.conflict-file-list')!;
  const elEditorArea = elRoot.querySelector<HTMLElement>('.conflict-editor-area')!;
  const elCurrentFilePath = elEditorArea.querySelector<HTMLElement>('.conflict-current-file-path')!;
  const elMarkResolvedBtn = elEditorArea.querySelector<HTMLButtonElement>('.conflict-mark-resolved-btn')!;
  const elTextarea = elEditorArea.querySelector<HTMLTextAreaElement>('#edit_area')!;
  const elCommitMessage = elRoot.querySelector<HTMLInputElement>('.conflict-commit-message')!;
  const elCommitBtn = elRoot.querySelector<HTMLButtonElement>('.conflict-commit-btn')!;
  const elHint = elEditorArea.querySelector<HTMLElement>('.conflict-editor-hint')!;

  // Per-file state: unresolved content (with markers) and resolved content
  const fileStates = new Map<string, FileState>(
    conflictedFiles.map((p) => [p, {status: 'unresolved', content: null}]),
  );
  let currentFile = '';
  let editor: CodemirrorEditor | null = null;
  let switching = false; // guard against concurrent switches

  // ---- CodeMirror helpers ----

  function editorGetContent(): string {
    return editor!.view.state.doc.toString();
  }

  function editorSetContent(text: string): void {
    editor!.view.dispatch({
      changes: {from: 0, to: editor!.view.state.doc.length, insert: text},
      selection: {anchor: 0},
    });
    editor!.view.scrollDOM.scrollTop = 0;
  }

  function hasConflictMarkers(text: string): boolean {
    return text.includes('<<<<<<<') || text.includes('>>>>>>>');
  }

  // ---- UI update helpers ----

  function resolvedCount(): number {
    let n = 0;
    for (const s of fileStates.values()) if (s.status === 'resolved') n++;
    return n;
  }

  function updateSidebar(): void {
    for (const [fpath, state] of fileStates.entries()) {
      const item = elFileList.querySelector<HTMLElement>(`[data-path=${JSON.stringify(fpath)}]`);
      if (!item) continue;
      item.classList.toggle('active', fpath === currentFile);
      item.classList.toggle('resolved', state.status === 'resolved');
      item.classList.toggle('unresolved', state.status === 'unresolved');
      const icon = item.querySelector<HTMLElement>('.conflict-file-status-icon')!;
      if (state.status === 'resolved') {
        icon.textContent = '✓'; // checkmark (U+2713)
        icon.title = 'Resolved';
      } else {
        icon.textContent = '○'; // hollow circle (U+25CB)
        icon.title = 'Unresolved';
      }
    }
    const done = resolvedCount();
    const total = conflictedFiles.length;
    elCommitBtn.disabled = done < total;
    elCommitBtn.textContent = `${msgCommitMerge} (${done}/${total})`;
  }

  function updateHint(force?: FileStatus | 'markers'): void {
    const effectiveState = force ?? (fileStates.get(currentFile)?.status);
    const content = editorGetContent();
    if (effectiveState === 'resolved') {
      elHint.className = 'conflict-editor-hint resolved tw-px-3 tw-py-1 tw-text-xs tw-flex-shrink-0';
      elHint.setAttribute('data-hint', hintResolved);
    } else if (hasConflictMarkers(content)) {
      elHint.className = 'conflict-editor-hint has-markers tw-px-3 tw-py-1 tw-text-xs tw-flex-shrink-0';
      elHint.setAttribute('data-hint', hintHasMarkers);
    } else {
      elHint.className = 'conflict-editor-hint clean tw-px-3 tw-py-1 tw-text-xs tw-flex-shrink-0';
      elHint.setAttribute('data-hint', hintClean);
    }
  }

  // ---- File fetching & switching ----

  async function fetchFileContent(filePath: string): Promise<string> {
    const resp = await GET(`${fileContentUrl}?path=${encodeURIComponent(filePath)}`);
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const data = await resp.json() as {content: string};
    return data.content;
  }

  /** Save current editor content and switch to newPath. */
  async function switchToFile(newPath: string): Promise<void> {
    if (switching || newPath === currentFile) return;
    switching = true;
    elMarkResolvedBtn.disabled = true;

    // Persist latest editor text for the file we are leaving
    if (currentFile) {
      const prev = fileStates.get(currentFile)!;
      fileStates.set(currentFile, {...prev, content: editorGetContent()});
    }

    try {
      let state = fileStates.get(newPath)!;
      // Lazy-load conflict content on first visit
      if (state.content === null) {
        state = {...state, content: await fetchFileContent(newPath)};
        fileStates.set(newPath, state);
      }
      currentFile = newPath;
      editorSetContent(state.content!);
      await editor!.updateFilename(newPath.split('/').pop() || newPath);
      elCurrentFilePath.textContent = newPath;
      elCurrentFilePath.title = newPath;
      updateSidebar();
      updateHint();
    } catch (err) {
      showErrorToast(`Failed to load ${newPath}: ${String(err)}`);
    } finally {
      switching = false;
      elMarkResolvedBtn.disabled = false;
    }
  }

  // ---- Actions ----

  function markCurrentResolved(): void {
    if (!currentFile) return;
    const content = editorGetContent();
    if (hasConflictMarkers(content)) {
      showErrorToast(msgHasMarkers);
      return;
    }
    // Mark resolved and save resolved content
    fileStates.set(currentFile, {status: 'resolved', content});
    const shortName = currentFile.split('/').pop() || currentFile;
    showInfoToast(msgMarkResolved.replace('%s', shortName));
    updateSidebar();
    updateHint();

    // Auto-advance to the next unresolved file
    for (const [fpath, state] of fileStates.entries()) {
      if (state.status === 'unresolved') {
        switchToFile(fpath);
        return;
      }
    }
  }

  async function commitAllResolved(): Promise<void> {
    // Persist current editor state before committing
    if (currentFile) {
      const prev = fileStates.get(currentFile)!;
      fileStates.set(currentFile, {...prev, content: editorGetContent()});
    }

    const files = Array.from(fileStates.entries())
      .filter(([, s]) => s.status === 'resolved' && s.content !== null)
      .map(([p, s]) => ({path: p, content: s.content!}));

    if (files.length === 0) return;

    const message = elCommitMessage.value.trim() || defaultCommitMsg;
    elCommitBtn.disabled = true;

    try {
      const resp = await POST(resolveUrl, {data: {message, files}});
      if (resp.ok) {
        const data = await resp.json() as {redirect: string};
        window.location.href = data.redirect;
      } else {
        const data = await resp.json() as {error?: string};
        showErrorToast(data.error ?? 'Commit failed');
        elCommitBtn.disabled = false;
      }
    } catch {
      showErrorToast('Network error, please try again.');
      elCommitBtn.disabled = false;
    }
  }

  // ---- Bootstrap ----

  editor = await createCodeEditor(elTextarea);

  // File list: click to switch
  elFileList.addEventListener('click', (e) => {
    const item = (e.target as HTMLElement).closest<HTMLElement>('[data-path]');
    if (item) switchToFile(item.getAttribute('data-path')!);
  });

  // Mark-resolved button
  elMarkResolvedBtn.addEventListener('click', markCurrentResolved);

  // Commit button
  elCommitBtn.addEventListener('click', commitAllResolved);

  // Hint updates while typing
  editor.view.dom.addEventListener('input', () => updateHint());

  // Load the initial file
  await switchToFile(initialFile);
});

// initRepoConflictEditor is the exported init function registered in index.ts.
// The actual work is done via registerGlobalInitFunc when the element is observed.
export function initRepoConflictEditor() {}
