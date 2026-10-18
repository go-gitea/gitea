import {html, htmlEscape, htmlRaw} from '../../utils/html.ts';
import {createElementFromHTML} from '../../utils/dom.ts';
import {pathEscapeSegments} from '../../utils/url.ts';
import {basename} from '../../utils.ts';
import {svg, svgRaw} from '../../svg.ts';
import {trString} from '../i18n.ts';
import {diffTreeStore} from '../diff-file.ts';
import type {DiffSide} from './parse.ts';
import type {DiffStreamFile} from './stream.ts';
import type {DiffFileEntry, DiffViewerConfig, DiffViewerLocale} from './types.ts';

// a file of the commit before ("old") or after ("new") the changes, only for configs with a repo link
export function fileUrl(config: DiffViewerConfig, view: 'src' | 'raw' | 'media', side: DiffSide, path: string): string {
  const commitId = side === 'old' ? config.beforeCommitId! : config.afterCommitId;
  return `${config.repoLink}/${view}/commit/${encodeURIComponent(commitId)}/${pathEscapeSegments(path)}`;
}

export function applyFolded(entry: DiffFileEntry, folded: boolean) {
  entry.folded = folded;
  const box = entry.element;
  if (!box) return;
  box.setAttribute('data-folded', String(folded));
  box.querySelector('.fold-file')!.replaceChildren(createElementFromHTML(svg(`octicon-chevron-${folded ? 'right' : 'down'}`, 18)));
}

function entryModeLabel(file: DiffStreamFile, locale: DiffViewerLocale): string {
  const names: Record<string, string> = {
    '040000': locale.modeDirectory, '100644': locale.modeNormal, '100755': locale.modeExecutable,
    '120000': locale.modeSymlink, '160000': locale.modeSubmodule,
  };
  const name = (mode: string) => names[mode] ?? mode;
  if (file.mode && file.oldMode) return trString(locale.modeChangedFormat, name(file.oldMode), name(file.mode));
  if (file.mode && file.mode !== '100644') return name(file.mode);
  return '';
}

function fileMenuHtml(entry: DiffFileEntry, config: DiffViewerConfig, locale: DiffViewerLocale): string {
  const {file} = entry;
  if (file.submodule) return '';
  let items = '';
  if (!file.isBinary && !file.tooLarge && !entry.missing) {
    items += html`
      <button type="button" class="unescape-button item" data-unicode-content-selector="#diff-${file.nameHash}">${locale.unescape}</button>
      <button type="button" class="escape-button tw-hidden item" data-unicode-content-selector="#diff-${file.nameHash}">${locale.escape}</button>
    `;
  }
  items += html`<button type="button" class="item" data-clipboard-text="${basename(file.name)}">${locale.copyFilename}</button>`;
  if (config.repoLink) {
    items += html`<a class="item" rel="nofollow" href="${fileUrl(config, 'src', file.type === 'deleted' ? 'old' : 'new', file.name)}">${locale.viewFile}</a>`;
    if (file.type !== 'deleted' && config.editFileLink) {
      const returnUri = encodeURIComponent(`${config.editReturnUri}#diff-${file.nameHash}`);
      items += html`<a class="item" rel="nofollow" href="${config.editFileLink}${pathEscapeSegments(file.name)}?return_uri=${returnUri}">${locale.editFile}</a>`;
    }
  }
  return html`<button type="button" class="btn tw-p-1" data-global-init="initDiffHeaderPopupMenu">${htmlRaw(svg('octicon-kebab-horizontal', 18))}</button><div class="tippy-target">${htmlRaw(items)}</div>`;
}

export function buildFileBox(entry: DiffFileEntry, config: DiffViewerConfig, locale: DiffViewerLocale, hasRenderedView: boolean): HTMLElement {
  const {file} = entry;
  const treeItem = diffTreeStore().fullNameMap[file.name];
  const fileName = file.oldName !== file.name ? `${file.oldName} → ${file.name}` : file.name;
  const lines = entry.hunks.flatMap((hunk) => hunk.lines);
  const additions = lines.filter((line) => line.type === 'add').length;
  const deletions = lines.filter((line) => line.type === 'del').length;
  let stats = '';
  if (file.isBinary) {
    stats = htmlEscape(locale.bin);
  } else if (additions + deletions) {
    stats = html`
      <div class="flex-text-block tw-flex-shrink-0 tw-text-[13px]">
        <span>${additions ? htmlRaw`<strong class="tw-text-diff-added-fg">+${additions}</strong>` : ''} ${deletions ? htmlRaw`<strong class="tw-text-diff-removed-fg">-${deletions}</strong>` : ''}</span>
        <span class="diff-stats-bar"><div class="diff-stats-add-bar" style="width: ${100 * additions / (additions + deletions)}%"></div></span>
      </div>
    `;
  }
  const changedLabel = treeItem?.HasChanged ? html`<span class="changed-since-last-review unselectable not-mobile">${locale.changedSinceLastReview}</span>` : '';
  const viewedForm = config.canReviewFiles ? html`
    <label class="viewed-file-form unselectable${treeItem?.IsViewed ? ' viewed-file-checked-form' : ''}">
      <input type="checkbox" autocomplete="off"${htmlRaw(treeItem?.IsViewed ? ' checked' : '')}> ${locale.viewed}
    </label>
  ` : '';
  const labels = [
    file.lfs ? 'LFS' : '',
    file.isGenerated ? locale.generated : '',
    file.isVendored ? locale.vendored : '',
    entryModeLabel(file, locale),
  ].filter(Boolean).map((label) => html`<span class="ui label">${label}</span>`).join('');
  const protectedLabel = file.isProtected ? html`<span class="ui basic label">${locale.protected}</span>` : '';
  // the global init "initDiffFileViewToggle" switches between the source and the rendered view
  const viewToggle = hasRenderedView ? html`
    <div class="ui compact icon buttons" data-global-init="initDiffFileViewToggle">
      <button type="button" class="ui tiny basic button file-view-toggle" data-toggle-selector="#diff-source-${file.nameHash}" data-tooltip-content="${locale.viewSource}">${htmlRaw(svg('octicon-code'))}</button>
      <button type="button" class="ui tiny basic button file-view-toggle active" data-toggle-selector="#diff-rendered-${file.nameHash}" data-tooltip-content="${locale.viewRendered}">${htmlRaw(svg('octicon-file'))}</button>
    </div>
  ` : '';
  return createElementFromHTML<HTMLElement>(html`
    <div class="diff-file-box file-content tab-size-${file.tabSize} tw-absolute tw-inset-x-0" id="diff-${file.nameHash}" data-old-filename="${file.oldName}" data-new-filename="${file.name}">
      <div class="diff-file-header sticky-2nd-row ui top attached header">
        <div class="diff-file-name tw-flex tw-flex-1 tw-items-center tw-gap-1 tw-flex-wrap">
          <div class="flex-text-block">
            <button type="button" class="fold-file btn interact-bg tw-flex-shrink-0 tw-p-1"></button>
            <a class="muted file-link tw-font-mono" title="${fileName}" href="#diff-${file.nameHash}">${fileName}</a>
          </div>
          <button type="button" class="btn interact-fg tw-p-2 tw-shrink-0" data-clipboard-text="${file.name}" data-tooltip-content="${locale.copyPath}">${htmlRaw(svg('octicon-copy', 14))}</button>
          ${htmlRaw(labels)}
        </div>
        <div class="diff-file-header-actions flex-text-block tw-justify-end tw-flex-wrap">${htmlRaw(stats)}${htmlRaw(viewToggle)}${htmlRaw(protectedLabel)}${htmlRaw(changedLabel)}${htmlRaw(viewedForm)}${htmlRaw(fileMenuHtml(entry, config, locale))}</div>
      </div>
      <div class="diff-file-body ui attached unstackable table segment"></div>
    </div>
  `);
}

export function submoduleNotice(file: DiffStreamFile, locale: DiffViewerLocale): HTMLElement {
  const {submodule, name, type} = file;
  const link = (href: string | undefined, text: string) => (href ? html`<a href="${href}">${text}</a>` : htmlEscape(text));
  const shortSha = (sha = '') => sha.substring(0, 10);
  const nameHtml = link(submodule!.repoLink, name);
  let message: string;
  if (type === 'deleted') {
    message = trString(locale.submoduleDeleted, nameHtml, link(submodule!.oldLink, shortSha(submodule!.oldCommit)));
  } else if (type === 'added') {
    message = trString(locale.submoduleAdded, nameHtml, link(submodule!.newLink, shortSha(submodule!.newCommit)));
  } else {
    const range = `${shortSha(submodule!.oldCommit)}...${shortSha(submodule!.newCommit)}`;
    message = trString(locale.submoduleUpdated, nameHtml, link(submodule!.compareLink, range));
  }
  return createElementFromHTML(html`<div class="tw-p-3 flex-text-block">${svgRaw('octicon-file-submodule')} <span>${htmlRaw(message)}</span></div>`);
}
