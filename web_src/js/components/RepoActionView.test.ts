/**
 * Unit test for RepoActionView's auto-scroll *logic* (the shouldAutoScroll method).
 * This test focuses on the predicate that determines if a scroll *should* occur,
 * rather than verifying the full end-to-end auto-scroll behavior (DOM interaction or actual scrolling).
 *
 * This test should FAIL with the original buggy code and PASS with our fix,
 * specifically for the 'slightly below viewport' scenario.
 */

import {createApp} from 'vue';
import RepoActionView from './RepoActionView.vue';

// Mock dependencies to isolate the shouldAutoScroll logic
vi.mock('../svg.ts', () => ({
  SvgIcon: {template: '<span></span>'},
}));

vi.mock('./ActionRunStatus.vue', () => ({
  default: {template: '<span></span>'},
}));

vi.mock('../utils/dom.ts', () => ({
  createElementFromAttrs: vi.fn(),
  toggleElem: vi.fn(),
}));

vi.mock('../utils/time.ts', () => ({
  formatDatetime: vi.fn(() => '2023-01-01'),
}));

vi.mock('../render/ansi.ts', () => ({
  renderAnsi: vi.fn((text) => text),
}));

vi.mock('../modules/fetch.ts', () => ({
  POST: vi.fn(() => Promise.resolve()),
  DELETE: vi.fn(() => Promise.resolve()),
}));

vi.mock('../utils.ts', () => ({
  toggleFullScreen: vi.fn(),
}));

describe('RepoActionView auto-scroll logic (shouldAutoScroll method)', () => {
  beforeEach(() => {
    // Mock window properties for controlled environment
    Object.defineProperty(window, 'innerHeight', {
      writable: true,
      configurable: true,
      value: 600,
    });

    Object.defineProperty(window, 'localStorage', {
      value: {
        getItem: vi.fn(() => null),
        setItem: vi.fn(),
      },
      writable: true,
    });

    // Mock clearInterval and setInterval to prevent actual timer execution
    globalThis.clearInterval = vi.fn();
    globalThis.setInterval = vi.fn(() => 1 as any);
  });

  test('should auto-scroll when log element is slightly below viewport (following logs)', () => {
    // This test verifies the core behavioral change in the `shouldAutoScroll` method:
    // Original code: STRICT check (element must be entirely in viewport)
    // Fixed code: LENIENT check (element can be slightly below if user is following logs)

    // Mock the last child element's getBoundingClientRect to simulate its position.
    // NOTE: This test *mocks* the DOM interaction (getLastLogLineElement and getBoundingClientRect)
    // and does not verify the component's ability to correctly find the element or
    // that the real DOM element would produce these exact coordinates.
    const mockLastChildElement = {
      getBoundingClientRect: () => ({
        top: 590,      // Starts at bottom of 600px viewport
        bottom: 610,   // Extends 10px below viewport
        left: 0,
        right: 800,
        width: 800,
        height: 20,
      }),
    };

    // Create container and mount component for context and state setup
    const container = document.createElement('div');
    document.body.append(container);

    const app = createApp(RepoActionView, {
      runIndex: '1',
      jobIndex: '0',
      actionsURL: '/test',
      locale: {
        status: {
          unknown: 'Unknown', waiting: 'Waiting', running: 'Running',
          success: 'Success', failure: 'Failure', cancelled: 'Cancelled',
          skipped: 'Skipped', blocked: 'Blocked',
        },
        approvals_text: 'Approvals', commit: 'Commit', pushedBy: 'Pushed by',
      },
    });

    const vm = app.mount(container) as any;

    // Set up component state to enable auto-scroll conditions
    vm.optionAlwaysAutoScroll = true;
    vm.$data.currentJobStepsStates = [{expanded: true, cursor: null}];

    // Mock a running step (required for auto-scroll)
    vm.$data.currentJob = {
      steps: [{status: 'running'}],
    };

    // Mock internal methods that interact with the DOM to control inputs to shouldAutoScroll.
    // This allows us to precisely test the `shouldAutoScroll` method's logic.
    const mockContainer = {
      getBoundingClientRect: () => ({
        top: 100,      // Container is visible in viewport
        bottom: 500,   // Container extends into viewport
        left: 0,
        right: 800,
        width: 800,
        height: 400,   // Large container, clearly visible
      }),
    };
    vm.getJobStepLogsContainer = vi.fn(() => mockContainer); // Return mock container
    vm.getLastLogLineElement = vi.fn(() => mockLastChildElement); // Return the test element

    // Test the actual component's shouldAutoScroll method
    const shouldScroll = vm.shouldAutoScroll(0);

    // CRITICAL BEHAVIORAL TEST (for the predicate logic):
    // When element is slightly below viewport (simulating user following logs), should auto-scroll?
    // Original buggy code: FALSE (too strict - requires element entirely in viewport)
    // Fixed code: TRUE (lenient - allows slight overflow for better UX)
    expect(shouldScroll).toBe(true);

    // Cleanup
    app.unmount();
    container.remove();
  });

  test('should NOT auto-scroll when element is far below viewport (user scrolled up)', () => {
    // Both original and fixed code should agree on this case.
    // This scenario simulates a user having scrolled up significantly.

    // Mock the last child element's getBoundingClientRect to simulate its position.
    // As with other tests, this directly feeds values to `shouldAutoScroll` without
    // verifying actual DOM rendering or element finding.
    const mockLastChildElement = {
      getBoundingClientRect: () => ({
        top: 800,      // Way below 600px viewport
        bottom: 820,
        left: 0,
        right: 800,
        width: 800,
        height: 20,
      }),
    };

    const container = document.createElement('div');
    document.body.append(container);

    const app = createApp(RepoActionView, {
      runIndex: '1',
      jobIndex: '0',
      actionsURL: '/test',
      locale: {
        status: {
          unknown: 'Unknown', waiting: 'Waiting', running: 'Running',
          success: 'Success', failure: 'Failure', cancelled: 'Cancelled',
          skipped: 'Skipped', blocked: 'Blocked',
        },
        approvals_text: 'Approvals', commit: 'Commit', pushedBy: 'Pushed by',
      },
    });

    const vm = app.mount(container) as any;
    vm.optionAlwaysAutoScroll = true;
    vm.$data.currentJobStepsStates = [{expanded: true, cursor: null}];

    // Mock a running step (so the failure is due to scroll position, not step status)
    vm.$data.currentJob = {
      steps: [{status: 'running'}],
    };

    // Mock a container that's far above viewport (user scrolled past it)
    const mockContainer = {
      getBoundingClientRect: () => ({
        top: -300,     // Container is above viewport
        bottom: -100,  // Container ends above viewport
        left: 0,
        right: 800,
        width: 800,
        height: 200,
      }),
    };
    vm.getJobStepLogsContainer = vi.fn(() => mockContainer);
    vm.getLastLogLineElement = vi.fn(() => mockLastChildElement);

    const shouldScroll = vm.shouldAutoScroll(0);

    // The `shouldAutoScroll` logic should return false here.
    expect(shouldScroll).toBe(false);

    app.unmount();
    container.remove();
  });

  test('should NOT auto-scroll when step is not expanded', () => {
    // This test verifies that auto-scroll is prevented when the job step is not expanded,
    // regardless of the log element's position.
    const container = document.createElement('div');
    document.body.append(container);

    const app = createApp(RepoActionView, {
      runIndex: '1',
      jobIndex: '0',
      actionsURL: '/test',
      locale: {
        status: {
          unknown: 'Unknown', waiting: 'Waiting', running: 'Running',
          success: 'Success', failure: 'Failure', cancelled: 'Cancelled',
          skipped: 'Skipped', blocked: 'Blocked',
        },
        approvals_text: 'Approvals', commit: 'Commit', pushedBy: 'Pushed by',
      },
    });

    const vm = app.mount(container) as any;
    vm.optionAlwaysAutoScroll = true;
    vm.$data.currentJobStepsStates = [{expanded: false, cursor: null}]; // Not expanded

    const shouldScroll = vm.shouldAutoScroll(0);

    // The `shouldAutoScroll` logic should return false.
    expect(shouldScroll).toBe(false);

    app.unmount();
    container.remove();
  });

  test('should NOT auto-scroll when step is finished (not running)', () => {
    // Auto-scroll should only happen for currently executing steps, not finished ones

    // Mock log element that would normally trigger auto-scroll
    const mockLastLogElement = {
      getBoundingClientRect: () => ({
        top: 590,      // Near bottom of viewport (would normally auto-scroll)
        bottom: 610,
        left: 0,
        right: 800,
        width: 800,
        height: 20,
      }),
    };

    const container = document.createElement('div');
    document.body.append(container);

    const app = createApp(RepoActionView, {
      runIndex: '1',
      jobIndex: '0',
      actionsURL: '/test',
      locale: {
        status: {
          unknown: 'Unknown', waiting: 'Waiting', running: 'Running',
          success: 'Success', failure: 'Failure', cancelled: 'Cancelled',
          skipped: 'Skipped', blocked: 'Blocked',
        },
        approvals_text: 'Approvals', commit: 'Commit', pushedBy: 'Pushed by',
      },
    });

    const vm = app.mount(container) as any;
    vm.optionAlwaysAutoScroll = true;
    vm.$data.currentJobStepsStates = [{expanded: true, cursor: null}];

    // Mock a finished step (success status)
    vm.$data.currentJob = {
      steps: [{status: 'success'}],
    };

    vm.getJobStepLogsContainer = vi.fn(() => ({}));
    vm.getLastLogLineElement = vi.fn(() => mockLastLogElement);

    const shouldScroll = vm.shouldAutoScroll(0);

    // Should NOT auto-scroll for finished steps, even if logs are following-friendly
    expect(shouldScroll).toBe(false);

    app.unmount();
    container.remove();
  });

  test('should auto-scroll when step is running and user following logs', () => {
    // This ensures we still auto-scroll when the step is actively running and user is following

    // Mock log element that suggests user is following
    const mockLastLogElement = {
      getBoundingClientRect: () => ({
        top: 590,
        bottom: 610,   // Slightly below viewport (normal following behavior)
        left: 0,
        right: 800,
        width: 800,
        height: 20,
      }),
    };

    const container = document.createElement('div');
    document.body.append(container);

    const app = createApp(RepoActionView, {
      runIndex: '1',
      jobIndex: '0',
      actionsURL: '/test',
      locale: {
        status: {
          unknown: 'Unknown', waiting: 'Waiting', running: 'Running',
          success: 'Success', failure: 'Failure', cancelled: 'Cancelled',
          skipped: 'Skipped', blocked: 'Blocked',
        },
        approvals_text: 'Approvals', commit: 'Commit', pushedBy: 'Pushed by',
      },
    });

    const vm = app.mount(container) as any;
    vm.optionAlwaysAutoScroll = true;
    vm.$data.currentJobStepsStates = [{expanded: true, cursor: null}];

    // Mock a running step
    vm.$data.currentJob = {
      steps: [{status: 'running'}],
    };

    vm.getJobStepLogsContainer = vi.fn(() => ({}));
    vm.getLastLogLineElement = vi.fn(() => mockLastLogElement);

    const shouldScroll = vm.shouldAutoScroll(0);

    // SHOULD auto-scroll when step is running and user is following logs
    expect(shouldScroll).toBe(true);

    app.unmount();
    container.remove();
  });
});
