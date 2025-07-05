import {test, expect, beforeEach, afterEach, vi} from 'vitest';
import {mount} from '@vue/test-utils';
import RepoActionView from './RepoActionView.vue';

/**
 * Focused tests for RepoActionView auto-expand functionality.
 *
 * This test suite specifically targets the "Always expand running logs" setting.
 */

// Helper function to create default props
function createDefaultProps() {
  return {
    runIndex: '1',
    jobIndex: '0',
    actionsURL: '/test/actions',
    locale: {
      status: {},
      approve: 'Approve',
      cancel: 'Cancel',
      rerun_all: 'Rerun all',
      scheduled: 'Scheduled',
      commit: 'Commit',
      pushedBy: 'pushed by',
      logsAlwaysAutoScroll: 'Always auto scroll logs',
      logsAlwaysExpandRunning: 'Always expand running logs',
      showLogSeconds: 'Show seconds',
      showTimeStamps: 'Show timestamps',
      showFullScreen: 'Show full screen',
      downloadLogs: 'Download logs',
      artifactsTitle: 'Artifacts',
      artifactExpired: 'Expired',
      confirmDeleteArtifact: 'Confirm delete artifact: %s',
      rerun: 'Rerun',
    },
  };
}

// Helper function to create mock API response
function createMockApiResponse(steps: any[] = [], runStatus = 'running') {
  return {
    artifacts: [] as any[],
    state: {
      run: {
        link: '',
        title: '',
        titleHTML: '',
        status: runStatus,
        canCancel: false,
        canApprove: false,
        canRerun: false,
        canDeleteArtifact: false,
        done: false,
        workflowID: '',
        workflowLink: '',
        isSchedule: false,
        jobs: [] as any[],
        commit: {
          localeCommit: '',
          localePushedBy: '',
          shortSHA: '',
          link: '',
          pusher: {displayName: '', link: ''},
          branch: {name: '', link: '', isDeleted: false},
        },
      },
      currentJob: {
        title: 'Test Job',
        detail: 'Test job detail',
        steps,
      },
    },
    logs: {stepsLog: [] as any[]},
  };
}

// Helper function to setup auto-expand localStorage
function enableAutoExpand() {
  localStorage.setItem('actions-view-options', JSON.stringify({
    autoScroll: true,
    expandRunning: true,
  }));
}

beforeEach(() => {
  // Setup window.config for CSRF token which is needed by fetch.ts
  (globalThis as any).window = globalThis.window || {};
  globalThis.window.config = globalThis.window.config || {};
  globalThis.window.config.csrfToken = 'test-csrf-token';

  // Default mock - needed because unmocked `loadJobData` is called when component is mounted
  globalThis.fetch = vi.fn(() =>
    Promise.resolve({
      ok: true,
      json: () => Promise.resolve(createMockApiResponse()),
    } as Response),
  );
});

afterEach(() => {
  localStorage.removeItem('actions-view-options');
});

test('auto expand works on subsequent loads', async () => {
  enableAutoExpand();

  const wrapper = mount(RepoActionView, {props: createDefaultProps()});
  await wrapper.vm.$nextTick();

  // Stop the interval timer
  if (wrapper.vm.intervalID) {
    clearInterval(wrapper.vm.intervalID);
    wrapper.vm.intervalID = null;
  }

  // Create mock fetch function
  const mockResponse = createMockApiResponse([
    {summary: 'Step 1', duration: '1s', status: 'running'},
  ]);
  const mockFetchJobData = vi.fn().mockResolvedValue(mockResponse);

  // Reset component state to ensure isFirstLoad = true
  wrapper.vm.run.status = '' as any;
  wrapper.vm.currentJob.steps = [];
  wrapper.vm.currentJobStepsStates = [];
  wrapper.vm.loadingAbortController = null;

  // Call the real loadJob method with our mock fetch function
  await wrapper.vm.loadJob(mockFetchJobData);

  // First load should work - step should be auto-expanded
  expect(wrapper.vm.run.status).toBe('running');
  expect(wrapper.vm.currentJobStepsStates.length).toBe(1);
  expect(wrapper.vm.currentJobStepsStates[0].expanded).toBe(true);

  // manually collapse the step
  wrapper.vm.currentJobStepsStates[0].expanded = false;

  // Clear abort controller and call loadJob again (isFirstLoad will now be false)
  wrapper.vm.loadingAbortController = null;
  await wrapper.vm.loadJob(mockFetchJobData);

  // The step should auto-expand on subsequent loads
  expect(wrapper.vm.currentJobStepsStates[0].expanded).toBe(true);

  wrapper.unmount();
});

test('auto expand works when step becomes running', async () => {
  enableAutoExpand();

  const wrapper = mount(RepoActionView, {props: createDefaultProps()});
  await wrapper.vm.$nextTick();

  // Stop the interval timer
  if (wrapper.vm.intervalID) {
    clearInterval(wrapper.vm.intervalID);
    wrapper.vm.intervalID = null;
  }

  // Create mock fetch function that returns different responses on each call
  let callCount = 0;
  const mockFetchJobData = vi.fn().mockImplementation(async () => {
    callCount++;
    const stepStatus = callCount === 1 ? 'waiting' : 'running';

    return createMockApiResponse([
      {summary: 'Step 1', duration: '1s', status: stepStatus},
      {summary: 'Step 2', duration: '0s', status: 'waiting'},
    ]);
  });

  // Reset component state
  wrapper.vm.run.status = 'unknown' as any;
  wrapper.vm.currentJob.steps = [];
  wrapper.vm.currentJobStepsStates = [];
  wrapper.vm.loadingAbortController = null;

  // First load - step is waiting (using real component loadJob logic)
  await wrapper.vm.loadJob(mockFetchJobData);
  expect(wrapper.vm.currentJobStepsStates.length).toBe(2);
  expect(wrapper.vm.currentJobStepsStates[0].expanded).toBe(false); // Not expanded because step is waiting

  // Clear abort controller and do second load - step becomes running, should auto-expand
  wrapper.vm.loadingAbortController = null;
  await wrapper.vm.loadJob(mockFetchJobData);

  // The step transitioned to running and should auto-expand even when isFirstLoad = false
  expect(wrapper.vm.currentJobStepsStates[0].expanded).toBe(true);

  wrapper.unmount();
});
