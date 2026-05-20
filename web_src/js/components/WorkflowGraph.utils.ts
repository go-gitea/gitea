import ELK from 'elkjs/lib/elk.bundled.js';
import type {ElkNode, ElkExtendedEdge} from 'elkjs/lib/elk-api.js';
import type {ActionsJob, ActionsStatus} from '../modules/gitea-actions.ts';

export type GraphNodeType = 'job' | 'matrix' | 'group';

export type GraphNode = {
  id: string;
  type: GraphNodeType;
  name: string;
  status: ActionsStatus;
  duration: string;
  x: number;
  y: number;
  level: number;
  displayHeight: number;
  jobs: ActionsJob[];
  matrixKey?: string;
};

export type Edge = {
  fromId: string;
  toId: string;
  key: string;
};

export type RoutedEdge = Edge & {
  path: string;
  fromNode: GraphNode;
  toNode: GraphNode;
};

export type SharedSegment = {
  key: string;
  edgeKeys: string[];
  path: string;
};

export type GraphHighlightState = {
  nodeIds: Set<string>;
  edgeKeys: Set<string>;
};

export type WorkflowGraphLayoutOptions = {
  margin: number;
  nodeWidth: number;
  nodeHeight: number;
  columnGap: number;
  laneGap: number;
  groupRowHeight: number;
  groupPadY: number;
  matrixCollapsedHeight: number;
  matrixLabelHeight: number;
  matrixRowHeight: number;
  matrixPadY: number;
};

export type WorkflowGraphModel = {
  nodes: GraphNode[];
  edges: Edge[];
  routedEdges: RoutedEdge[];
  sharedSegments: SharedSegment[];
  adjacency: NodeAdjacency;
};

export type NodeAdjacency = {
  incomingByNodeId: Map<string, string[]>;
  outgoingByNodeId: Map<string, string[]>;
};

const defaultLayoutOptions: WorkflowGraphLayoutOptions = {
  margin: 24,
  nodeWidth: 220,
  nodeHeight: 40,
  columnGap: 96,
  laneGap: 32,
  groupRowHeight: 28,
  groupPadY: 8,
  matrixCollapsedHeight: 72,
  matrixLabelHeight: 14,
  matrixRowHeight: 28,
  matrixPadY: 8,
};

function canonicalKey(ids: Iterable<string>): string {
  return Array.from(ids).sort().join('');
}

function graphIdForJob(job: ActionsJob): string {
  return `job:${job.id}`;
}

export function matrixKeyFromJobName(name: string): string | null {
  const idx = name.indexOf(' (');
  if (idx === -1) return null;
  return name.slice(0, idx).trim() || null;
}

export function boxBottom(node: GraphNode): number {
  return node.y + node.displayHeight;
}

export function boxCenterY(node: GraphNode): number {
  return node.y + node.displayHeight / 2;
}

const matrixToggleHeight = 16;

function matrixPanelHeight(rowCount: number, expanded: boolean, options: WorkflowGraphLayoutOptions): number {
  if (rowCount <= 0) return options.nodeHeight;
  if (!expanded) return options.matrixCollapsedHeight;
  return options.matrixLabelHeight + rowCount * options.matrixRowHeight + options.matrixPadY * 2 + matrixToggleHeight;
}

function groupPanelHeight(rowCount: number, options: WorkflowGraphLayoutOptions): number {
  return rowCount * options.groupRowHeight + options.groupPadY * 2;
}

function compareStatusWorstFirst(a: ActionsStatus, b: ActionsStatus): number {
  const rank = (s: ActionsStatus) => {
    if (s === 'failure') return 0;
    if (s === 'cancelled') return 1;
    if (s === 'running') return 2;
    if (s === 'waiting') return 3;
    if (s === 'blocked') return 4;
    if (s === 'success') return 5;
    if (s === 'skipped') return 6;
    return 7;
  };
  return rank(a) - rank(b);
}

function aggregateStatus(children: ActionsJob[]): ActionsStatus {
  return children.map((c) => c.status).slice().sort(compareStatusWorstFirst)[0] ?? 'unknown';
}

function buildDirectNeedsMap(jobs: ActionsJob[]): Map<string, string[]> {
  const directNeedsByJobId = new Map<string, string[]>();
  const dependentsByJobId = new Map<string, Set<string>>();

  for (const job of jobs) {
    const needs = job.needs || [];
    directNeedsByJobId.set(job.jobId, needs);
    for (const need of needs) {
      if (!dependentsByJobId.has(need)) dependentsByJobId.set(need, new Set());
      dependentsByJobId.get(need)!.add(job.jobId);
    }
  }

  const reachabilityCache = new Map<string, boolean>();
  function canReach(fromJobId: string, toJobId: string): boolean {
    const cacheKey = `${fromJobId}->${toJobId}`;
    if (reachabilityCache.has(cacheKey)) return reachabilityCache.get(cacheKey)!;
    const visited = new Set<string>();
    const stack = Array.from(dependentsByJobId.get(fromJobId) || []);
    while (stack.length > 0) {
      const current = stack.pop()!;
      if (current === toJobId) {
        reachabilityCache.set(cacheKey, true);
        return true;
      }
      if (visited.has(current)) continue;
      visited.add(current);
      stack.push(...(dependentsByJobId.get(current) || []));
    }
    reachabilityCache.set(cacheKey, false);
    return false;
  }

  const reducedNeedsByJobId = new Map<string, string[]>();
  for (const [jobId, needs] of directNeedsByJobId) {
    reducedNeedsByJobId.set(jobId, needs.filter((need) => {
      return !needs.some((other) => other !== need && canReach(need, other));
    }));
  }
  return reducedNeedsByJobId;
}

export function computeJobLevels(jobs: ActionsJob[]): Map<string, number> {
  const jobMap = new Map<string, ActionsJob>();
  for (const job of jobs) {
    jobMap.set(job.name, job);
    if (job.jobId) jobMap.set(job.jobId, job);
  }

  const levels = new Map<string, number>();
  const visited = new Set<string>();
  const recursionStack = new Set<string>();

  function dfs(jobNameOrId: string): number {
    if (recursionStack.has(jobNameOrId)) return 0;
    if (visited.has(jobNameOrId)) return levels.get(jobNameOrId) ?? 0;
    recursionStack.add(jobNameOrId);
    visited.add(jobNameOrId);

    const job = jobMap.get(jobNameOrId);
    if (!job) {
      recursionStack.delete(jobNameOrId);
      return 0;
    }
    if (!job.needs?.length) {
      levels.set(job.jobId, 0);
      if (job.jobId !== job.name) levels.set(job.name, 0);
      recursionStack.delete(jobNameOrId);
      return 0;
    }

    let maxLevel = -1;
    for (const need of job.needs) {
      if (!jobMap.has(need)) continue;
      maxLevel = Math.max(maxLevel, dfs(need));
    }
    const level = maxLevel + 1;
    levels.set(job.name, level);
    levels.set(job.jobId, level);
    recursionStack.delete(jobNameOrId);
    return level;
  }

  for (const job of jobs) {
    if (!visited.has(job.jobId)) dfs(job.jobId);
  }
  return levels;
}

export function computeGraphHighlightState(hoveredId: string | null, adjacency: NodeAdjacency): GraphHighlightState {
  if (!hoveredId) return {nodeIds: new Set(), edgeKeys: new Set()};
  const {incomingByNodeId, outgoingByNodeId} = adjacency;

  const edgeKeys = new Set<string>();
  const collect = (startId: string, adj: Map<string, string[]>, edgeKeyForward: boolean): Set<string> => {
    const seen = new Set<string>();
    const queue = [startId];
    while (queue.length > 0) {
      const current = queue.shift()!;
      if (seen.has(current)) continue;
      seen.add(current);
      for (const next of adj.get(current) || []) {
        edgeKeys.add(edgeKeyForward ? `${current}->${next}` : `${next}->${current}`);
        if (!seen.has(next)) queue.push(next);
      }
    }
    return seen;
  };

  const ancestors = collect(hoveredId, incomingByNodeId, false);
  const descendants = collect(hoveredId, outgoingByNodeId, true);
  return {nodeIds: new Set([...ancestors, ...descendants]), edgeKeys};
}

type VisualGraphBuild = {
  nodes: GraphNode[];
  edges: Edge[];
};

function buildVisualGraph(
  jobs: ActionsJob[],
  expandedMatrixKeys: ReadonlySet<string>,
  options: WorkflowGraphLayoutOptions,
): VisualGraphBuild {
  const jobsByJobId = new Map<string, ActionsJob[]>();
  const jobIndexById = new Map<number, number>();
  for (const [index, job] of jobs.entries()) {
    jobIndexById.set(job.id, index);
    if (!jobsByJobId.has(job.jobId)) jobsByJobId.set(job.jobId, []);
    jobsByJobId.get(job.jobId)!.push(job);
  }

  const matrixJobsByKey = new Map<string, ActionsJob[]>();
  for (const job of jobs) {
    const matrixKey = matrixKeyFromJobName(job.name);
    if (!matrixKey) continue;
    if (!matrixJobsByKey.has(matrixKey)) matrixJobsByKey.set(matrixKey, []);
    matrixJobsByKey.get(matrixKey)!.push(job);
  }
  for (const list of matrixJobsByKey.values()) {
    list.sort((a, b) => (jobIndexById.get(a.id) ?? 0) - (jobIndexById.get(b.id) ?? 0));
  }

  const directNeedsByJobId = buildDirectNeedsMap(jobs);
  const rawLevels = computeJobLevels(jobs);
  const dependentsByJobId = new Map<string, string[]>();
  const rawEdges: Array<{from: ActionsJob; to: ActionsJob}> = [];

  for (const job of jobs) {
    for (const need of directNeedsByJobId.get(job.jobId) || []) {
      for (const upstream of jobsByJobId.get(need) || []) {
        rawEdges.push({from: upstream, to: job});
        if (!dependentsByJobId.has(upstream.jobId)) dependentsByJobId.set(upstream.jobId, []);
        dependentsByJobId.get(upstream.jobId)!.push(job.jobId);
      }
    }
  }
  for (const list of dependentsByJobId.values()) list.sort();

  // Group sibling jobs that share an identical (parents, children) signature into a single
  // collapsed "group" node. This is a visual aggregation only - the underlying jobs are
  // preserved on the node so the panel can list them.
  const groupedJobIds = new Map<number, string>();
  const groupsById = new Map<string, ActionsJob[]>();
  const groupCandidateBuckets = new Map<string, ActionsJob[]>();
  for (const job of jobs) {
    if (matrixKeyFromJobName(job.name)) continue;
    const needsKey = canonicalKey(directNeedsByJobId.get(job.jobId) || []);
    const childrenKey = (dependentsByJobId.get(job.jobId) || []).join('');
    if (!needsKey && !childrenKey) continue;
    const level = rawLevels.get(job.jobId) ?? 0;
    const key = `group:${level}:${needsKey}:${childrenKey}`;
    if (!groupCandidateBuckets.has(key)) groupCandidateBuckets.set(key, []);
    groupCandidateBuckets.get(key)!.push(job);
  }
  for (const [groupId, groupJobs] of groupCandidateBuckets) {
    if (groupJobs.length < 2) continue;
    groupJobs.sort((a, b) => (jobIndexById.get(a.id) ?? 0) - (jobIndexById.get(b.id) ?? 0));
    groupsById.set(groupId, groupJobs);
    for (const job of groupJobs) groupedJobIds.set(job.id, groupId);
  }

  const visualIdByJobId = new Map<number, string>();
  for (const job of jobs) {
    const matrixKey = matrixKeyFromJobName(job.name);
    if (matrixKey && (matrixJobsByKey.get(matrixKey)?.length ?? 0) > 1) {
      visualIdByJobId.set(job.id, `matrix:${matrixKey}`);
      continue;
    }
    visualIdByJobId.set(job.id, groupedJobIds.get(job.id) || graphIdForJob(job));
  }

  const emittedNodeIds = new Set<string>();
  const nodes: GraphNode[] = [];
  for (const job of jobs) {
    const visualId = visualIdByJobId.get(job.id);
    if (!visualId || emittedNodeIds.has(visualId)) continue;
    emittedNodeIds.add(visualId);

    const matrixKey = matrixKeyFromJobName(job.name);
    if (matrixKey && visualId.startsWith('matrix:')) {
      const matrixJobs = matrixJobsByKey.get(matrixKey) || [];
      nodes.push({
        id: visualId,
        type: 'matrix',
        name: matrixKey,
        status: aggregateStatus(matrixJobs),
        duration: '',
        x: 0, y: 0, level: 0,
        displayHeight: matrixPanelHeight(matrixJobs.length, expandedMatrixKeys.has(matrixKey), options),
        jobs: matrixJobs,
        matrixKey,
      });
      continue;
    }

    const groupJobs = groupsById.get(visualId);
    if (groupJobs) {
      nodes.push({
        id: visualId,
        type: 'group',
        name: groupJobs.map((g) => g.name).join(', '),
        status: aggregateStatus(groupJobs),
        duration: '',
        x: 0, y: 0, level: 0,
        displayHeight: groupPanelHeight(groupJobs.length, options),
        jobs: groupJobs,
      });
      continue;
    }

    nodes.push({
      id: visualId,
      type: 'job',
      name: job.name,
      status: job.status,
      duration: job.duration,
      x: 0, y: 0, level: 0,
      displayHeight: options.nodeHeight,
      jobs: [job],
    });
  }

  const seenEdges = new Set<string>();
  const edges: Edge[] = [];
  for (const {from, to} of rawEdges) {
    const fromId = visualIdByJobId.get(from.id);
    const toId = visualIdByJobId.get(to.id);
    if (!fromId || !toId || fromId === toId) continue;
    const key = `${fromId}->${toId}`;
    if (seenEdges.has(key)) continue;
    seenEdges.add(key);
    edges.push({fromId, toId, key});
  }

  return {nodes, edges};
}

function buildNodeAdjacency(edges: Edge[]): NodeAdjacency {
  const incomingByNodeId = new Map<string, string[]>();
  const outgoingByNodeId = new Map<string, string[]>();
  for (const edge of edges) {
    if (!incomingByNodeId.has(edge.toId)) incomingByNodeId.set(edge.toId, []);
    incomingByNodeId.get(edge.toId)!.push(edge.fromId);
    if (!outgoingByNodeId.has(edge.fromId)) outgoingByNodeId.set(edge.fromId, []);
    outgoingByNodeId.get(edge.fromId)!.push(edge.toId);
  }
  return {incomingByNodeId, outgoingByNodeId};
}

const cornerRadius = 8;

// Sanitize ELK node IDs: ELK forbids '.' and ':' in identifiers.
function toElkId(id: string): string {
  return id.replaceAll(':', '_').replaceAll('.', '_');
}

const elk = new ELK();

async function runElkLayout(
  nodes: GraphNode[],
  edges: Edge[],
  options: WorkflowGraphLayoutOptions,
): Promise<void> {
  const elkNodes: ElkNode[] = nodes.map((n) => ({
    id: toElkId(n.id),
    width: options.nodeWidth,
    height: n.displayHeight,
  }));

  const elkEdges: ElkExtendedEdge[] = edges.map((e, i) => ({
    id: `e${i}`,
    sources: [toElkId(e.fromId)],
    targets: [toElkId(e.toId)],
  }));

  const graph: ElkNode = {
    id: 'root',
    children: elkNodes,
    edges: elkEdges,
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': 'RIGHT',
      'elk.layered.layering.strategy': 'NETWORK_SIMPLEX',
      'elk.layered.nodePlacement.strategy': 'LINEAR_SEGMENTS',
      'elk.edgeRouting': 'ORTHOGONAL',
      'elk.layered.mergeEdges': 'true',
      'elk.layered.spacing.nodeNodeBetweenLayers': String(options.columnGap),
      'elk.spacing.nodeNode': String(options.laneGap),
      'elk.padding': `[top=${options.margin},left=${options.margin},bottom=${options.margin},right=${options.margin}]`,
    },
  };

  const result = await elk.layout(graph);

  // Determine the layer (column index) for each node by sorting on x.
  const elkChildMap = new Map<string, ElkNode>();
  for (const child of result.children || []) elkChildMap.set(child.id, child);

  // Collect unique x positions to compute layer indices.
  const xValues = new Set<number>();
  for (const child of result.children || []) xValues.add(Math.round(child.x!));
  const sortedXs = Array.from(xValues).sort((a, b) => a - b);
  const xToLayer = new Map<number, number>();
  for (const [i, x] of sortedXs.entries()) xToLayer.set(x, i);

  // Snap nodes to uniform column grid while keeping elkjs Y ordering.
  for (const n of nodes) {
    const elkNode = elkChildMap.get(toElkId(n.id));
    if (!elkNode) continue;
    const layer = xToLayer.get(Math.round(elkNode.x!)) ?? 0;
    n.level = layer;
    n.x = options.margin + layer * (options.nodeWidth + options.columnGap);
    n.y = elkNode.y!;
  }
}

// Edges from the same source share a trackX (vertical trunk in the column gap).
// Different sources in the same column get distinct trackX positions so their
// vertical segments don't overlap — this prevents unrelated nodes from looking
// visually connected.
function buildRoutedEdges(
  nodesById: Map<string, GraphNode>,
  edges: Edge[],
  options: WorkflowGraphLayoutOptions,
): Pick<WorkflowGraphModel, 'routedEdges' | 'sharedSegments'> {
  const routedEdges: RoutedEdge[] = [];

  // Collect sources that need a vertical track per column gap.
  const gapSources = new Map<number, Map<string, GraphNode>>();
  for (const edge of edges) {
    const fromNode = nodesById.get(edge.fromId);
    const toNode = nodesById.get(edge.toId);
    if (!fromNode || !toNode) continue;
    if (Math.abs(boxCenterY(fromNode) - boxCenterY(toNode)) < 0.5) continue;
    const gapX = fromNode.x + options.nodeWidth;
    if (!gapSources.has(gapX)) gapSources.set(gapX, new Map());
    gapSources.get(gapX)!.set(edge.fromId, fromNode);
  }

  // Distribute trackX positions evenly across each gap, sorted by source Y.
  const trackXMap = new Map<string, number>();
  for (const [gapX, sources] of gapSources) {
    const sorted = Array.from(sources.values()).sort((a, b) => boxCenterY(a) - boxCenterY(b));
    for (let i = 0; i < sorted.length; i++) {
      trackXMap.set(`${sorted[i].id}:${gapX}`, gapX + options.columnGap * (i + 1) / (sorted.length + 1));
    }
  }

  for (const edge of edges) {
    const fromNode = nodesById.get(edge.fromId);
    const toNode = nodesById.get(edge.toId);
    if (!fromNode || !toNode) continue;

    const startX = fromNode.x + options.nodeWidth;
    const endX = toNode.x;
    const startY = boxCenterY(fromNode);
    const endY = boxCenterY(toNode);

    if (Math.abs(startY - endY) < 0.5) {
      routedEdges.push({...edge, fromNode, toNode, path: `M ${startX} ${startY} H ${endX}`});
      continue;
    }

    const trackX = trackXMap.get(`${edge.fromId}:${startX}`) ?? (startX + options.columnGap / 2);
    const dy = endY > startY ? 1 : -1;
    const r = Math.min(cornerRadius, Math.abs(endY - startY) / 2, Math.abs(trackX - startX), Math.abs(endX - trackX));

    const path = [
      `M ${startX} ${startY}`,
      `H ${trackX - r}`,
      `Q ${trackX} ${startY} ${trackX} ${startY + r * dy}`,
      `V ${endY - r * dy}`,
      `Q ${trackX} ${endY} ${trackX + r} ${endY}`,
      `H ${endX}`,
    ].join(' ');

    routedEdges.push({...edge, fromNode, toNode, path});
  }

  return {routedEdges, sharedSegments: []};
}

export async function createWorkflowGraphModel(
  jobs: ActionsJob[],
  expandedMatrixKeys: ReadonlySet<string> = new Set(),
  partialOptions: Partial<WorkflowGraphLayoutOptions> = {},
): Promise<WorkflowGraphModel> {
  const options = {...defaultLayoutOptions, ...partialOptions};
  const {nodes, edges} = buildVisualGraph(jobs, expandedMatrixKeys, options);
  const nodesById = new Map(nodes.map((n) => [n.id, n]));
  const adjacency = buildNodeAdjacency(edges);
  await runElkLayout(nodes, edges, options);
  return {nodes, edges, ...buildRoutedEdges(nodesById, edges, options), adjacency};
}

export function getWorkflowGraphLayoutOptions(partialOptions: Partial<WorkflowGraphLayoutOptions> = {}): WorkflowGraphLayoutOptions {
  return {...defaultLayoutOptions, ...partialOptions};
}
