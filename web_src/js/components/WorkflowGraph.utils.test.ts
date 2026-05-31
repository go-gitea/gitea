import {computeGraphHighlightState, computeJobLevels, createWorkflowGraphModel, matrixKeyFromJobName} from './WorkflowGraph.utils.ts';
import type {ActionsJob} from '../modules/gitea-actions.ts';

const mockJobs: ActionsJob[] = [
  {id: 1, link: '', jobId: 'job-100', name: 'job-100', status: 'success', canRerun: false, duration: '3s'},
  {id: 2, link: '', jobId: 'job-101', name: 'job-101', status: 'success', canRerun: false, duration: '3s', needs: ['job-100']},
  {id: 3, link: '', jobId: 'job-102', name: 'job-102', status: 'success', canRerun: false, duration: '4s', needs: ['job-101']},
  {id: 4, link: '', jobId: 'job-103', name: 'job-103', status: 'success', canRerun: false, duration: '2s', needs: ['job-100']},
  {id: 5, link: '', jobId: 'prep-jdk', name: 'prep-jdk', status: 'success', canRerun: false, duration: '3s'},
  {id: 6, link: '', jobId: 'code-analysis', name: 'code-analysis', status: 'success', canRerun: false, duration: '3s'},
  {id: 7, link: '', jobId: 'matrix-e2e-1-chromium', name: 'matrix-e2e (1, chromium)', status: 'success', canRerun: false, duration: '2s', needs: ['job-100', 'prep-jdk', 'code-analysis']},
  {id: 8, link: '', jobId: 'matrix-e2e-1-firefox', name: 'matrix-e2e (1, firefox)', status: 'success', canRerun: false, duration: '2s', needs: ['job-100', 'prep-jdk', 'code-analysis']},
  {id: 9, link: '', jobId: 'matrix-e2e-2-chromium', name: 'matrix-e2e (2, chromium)', status: 'success', canRerun: false, duration: '2s', needs: ['job-100', 'prep-jdk', 'code-analysis']},
  {id: 10, link: '', jobId: 'matrix-e2e-3-chromium', name: 'matrix-e2e (3, chromium)', status: 'success', canRerun: false, duration: '4s', needs: ['job-100', 'prep-jdk', 'code-analysis']},
  {id: 11, link: '', jobId: 'matrix-e2e-3-firefox', name: 'matrix-e2e (3, firefox)', status: 'success', canRerun: false, duration: '2s', needs: ['job-100', 'prep-jdk', 'code-analysis']},
  {id: 12, link: '', jobId: 'matrix-e2e-99-webkit', name: 'matrix-e2e (99, webkit)', status: 'success', canRerun: false, duration: '2s', needs: ['job-100', 'prep-jdk', 'code-analysis']},
  {id: 13, link: '', jobId: 'unit-test', name: 'unit-test', status: 'success', canRerun: false, duration: '3s', needs: ['prep-jdk', 'code-analysis']},
  {id: 14, link: '', jobId: 'arch-test', name: 'arch-test', status: 'success', canRerun: false, duration: '3s', needs: ['prep-jdk', 'code-analysis']},
  {id: 15, link: '', jobId: 'integration-test', name: 'integration-test', status: 'success', canRerun: false, duration: '4s', needs: ['prep-jdk', 'code-analysis']},
  {id: 16, link: '', jobId: 'build-image', name: 'build-image', status: 'success', canRerun: false, duration: '3s', needs: [
    'unit-test',
    'arch-test',
    'integration-test',
    'matrix-e2e-1-chromium',
    'matrix-e2e-1-firefox',
    'matrix-e2e-2-chromium',
    'matrix-e2e-3-chromium',
    'matrix-e2e-3-firefox',
    'matrix-e2e-99-webkit',
  ]},
];

const verifyDeployJobs: ActionsJob[] = [
  {id: 101, link: '', jobId: 'seed-dev', name: 'seed-dev', status: 'success', canRerun: false, duration: '2s'},
  {id: 102, link: '', jobId: 'seed-qa', name: 'seed-qa', status: 'success', canRerun: false, duration: '3s'},
  {id: 103, link: '', jobId: 'verify-dev', name: 'Verify Dev', status: 'success', canRerun: false, duration: '3s', needs: ['seed-dev']},
  {id: 104, link: '', jobId: 'verify-qa', name: 'Verify QA', status: 'success', canRerun: false, duration: '4s', needs: ['seed-qa']},
  {id: 105, link: '', jobId: 'deploy', name: 'Deploy', status: 'blocked', canRerun: false, duration: '', needs: ['verify-dev', 'verify-qa']},
];

// Multi-level pipeline with two matrices and a leaf with two parents.
const wfTest1Jobs: ActionsJob[] = [
  {id: 1, link: '', jobId: 'init', name: 'Initialize Pipeline', status: 'success', canRerun: false, duration: '1s'},
  {id: 2, link: '', jobId: 'lint-frontend', name: 'Lint Frontend', status: 'success', canRerun: false, duration: '3s', needs: ['init']},
  {id: 3, link: '', jobId: 'lint-backend', name: 'Lint Backend', status: 'success', canRerun: false, duration: '3s', needs: ['init']},
  {id: 4, link: '', jobId: 'build-frontend', name: 'Build Frontend', status: 'success', canRerun: false, duration: '4s', needs: ['lint-frontend']},
  {id: 5, link: '', jobId: 'build-backend', name: 'Build Backend', status: 'success', canRerun: false, duration: '5s', needs: ['lint-backend']},
  {id: 6, link: '', jobId: 'tu-api-t', name: 'Unit Tests (api, true)', status: 'success', canRerun: false, duration: '3s', needs: ['build-frontend', 'build-backend']},
  {id: 7, link: '', jobId: 'tu-api-f', name: 'Unit Tests (api, false)', status: 'success', canRerun: false, duration: '3s', needs: ['build-frontend', 'build-backend']},
  {id: 8, link: '', jobId: 'tu-svc-t', name: 'Unit Tests (service, true)', status: 'success', canRerun: false, duration: '3s', needs: ['build-frontend', 'build-backend']},
  {id: 9, link: '', jobId: 'test-integration', name: 'Integration Tests', status: 'success', canRerun: false, duration: '6s', needs: ['build-backend']},
  {id: 10, link: '', jobId: 'te-c-d', name: 'E2E Tests (chrome, desktop)', status: 'success', canRerun: false, duration: '4s', needs: ['build-frontend', 'tu-api-t', 'tu-api-f', 'tu-svc-t']},
  {id: 11, link: '', jobId: 'te-c-m', name: 'E2E Tests (chrome, mobile)', status: 'success', canRerun: false, duration: '4s', needs: ['build-frontend', 'tu-api-t', 'tu-api-f', 'tu-svc-t']},
  {id: 12, link: '', jobId: 'te-f-d', name: 'E2E Tests (firefox, desktop)', status: 'success', canRerun: false, duration: '4s', needs: ['build-frontend', 'tu-api-t', 'tu-api-f', 'tu-svc-t']},
  {id: 13, link: '', jobId: 'bundle-app', name: 'Bundle Application', status: 'success', canRerun: false, duration: '3s', needs: ['tu-api-t', 'tu-api-f', 'tu-svc-t', 'test-integration', 'te-c-d', 'te-c-m', 'te-f-d']},
  {id: 14, link: '', jobId: 'deploy-dev', name: 'Deploy to Dev', status: 'success', canRerun: false, duration: '3s', needs: ['bundle-app']},
  {id: 15, link: '', jobId: 'deploy-qa', name: 'Deploy to QA', status: 'success', canRerun: false, duration: '3s', needs: ['bundle-app']},
  {id: 16, link: '', jobId: 'verify-dev', name: 'Verify Dev', status: 'success', canRerun: false, duration: '2s', needs: ['deploy-dev']},
  {id: 17, link: '', jobId: 'verify-qa', name: 'Verify QA', status: 'success', canRerun: false, duration: '2s', needs: ['deploy-qa']},
  {id: 18, link: '', jobId: 'deploy-prod', name: 'Deploy to Production', status: 'success', canRerun: false, duration: '5s', needs: ['verify-dev', 'verify-qa']},
  {id: 19, link: '', jobId: 'post-deploy-checks', name: 'Post-Deploy Checks', status: 'success', canRerun: false, duration: '2s', needs: ['deploy-prod']},
];

test('matrix key heuristic strips trailing parameter list', () => {
  expect(matrixKeyFromJobName('matrix-e2e (1, chromium)')).toBe('matrix-e2e');
  expect(matrixKeyFromJobName('plain-job')).toBeNull();
});

test('computeJobLevels keeps stable topological levels', () => {
  const levels = computeJobLevels(mockJobs);
  expect(levels.get('job-100')).toBe(0);
  expect(levels.get('job-101')).toBe(1);
  expect(levels.get('job-102')).toBe(2);
  expect(levels.get('build-image')).toBe(2);
});

test('graph model collapses matrix and groups jobs that share parents and children', () => {
  const graph = createWorkflowGraphModel(mockJobs);

  expect(graph.nodes.find((n) => n.type === 'matrix')?.jobs).toHaveLength(6);
  const groupJobIds = graph.nodes.filter((n) => n.type === 'group').map((g) => g.jobs.map((j) => j.jobId));
  expect(groupJobIds).toEqual(expect.arrayContaining([
    ['prep-jdk', 'code-analysis'],
    ['unit-test', 'arch-test', 'integration-test'],
  ]));
});

test('expanded matrix height includes summary and toggle rows', () => {
  const collapsed = createWorkflowGraphModel(mockJobs);
  const expanded = createWorkflowGraphModel(mockJobs, new Set(['matrix-e2e']));
  const collapsedMatrix = collapsed.nodes.find((n) => n.id === 'matrix:matrix-e2e');
  const expandedMatrix = expanded.nodes.find((n) => n.id === 'matrix:matrix-e2e');

  expect(collapsedMatrix?.displayHeight).toBeLessThan(expandedMatrix?.displayHeight ?? 0);
  // 6 jobs * 26 row height + 40 header + 6 pad * 2 = 208
  expect(expandedMatrix?.displayHeight).toBe(208);
});

test('every dependency is rendered as one routed edge', () => {
  const graph = createWorkflowGraphModel(mockJobs);
  const rootGroup = graph.nodes.find((n) => n.type === 'group' && n.jobs.some((j) => j.jobId === 'prep-jdk'))!;
  const testGroup = graph.nodes.find((n) => n.type === 'group' && n.jobs.some((j) => j.jobId === 'unit-test'))!;
  const expectedKeys = [
    `${rootGroup.id}->matrix:matrix-e2e`,
    `${rootGroup.id}->${testGroup.id}`,
  ];
  const keys = new Set(graph.routedEdges.map((e) => e.key));
  for (const k of expectedKeys) expect(keys.has(k)).toBe(true);
});

test('same-row edge collapses to a single horizontal line', () => {
  const graph = createWorkflowGraphModel(verifyDeployJobs);
  const verifyDevEdge = graph.routedEdges.find((e) => e.fromId === 'job:101' && e.toId === 'job:103');
  const verifyQaEdge = graph.routedEdges.find((e) => e.fromId === 'job:102' && e.toId === 'job:104');
  expect(verifyDevEdge?.path).toMatch(/^M [\d.]+ [\d.]+ H [\d.]+$/);
  expect(verifyQaEdge?.path).toMatch(/^M [\d.]+ [\d.]+ H [\d.]+$/);
});

test('different-row edge uses cubic bezier curve', () => {
  const graph = createWorkflowGraphModel(verifyDeployJobs);
  const deployLowerEdge = graph.routedEdges.find((e) => e.fromId === 'job:104' && e.toId === 'job:105');
  expect(deployLowerEdge?.path).toContain(' C ');
});

test('multi-level pipeline with two matrices and a converging leaf renders without errors', () => {
  const graph = createWorkflowGraphModel(wfTest1Jobs);
  const matrices = graph.nodes.filter((n) => n.type === 'matrix');
  expect(matrices.map((n) => n.matrixKey).sort()).toEqual(['E2E Tests', 'Unit Tests']);

  const deployProd = graph.nodes.find((n) => n.id === 'job:18');
  const verifyDev = graph.nodes.find((n) => n.id === 'job:16');
  const verifyQa = graph.nodes.find((n) => n.id === 'job:17');
  expect(verifyDev?.level).toBe(verifyQa?.level);
  expect(deployProd?.level).toBe((verifyDev?.level ?? 0) + 1);

  for (const node of graph.nodes) {
    expect(Number.isFinite(node.x)).toBe(true);
    expect(Number.isFinite(node.y)).toBe(true);
    expect(node.x).toBeGreaterThanOrEqual(0);
    expect(node.y).toBeGreaterThanOrEqual(0);
  }
  for (const edge of graph.routedEdges) {
    expect(edge.path).not.toMatch(/NaN|undefined|Infinity/);
  }
});

test('directed highlight state covers ancestors and descendants of the hovered node', () => {
  const graph = createWorkflowGraphModel(mockJobs);
  const rootGroup = graph.nodes.find((n) => n.type === 'group' && n.jobs.some((j) => j.jobId === 'prep-jdk'))!;

  const highlight = computeGraphHighlightState(rootGroup.id, graph.adjacency);
  expect(highlight.nodeIds.has('matrix:matrix-e2e')).toBe(true);
  expect(highlight.nodeIds.has('job:16')).toBe(true);
  expect(highlight.edgeKeys.has(`${rootGroup.id}->matrix:matrix-e2e`)).toBe(true);
});

test('directed highlight state for converging graph excludes sibling branch when hovering parent', () => {
  const graph = createWorkflowGraphModel(verifyDeployJobs);

  const parentHighlight = computeGraphHighlightState('job:103', graph.adjacency);
  expect(parentHighlight.nodeIds.has('job:101')).toBe(true);
  expect(parentHighlight.nodeIds.has('job:105')).toBe(true);
  expect(parentHighlight.nodeIds.has('job:104')).toBe(false);
  expect(parentHighlight.edgeKeys.has('job:103->job:105')).toBe(true);
  expect(parentHighlight.edgeKeys.has('job:104->job:105')).toBe(false);

  const sinkHighlight = computeGraphHighlightState('job:105', graph.adjacency);
  expect(sinkHighlight.nodeIds.has('job:103')).toBe(true);
  expect(sinkHighlight.nodeIds.has('job:104')).toBe(true);
  expect(sinkHighlight.edgeKeys.has('job:103->job:105')).toBe(true);
  expect(sinkHighlight.edgeKeys.has('job:104->job:105')).toBe(true);
});
