import {createApp} from 'vue';
import RepoActionView from '../components/RepoActionView.vue';

export function initRepositoryActionView() {
  const el = document.querySelector('#repo-action-view');
  if (!el) return;

  // TODO: the parent element's full height doesn't work well now,
  // but we can not pollute the global style at the moment, only fix the height problem for pages with this component
  const parentFullHeight = document.querySelector<HTMLElement>('body > div.full.height');
  if (parentFullHeight) parentFullHeight.classList.add('tw-pb-0');

  const analysisRunLink = el.getAttribute('data-analysis-run-link');
  const view = createApp(RepoActionView, {
    jobId: parseInt(el.getAttribute('data-job-id')!),
    actionsViewUrl: el.getAttribute('data-actions-view-url'),
    analysis: analysisRunLink ? {
      runLink: analysisRunLink,
      failureTagsUrl: el.getAttribute('data-analysis-failure-tags-url')!,
      locale: {
        title: el.getAttribute('data-locale-analysis-title')!,
        tabRun: el.getAttribute('data-locale-analysis-tab-run')!,
        add: el.getAttribute('data-locale-analysis-add')!,
        edit: el.getAttribute('data-locale-analysis-edit')!,
        delete: el.getAttribute('data-locale-analysis-delete')!,
        save: el.getAttribute('data-locale-analysis-save')!,
        cancel: el.getAttribute('data-locale-analysis-cancel')!,
        notePlaceholder: el.getAttribute('data-locale-analysis-placeholder')!,
        tagsLabel: el.getAttribute('data-locale-analysis-tags')!,
        empty: el.getAttribute('data-locale-analysis-empty')!,
        confirmDelete: el.getAttribute('data-locale-analysis-confirm-delete')!,
      },
    } : undefined,
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
      triggeredVia: el.getAttribute('data-locale-triggered-via'),
      totalDuration: el.getAttribute('data-locale-total-duration'),
      artifactsTitle: el.getAttribute('data-locale-artifacts-title'),
      artifactExpired: el.getAttribute('data-locale-artifact-expired'),
      artifactExpiresAt: el.getAttribute('data-locale-artifact-expires-at'),
      confirmDeleteArtifact: el.getAttribute('data-locale-confirm-delete-artifact'),
      showTimeStamps: el.getAttribute('data-locale-show-timestamps'),
      showLogSeconds: el.getAttribute('data-locale-show-log-seconds'),
      showFullScreen: el.getAttribute('data-locale-show-full-screen'),
      downloadLogs: el.getAttribute('data-locale-download-logs'),
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
      runDetails: el.getAttribute('data-locale-run-details'),
    },
  });
  view.mount(el);
}
