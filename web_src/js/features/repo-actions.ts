import {createApp} from 'vue';
import RepoActionView from '../components/RepoActionView.vue';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {html} from '../utils/html.ts';
import {GET} from '../modules/fetch.ts';
import {activePageTimerRefresh, createElementFromHTML} from '../utils/dom.ts';

export function updateWorkflowBadgeFields(form: HTMLElement, branch: string): void {
  const badgeURLParsed = new URL(form.getAttribute('data-badge-url')!);
  badgeURLParsed.searchParams.set('branch', branch);

  const badgeURL = badgeURLParsed.href;
  const workflowURL = form.getAttribute('data-workflow-url')!;
  const displayName = form.getAttribute('data-workflow-display-name')!;
  const markdownAltText = displayName.replaceAll(/[\\[\]]/g, (c) => `\\${c}`);

  form.querySelector<HTMLImageElement>('[data-workflow-badge-image]')!.src = badgeURL;
  form.querySelector<HTMLInputElement>('#workflow-badge-url')!.value = badgeURL;
  form.querySelector<HTMLTextAreaElement>('#workflow-badge-markdown')!.value = `[![${markdownAltText}](${badgeURL})](${workflowURL})`;
  form.querySelector<HTMLTextAreaElement>('#workflow-badge-html')!.value = html`<a href="${workflowURL}"><img src="${badgeURL}" alt="${displayName}"></a>`;
}

function initWorkflowBadgeForm(form: HTMLElement): void {
  const branchInput = form.querySelector<HTMLInputElement>('[data-workflow-badge-branch]')!;
  branchInput.addEventListener('change', () => updateWorkflowBadgeFields(form, branchInput.value));
  updateWorkflowBadgeFields(form, branchInput.value);
}

export function initRepositoryActions() {
  registerGlobalInitFunc('initWorkflowBadgeForm', initWorkflowBadgeForm);
  initRepositoryActionsView();
  registerGlobalInitFunc('initActionRunsList', initActionRunsList);
}

function initRepositoryActionsView() {
  const el = document.querySelector('#repo-action-view');
  if (!el) return;

  // TODO: the parent element's full height doesn't work well now,
  // but we can not pollute the global style at the moment, only fix the height problem for pages with this component
  const parentFullHeight = document.querySelector<HTMLElement>('body > div.full.height');
  if (parentFullHeight) parentFullHeight.classList.add('tw-pb-0');

  const view = createApp(RepoActionView, {
    jobId: parseInt(el.getAttribute('data-job-id')!),
    actionsViewUrl: el.getAttribute('data-actions-view-url'),
    locale: {
      approve: el.getAttribute('data-locale-approve'),
      cancel: el.getAttribute('data-locale-cancel'),
      rerun: el.getAttribute('data-locale-rerun'),
      rerun_all: el.getAttribute('data-locale-rerun-all'),
      rerun_failed: el.getAttribute('data-locale-rerun-failed'),
      latest: el.getAttribute('data-locale-latest'),
      latestAttempt: el.getAttribute('data-locale-latest-attempt'),
      attempt: el.getAttribute('data-locale-attempt'),
      scheduled: el.getAttribute('data-locale-runs-scheduled'),
      commit: el.getAttribute('data-locale-runs-commit'),
      pushedBy: el.getAttribute('data-locale-runs-pushed-by'),
      summary: el.getAttribute('data-locale-summary'),
      allJobs: el.getAttribute('data-locale-all-jobs'),
      jobSummaries: el.getAttribute('data-locale-job-summaries'),
      expandCallerJobs: el.getAttribute('data-locale-expand-caller-jobs'),
      collapseCallerJobs: el.getAttribute('data-locale-collapse-caller-jobs'),
      triggeredVia: el.getAttribute('data-locale-triggered-via'),
      rerunTriggered: el.getAttribute('data-locale-rerun-triggered'),
      backToPullRequest: el.getAttribute('data-locale-back-to-pull-request'),
      backToWorkflow: el.getAttribute('data-locale-back-to-workflow'),
      statusLabel: el.getAttribute('data-locale-status-label'),
      totalDuration: el.getAttribute('data-locale-total-duration'),
      artifactsTitle: el.getAttribute('data-locale-artifacts-title'),
      artifactExpired: el.getAttribute('data-locale-artifact-expired'),
      artifactExpiresAt: el.getAttribute('data-locale-artifact-expires-at'),
      confirmDeleteArtifact: el.getAttribute('data-locale-confirm-delete-artifact'),
      showTimeStamps: el.getAttribute('data-locale-show-timestamps'),
      showLogSeconds: el.getAttribute('data-locale-show-log-seconds'),
      showFullScreen: el.getAttribute('data-locale-show-full-screen'),
      downloadLogs: el.getAttribute('data-locale-download-logs'),
      copyOutput: el.getAttribute('data-locale-copy-output'),
      status: {
        unknown: el.getAttribute('data-locale-status-unknown'),
        waiting: el.getAttribute('data-locale-status-waiting'),
        running: el.getAttribute('data-locale-status-running'),
        cancelling: el.getAttribute('data-locale-status-cancelling'),
        success: el.getAttribute('data-locale-status-success'),
        failure: el.getAttribute('data-locale-status-failure'),
        cancelled: el.getAttribute('data-locale-status-cancelled'),
        skipped: el.getAttribute('data-locale-status-skipped'),
        blocked: el.getAttribute('data-locale-status-blocked'),
      },
      logsAlwaysAutoScroll: el.getAttribute('data-locale-logs-always-auto-scroll'),
      logsAlwaysExpandRunning: el.getAttribute('data-locale-logs-always-expand-running'),
      workflowFile: el.getAttribute('data-locale-workflow-file'),
      workflowFileNoPermission: el.getAttribute('data-locale-workflow-file-no-permission'),
      runDetails: el.getAttribute('data-locale-run-details'),
      workflowDependencies: el.getAttribute('data-locale-workflow-dependencies'),
      graphJobsCount1: el.getAttribute('data-locale-graph-jobs-count-1'),
      graphJobsCountN: el.getAttribute('data-locale-graph-jobs-count-n'),
      graphDependenciesCount1: el.getAttribute('data-locale-graph-dependencies-count-1'),
      graphDependenciesCountN: el.getAttribute('data-locale-graph-dependencies-count-n'),
      graphSuccessRate: el.getAttribute('data-locale-graph-success-rate'),
      graphZoomIn: el.getAttribute('data-locale-graph-zoom-in'),
      graphZoomMax: el.getAttribute('data-locale-graph-zoom-max'),
      graphZoomOut: el.getAttribute('data-locale-graph-zoom-out'),
      graphResetView: el.getAttribute('data-locale-graph-reset-view'),
    },
  });
  view.mount(el);
}

function initActionRunsList(el: HTMLElement) {
  activePageTimerRefresh({
    interval: () => Number(el.getAttribute('data-action-runs-refresh-interval')),
    async callback() {
      const resp = await GET(el.getAttribute('data-action-runs-refresh-link')!);
      if (!resp.ok) return;
      const newEl = createElementFromHTML(await resp.text());
      for (const attr of newEl.attributes) el.setAttribute(attr.name, attr.value);
      for (const newItem of newEl.querySelectorAll(':scope > .item')) {
        const oldItem = el.querySelector(`#${newItem.id}`);
        if (!oldItem) return;
        if (oldItem.querySelector('.ui.dropdown.active')) continue;
        Idiomorph.morph(oldItem, newItem, {morphStyle: 'outerHTML'});
      }
    },
  });
}
