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
  matrixHeaderHeight: number;
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
  matrixCollapsedHeight: 78,
  matrixHeaderHeight: 24,
  matrixRowHeight: 26,
  matrixPadY: 6,
};

function canonicalKey(ids: Iterable<string>): string {
  return Array.from(ids).sort().join('');
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

function matrixPanelHeight(rowCount: number, expanded: boolean, options: WorkflowGraphLayoutOptions): number {
  if (rowCount <= 0) return options.nodeHeight;
  if (!expanded) return options.matrixCollapsedHeight;
  return options.matrixHeaderHeight + rowCount * options.matrixRowHeight + options.matrixPadY * 2;
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
      return needs.every((other) => other === need || !canReach(need, other));
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
    // Reusable callers are distinct workflow files — never fold them into a matrix bucket
    // even if their display name happens to look like "name (variant)".
    if (job.isReusableCaller) continue;
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
    // Reusable callers represent distinct workflow files — keep each as its own node so the
    // graph mirrors GitHub Actions, where every caller shows up as its own box even when
    // siblings share an identical (parents, children) dependency signature.
    if (job.isReusableCaller) continue;
    const needsKey = canonicalKey(directNeedsByJobId.get(job.jobId) || []);
    const childrenKey = (dependentsByJobId.get(job.jobId) || []).join('');
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
    // Symmetric with the matrix-bucket loop above: a reusable caller whose display name
    // happens to look like "name (variant)" must never be folded into the matrix node, or it
    // would silently vanish (its visualId would point at a matrix node it isn't part of).
    if (matrixKey && !job.isReusableCaller && (matrixJobsByKey.get(matrixKey)?.length ?? 0) > 1) {
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

function assignNodeLevels(nodes: GraphNode[], {incomingByNodeId}: NodeAdjacency): void {
  const cache = new Map<string, number>();
  function levelFor(id: string, visiting = new Set<string>()): number {
    if (cache.has(id)) return cache.get(id)!;
    if (visiting.has(id)) return 0;
    visiting.add(id);
    const incoming = incomingByNodeId.get(id) || [];
    const level = incoming.length > 0 ?
      Math.max(...incoming.map((fromId) => levelFor(fromId, visiting))) + 1 :
      0;
    visiting.delete(id);
    cache.set(id, level);
    return level;
  }
  for (const node of nodes) node.level = levelFor(node.id);
}

// Roots stay in input order; later levels are sorted by the mean parent Y so that simple
// chains stay on a straight horizontal line.
function assignNodeCoordinates(nodesById: Map<string, GraphNode>, nodes: GraphNode[], adjacency: NodeAdjacency, options: WorkflowGraphLayoutOptions): void {
  const {incomingByNodeId} = adjacency;
  const inputRank = (node: GraphNode): number => Math.min(...node.jobs.map((j) => j.id));

  const nodesByLevel = new Map<number, GraphNode[]>();
  for (const node of nodes) {
    if (!nodesByLevel.has(node.level)) nodesByLevel.set(node.level, []);
    nodesByLevel.get(node.level)!.push(node);
  }
  const orderedLevels = Array.from(nodesByLevel.keys()).sort((a, b) => a - b);

  // Initial X assignment and a default Y so barycenters can use a finite value.
  for (const level of orderedLevels) {
    const list = nodesByLevel.get(level)!;
    list.sort((a, b) => inputRank(a) - inputRank(b));
    let yCursor = options.margin;
    for (const node of list) {
      node.x = options.margin + level * (options.nodeWidth + options.columnGap);
      node.y = yCursor;
      yCursor += node.displayHeight + options.laneGap;
    }
  }

  function packLevel(level: number, anchorOf: (n: GraphNode) => number): void {
    const list = nodesByLevel.get(level)!;
    const sorted = Array.from(list).sort((a, b) => anchorOf(a) - anchorOf(b) || inputRank(a) - inputRank(b));
    // Pack tight to top after sorting. Using barycenter only for order (not Y) keeps terminal
    // nodes like build-image close to the top of their column instead of being pulled down to
    // the mean Y of their parents — matching GitHub Actions' compact layout.
    let prevBottom = options.margin - options.laneGap;
    for (const node of sorted) {
      node.y = prevBottom + options.laneGap;
      prevBottom = boxBottom(node);
    }
    nodesByLevel.set(level, sorted);
  }

  function meanCenterOf(ids: string[]): number | null {
    if (ids.length === 0) return null;
    let sum = 0;
    for (const id of ids) sum += boxCenterY(nodesById.get(id)!);
    return sum / ids.length;
  }

  // Down-only barycenter pass: each child is anchored to the mean Y of its parents. Roots
  // keep their initial yaml-declaration order (via inputRank), matching how GitHub Actions
  // arranges root jobs. This produces a "main chain on top" layout where job-100 → job-101 →
  // job-102 stays on a straight horizontal line.
  for (const level of orderedLevels) {
    if (level === 0) continue;
    packLevel(level, (node) => meanCenterOf(incomingByNodeId.get(node.id) || []) ?? boxCenterY(node));
  }
}

// Per-edge connector: source stub → cubic-bezier corner down/up to column midpoint →
// vertical run → cubic-bezier corner back to horizontal → target stub. The corner radius is
// fixed (not clamped to the row delta) so any two edges sharing the same source produce the
// same source-side path and overlap into a single visual line until they diverge at the V.
const cornerRadius = 12;

function connectorPath(sx: number, sy: number, ex: number, ey: number, options: WorkflowGraphLayoutOptions): string {
  if (Math.abs(sy - ey) < 0.5) return `M ${sx} ${sy} H ${ex}`;
  // Anchor the V segment in the column gap immediately before the target instead of the
  // horizontal midpoint. The long H stays at the source's Y, matching GitHub Actions' style
  // — a multi-column edge runs along the source row across intermediate columns, then turns
  // up/down only when it reaches the target column.
  const midX = Math.max(ex - options.columnGap / 2, (sx + ex) / 2);
  const dy = ey > sy ? 1 : -1;
  // Keep the same H prefix to `midX - cornerRadius` for every edge so that edges sharing a
  // source overlap visually until they fork. When there isn't 2*cornerRadius of vertical
  // room for the V segment, emit a single S-curve between (midX - r, sy) and (midX + r, ey)
  // instead of a backward V kink.
  if (Math.abs(ey - sy) < cornerRadius * 2) {
    return [
      `M ${sx} ${sy}`,
      `H ${midX - cornerRadius}`,
      `C ${midX} ${sy} ${midX} ${ey} ${midX + cornerRadius} ${ey}`,
      `H ${ex}`,
    ].join(' ');
  }
  const half = cornerRadius / 2;
  return [
    `M ${sx} ${sy}`,
    `H ${midX - cornerRadius}`,
    `C ${midX - half} ${sy} ${midX} ${sy + half * dy} ${midX} ${sy + cornerRadius * dy}`,
    `V ${ey - cornerRadius * dy}`,
    `C ${midX} ${ey - half * dy} ${midX + half} ${ey} ${midX + cornerRadius} ${ey}`,
    `H ${ex}`,
  ].join(' ');
}

function buildRoutedEdges(
  nodesById: Map<string, GraphNode>,
  edges: Edge[],
  options: WorkflowGraphLayoutOptions,
): Pick<WorkflowGraphModel, 'routedEdges' | 'sharedSegments'> {
  const routedEdges: RoutedEdge[] = [];
  for (const edge of edges) {
    const fromNode = nodesById.get(edge.fromId);
    const toNode = nodesById.get(edge.toId);
    if (!fromNode || !toNode) continue;
    const startX = fromNode.x + options.nodeWidth;
    const endX = toNode.x;
    const startY = boxCenterY(fromNode);
    const endY = boxCenterY(toNode);
    routedEdges.push({...edge, fromNode, toNode, path: connectorPath(startX, startY, endX, endY, options)});
  }

  return {routedEdges, sharedSegments: []};
}

export function createWorkflowGraphModel(
  jobs: ActionsJob[],
  expandedMatrixKeys: ReadonlySet<string> = new Set(),
  partialOptions: Partial<WorkflowGraphLayoutOptions> = {},
): WorkflowGraphModel {
  const options = {...defaultLayoutOptions, ...partialOptions};
  const {nodes, edges} = buildVisualGraph(jobs, expandedMatrixKeys, options);
  const nodesById = new Map(nodes.map((n) => [n.id, n]));
  const adjacency = buildNodeAdjacency(edges);
  assignNodeLevels(nodes, adjacency);
  assignNodeCoordinates(nodesById, nodes, adjacency, options);
  return {nodes, edges, ...buildRoutedEdges(nodesById, edges, options), adjacency};
}

export function getWorkflowGraphLayoutOptions(partialOptions: Partial<WorkflowGraphLayoutOptions> = {}): WorkflowGraphLayoutOptions {
  return {...defaultLayoutOptions, ...partialOptions};
}
