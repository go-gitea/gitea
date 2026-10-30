import {createApp, reactive} from 'vue';
import ActionRunSummaryView, {type ActionRunSummaryViewLocale} from './ActionRunSummaryView.vue';
import {createEmptyActionsRun, type ActionRunViewStore} from './ActionRunView.ts';

const locale: ActionRunSummaryViewLocale = {
  status: {
    unknown: 'Unknown',
    waiting: 'Waiting',
    running: 'Running',
    cancelling: 'Canceling',
    success: 'Success',
    failure: 'Failure',
    cancelled: 'Canceled',
    skipped: 'Skipped',
    blocked: 'Blocked',
    pending: 'Pending',
  },
  statusLabel: 'Status',
  totalDuration: 'Total duration',
  artifactsTitle: 'Artifacts',
  pagesTitle: 'Pages',
  pagesArtifactSubmitted: 'Artifact submitted',
  pagesViewCurrentSite: 'View current site',
  triggeredVia: 'Triggered via %s',
  rerunTriggered: 'Re-run triggered',
  graphJobsCount1: '%d job',
  graphJobsCountN: '%d jobs',
  graphDependenciesCount1: '%d dependency',
  graphDependenciesCountN: '%d dependencies',
  graphSuccessRate: '%s success',
  graphZoomIn: 'Zoom in',
  graphZoomMax: 'Maximum zoom',
  graphZoomOut: 'Zoom out',
  graphResetView: 'Reset view',
};

function mountSummary(hasPagesArtifact: boolean, pagesURL: string) {
  const currentRun = createEmptyActionsRun();
  currentRun.hasPagesArtifact = hasPagesArtifact;
  currentRun.pagesURL = pagesURL;
  const store: ActionRunViewStore = {
    viewData: reactive({currentRun, runArtifacts: []}),
    startPollingCurrentRun: vi.fn(async () => {}),
    forceReloadCurrentRun: vi.fn(async () => {}),
    stopPollingCurrentRun: vi.fn(),
  };
  const root = document.createElement('div');
  const app = createApp(ActionRunSummaryView, {store, locale, artifactCount: 0});
  app.mount(root);
  return {app, root};
}

test('shows the current Pages address for a run that submitted an artifact', () => {
  const withURL = mountSummary(true, 'https://user.pages.example/repo/');
  const link = withURL.root.querySelector<HTMLAnchorElement>('.action-run-summary-pages a')!;
  expect(link.textContent).toBe('View current site');
  expect(link.href).toBe('https://user.pages.example/repo/');
  withURL.app.unmount();

  const withoutURL = mountSummary(true, '');
  expect(withoutURL.root.querySelector('.action-run-summary-pages')!.textContent).toContain('Artifact submitted');
  withoutURL.app.unmount();

  const ordinaryRun = mountSummary(false, '');
  expect(ordinaryRun.root.querySelector('.action-run-summary-pages')).toBeNull();
  ordinaryRun.app.unmount();
});
