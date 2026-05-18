<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue';
import {SvgIcon} from '../svg.ts';
import ActionStatusIcon from './ActionStatusIcon.vue';
import {localUserSettings} from '../modules/user-settings.ts';
import {isPlainClick} from '../utils/dom.ts';
import {debounce} from 'throttle-debounce';
import type {ActionsJob, ActionsStatus} from '../modules/gitea-actions.ts';
import type {ActionRunViewStore} from './ActionRunView.ts';

interface JobNode {
  id: number;
  name: string;
  status: ActionsStatus;
  duration: string;

  x: number;
  y: number;
  level: number;
}

interface Edge {
  fromId: number;
  toId: number;
  key: string;
}

interface RoutedEdge extends Edge {
  path: string;
  fromNode: JobNode;
  toNode: JobNode;
}

interface StoredState {
  scale: number;
  translateX: number;
  translateY: number;
  timestamp: number;
}

const props = defineProps<{
  store: ActionRunViewStore;
  jobs: ActionsJob[];
  runLink: string;
  workflowId: string;
}>()

const settingKeyStates = 'actions-graph-states';
const maxStoredStates = 10;

const scale = ref(1);
const translateX = ref(0);
const translateY = ref(0);
const isDragging = ref(false);
const lastMousePos = ref({x: 0, y: 0});
const graphContainer = ref<HTMLElement | null>(null);
const hoveredJobId = ref<number | null>(null);

const stateKey = () => `${props.store.viewData.currentRun.repoId}-${props.workflowId}`;

const loadSavedState = () => {
  const allStates = localUserSettings.getJsonObject<Record<string, StoredState>>(settingKeyStates, {});
  const saved = allStates[stateKey()];
  if (!saved) return;
  scale.value = clampScale(saved.scale ?? scale.value);
  translateX.value = saved.translateX ?? translateX.value;
  translateY.value = saved.translateY ?? translateY.value;
};

const saveState = () => {
  const allStates = localUserSettings.getJsonObject<Record<string, StoredState>>(settingKeyStates, {});
  allStates[stateKey()] = {
    scale: scale.value,
    translateX: translateX.value,
    translateY: translateY.value,
    timestamp: Date.now(),
  };

  const sortedStates = Object.entries(allStates)
    .sort(([, a], [, b]) => b.timestamp - a.timestamp)
    .slice(0, maxStoredStates);

  localUserSettings.setJsonObject(settingKeyStates, Object.fromEntries(sortedStates));
};

const minNodeWidth = 168;
const maxNodeWidth = 232;
const nodeWidth = computed(() => {
  const maxNameLength = Math.max(...props.jobs.map(j => j.name.length), 0);
  return Math.min(Math.max(minNodeWidth, maxNameLength * 8), maxNodeWidth);
});

const horizontalSpacing = computed(() => nodeWidth.value + 84);
const graphWidth = computed(() => {
  if (jobsWithLayout.value.length === 0) return 800;
  const maxX = Math.max(...jobsWithLayout.value.map(j => j.x + nodeWidth.value));
  return maxX + margin * 2;
});

const graphHeight = computed(() => {
  if (jobsWithLayout.value.length === 0) return 400;
  const maxY = Math.max(...jobsWithLayout.value.map(j => j.y + nodeHeight));
  return maxY + margin * 2;
});


const jobsWithLayout = computed<JobNode[]>(() => {
  try {
    const levels = computeJobLevels(props.jobs);
    const currentHorizontalSpacing = horizontalSpacing.value;

    const jobsByLevel: ActionsJob[][] = [];
    let maxJobsPerLevel = 0;

    props.jobs.forEach(job => {
      const level = levels.get(job.name) || levels.get(job.jobId) || 0;

      if (!jobsByLevel[level]) {
        jobsByLevel[level] = [];
      }
      jobsByLevel[level].push(job);

      if (jobsByLevel[level].length > maxJobsPerLevel) {
        maxJobsPerLevel = jobsByLevel[level].length;
      }
    });

    const result: JobNode[] = [];
    jobsByLevel.forEach((levelJobs, levelIndex) => {
      if (!levelJobs || levelJobs.length === 0) {
        return;
      }

      const startY = margin;

      levelJobs.forEach((job, jobIndex) => {
        result.push({
          id: job.id,
          name: job.name,
          status: job.status,
          duration: job.duration,

          x: margin + levelIndex * currentHorizontalSpacing,
          y: startY + jobIndex * verticalSpacing,
          level: levelIndex,
        });
      });
    });

    return result;
  } catch (error) {
    return props.jobs.map((job, index) => ({
      id: job.id,
      name: job.name,
      status: job.status,
      duration: job.duration,

      x: margin + index * horizontalSpacing.value,
      y: margin,
      level: 0,
    }));
  }
});

function buildDirectNeedsMap(jobs: ActionsJob[]): Map<string, string[]> {
  const directNeedsByJobId = new Map<string, string[]>();
  const dependentsByJobId = new Map<string, Set<string>>();

  for (const job of jobs) {
    const needs = job.needs || [];
    directNeedsByJobId.set(job.jobId, needs);

    for (const need of needs) {
      if (!dependentsByJobId.has(need)) {
        dependentsByJobId.set(need, new Set());
      }
      dependentsByJobId.get(need)!.add(job.jobId);
    }
  }

  const reachabilityCache = new Map<string, boolean>();

  function canReach(fromJobId: string, toJobId: string): boolean {
    const cacheKey = `${fromJobId}->${toJobId}`;
    if (reachabilityCache.has(cacheKey)) {
      return reachabilityCache.get(cacheKey)!;
    }

    const visited = new Set<string>();
    const stack = [...(dependentsByJobId.get(fromJobId) || [])];

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
  for (const [jobId, needs] of directNeedsByJobId.entries()) {
    reducedNeedsByJobId.set(jobId, needs.filter((need) => {
      return !needs.some((otherNeed) => otherNeed !== need && canReach(need, otherNeed));
    }));
  }

  return reducedNeedsByJobId;
}

const directNeedsByJobId = computed(() => buildDirectNeedsMap(props.jobs));

const edges = computed<Edge[]>(() => {
  const edgesList: Edge[] = [];
  const jobsByJobId = new Map<string, ActionsJob[]>();

  for (const job of props.jobs) {
    if (!jobsByJobId.has(job.jobId)) {
      jobsByJobId.set(job.jobId, []);
    }
    jobsByJobId.get(job.jobId)!.push(job);
  }

  for (const job of props.jobs) {
    for (const need of directNeedsByJobId.value.get(job.jobId) || []) {
      const upstreamJobs = jobsByJobId.get(need) || [];
      for (const upstreamJob of upstreamJobs) {
        edgesList.push({
          fromId: upstreamJob.id,
          toId: job.id,
          key: `${upstreamJob.id}-${job.id}`,
        });
      }
    }
  }

  return edgesList;
});

function buildRoundedConnectorPath(startX: number, startY: number, endX: number, endY: number, turnX: number): string {
  const deltaY = endY - startY;
  if (Math.abs(deltaY) < 1) {
    return `M ${startX} ${startY} H ${endX}`;
  }

  const direction = deltaY > 0 ? 1 : -1;
  const elbowSize = Math.max(8, Math.min(24, Math.abs(deltaY) / 2, Math.abs(endX - startX) / 2));
  const controlOffset = elbowSize / 2;
  const clampedTurnX = Math.min(Math.max(turnX, startX + elbowSize), endX - elbowSize);

  return [
    `M ${startX} ${startY}`,
    `H ${clampedTurnX - elbowSize}`,
    `C ${clampedTurnX - controlOffset} ${startY} ${clampedTurnX} ${startY + direction * controlOffset} ${clampedTurnX} ${startY + direction * elbowSize}`,
    `V ${endY - direction * elbowSize}`,
    `C ${clampedTurnX} ${endY - direction * controlOffset} ${clampedTurnX + controlOffset} ${endY} ${clampedTurnX + elbowSize} ${endY}`,
    `H ${endX}`,
  ].join(' ');
}

const routedEdges = computed<RoutedEdge[]>(() => {
  const nodesById = new Map(jobsWithLayout.value.map((job) => [job.id, job]));
  const outgoingEdges = new Map<number, Edge[]>();
  const incomingEdges = new Map<number, Edge[]>();

  for (const edge of edges.value) {
    if (!outgoingEdges.has(edge.fromId)) {
      outgoingEdges.set(edge.fromId, []);
    }
    outgoingEdges.get(edge.fromId)!.push(edge);

    if (!incomingEdges.has(edge.toId)) {
      incomingEdges.set(edge.toId, []);
    }
    incomingEdges.get(edge.toId)!.push(edge);
  }

  for (const sourceEdges of outgoingEdges.values()) {
    sourceEdges.sort((a, b) => {
      const targetA = nodesById.get(a.toId);
      const targetB = nodesById.get(b.toId);
      if (!targetA || !targetB) return 0;
      return targetA.y - targetB.y || a.toId - b.toId;
    });
  }

  const edgePaths: RoutedEdge[] = [];

  for (const edge of edges.value) {
    const fromNode = nodesById.get(edge.fromId);
    const toNode = nodesById.get(edge.toId);
    if (!fromNode || !toNode) continue;

    const startX = fromNode.x + nodeWidth.value;
    const startY = fromNode.y + nodeHeight / 2;
    const endX = toNode.x;
    const endY = toNode.y + nodeHeight / 2;
    const sourceEdges = outgoingEdges.get(edge.fromId) || [];
    const targetEdges = incomingEdges.get(edge.toId) || [];
    const horizontalGap = endX - startX;
    const turnOffset = Math.min(28, Math.max(16, horizontalGap * 0.14));
    const sourceTurnX = startX + turnOffset;
    const targetTurnX = endX - turnOffset;

    let turnX = startX + horizontalGap / 2;
    if (sourceEdges.length > 1) {
      turnX = sourceTurnX;
    } else if (targetEdges.length > 1) {
      turnX = targetTurnX;
    }

    const path = buildRoundedConnectorPath(startX, startY, endX, endY, turnX);

    edgePaths.push({
      ...edge,
      path,
      fromNode,
      toNode,
    });
  }

  return edgePaths;
});

const graphMetrics = computed(() => {
  const successCount = jobsWithLayout.value.filter(job => job.status === 'success').length;

  const levels = new Map<number, number>();
  jobsWithLayout.value.forEach(job => {
    const count = levels.get(job.level) || 0;
    levels.set(job.level, count + 1);
  })
  const parallelism = Math.max(...Array.from(levels.values()), 0);

  return {
    successRate: `${((successCount / jobsWithLayout.value.length) * 100).toFixed(0)}%`,
    parallelism,
  };
})

const nodeHeight = 52;
const verticalSpacing = 90;
const margin = 40;

const minScale = 0.3;
const maxScale = 1;

function clampScale(nextScale: number): number {
  return Math.min(Math.max(Math.round(nextScale * 100) / 100, minScale), maxScale);
}

const canZoomIn = computed(() => scale.value < maxScale);

function zoomTo(nextScale: number) {
  scale.value = clampScale(nextScale);
}

function zoomIn() {
  zoomTo(scale.value * 1.2);
}

function zoomOut() {
  zoomTo(scale.value / 1.2);
}

function resetView() {
  scale.value = 1;
  translateX.value = 0;
  translateY.value = 0;
}

function handleMouseDown(e: MouseEvent) {
  if (!isPlainClick(e)) return;

  // don't start drag on interactive/text elements inside the SVG
  const target = e.target as Element;
  const interactive = target.closest('div, p, a, span, button, input, text, .job-node-group');
  if (interactive?.closest('svg')) return;

  e.preventDefault();

  isDragging.value = true;
  lastMousePos.value = {x: e.clientX, y: e.clientY};
  graphContainer.value!.style.cursor = 'grabbing';
}

function handleMouseMoveOnDocument(event: MouseEvent) {
  if (!isDragging.value) return;

  const dx = event.clientX - lastMousePos.value.x;
  const dy = event.clientY - lastMousePos.value.y;

  translateX.value += dx;
  translateY.value += dy;

  lastMousePos.value = {x: event.clientX, y: event.clientY};
}

function handleMouseUpOnDocument() {
  if (!isDragging.value) return;
  isDragging.value = false;
  graphContainer.value!.style.cursor = 'grab';
}

function handleWheel(event: WheelEvent) {
  // Without a modifier, let the wheel scroll the page
  if (!event.ctrlKey && !event.metaKey) {
    return;
  }
  event.preventDefault();
  const zoomFactor = Math.exp(-event.deltaY * 0.0015);
  zoomTo(scale.value * zoomFactor);
}

onMounted(() => {
  loadSavedState();
  watch([translateX, translateY, scale], debounce(500, saveState));
  watch([scale], debounce(100, saveState));

  document.addEventListener('mousemove', handleMouseMoveOnDocument);
  document.addEventListener('mouseup', handleMouseUpOnDocument);
});

onUnmounted(() => {
  document.removeEventListener('mousemove', handleMouseMoveOnDocument);
  document.removeEventListener('mouseup', handleMouseUpOnDocument);
});

function handleNodeMouseEnter(job: JobNode) {
  hoveredJobId.value = job.id;
}

function handleNodeMouseLeave() {
  hoveredJobId.value = null;
}

function isEdgeHighlighted(edge: RoutedEdge): boolean {
  if (!hoveredJobId.value) {
    return false;
  }
  return edge.fromId === hoveredJobId.value || edge.toId === hoveredJobId.value;
}

const nodesWithIncomingEdge = computed(() => {
  const set = new Set<number>();
  for (const edge of routedEdges.value) set.add(edge.toId);
  return set;
});

const nodesWithOutgoingEdge = computed(() => {
  const set = new Set<number>();
  for (const edge of routedEdges.value) set.add(edge.fromId);
  return set;
});


function computeJobLevels(jobs: ActionsJob[]): Map<string, number> {
  const jobMap = new Map<string, ActionsJob>()
  jobs.forEach(job => {
    jobMap.set(job.name, job);
    if (job.jobId) jobMap.set(job.jobId, job);
  });

  const levels = new Map<string, number>();
  const visited = new Set<string>();
  const recursionStack = new Set<string>();
  const MAX_DEPTH = 100;

  function dfs(jobNameOrId: string, depth: number = 0): number {
    if (depth > MAX_DEPTH) {
      console.error(`Max recursion depth (${MAX_DEPTH}) reached for: ${jobNameOrId}`);
      return 0;
    }

    if (recursionStack.has(jobNameOrId)) {
      console.error(`Cycle detected involving: ${jobNameOrId}`);
      return 0;
    }

    if (visited.has(jobNameOrId)) {
      return levels.get(jobNameOrId) || 0;
    }

    recursionStack.add(jobNameOrId);
    visited.add(jobNameOrId);

    const job = jobMap.get(jobNameOrId);
    if (!job) {
      recursionStack.delete(jobNameOrId);
      return 0;
    }

    if (!job.needs?.length) {
      levels.set(job.jobId, 0);
      recursionStack.delete(jobNameOrId);
      return 0;
    }

    let maxLevel = -1;
    for (const need of job.needs) {
      const needJob = jobMap.get(need);
      if (!needJob) continue;

      const needLevel = dfs(need, depth + 1);
      maxLevel = Math.max(maxLevel, needLevel);
    }

    const level = maxLevel + 1
    levels.set(job.name, level);
    if (job.jobId && job.jobId !== job.name) {
      levels.set(job.jobId, level);
    }

    recursionStack.delete(jobNameOrId);
    return level;
  }

  jobs.forEach(job => {
    if (!visited.has(job.name) && !visited.has(job.jobId)) {
      dfs(job.name);
    }
  })

  return levels;
}

function onNodeClick(job: JobNode, event: MouseEvent) {
  const link = `${props.runLink}/jobs/${job.id}`;
  if (event.ctrlKey || event.metaKey) {
    window.open(link, '_blank');
    return;
  }
  window.location.href = link;
}
</script>

<template>
  <div class="workflow-graph" v-if="jobs.length > 0">
    <div class="graph-header">
      <h4 class="graph-title">Workflow Dependencies</h4>
      <div class="graph-stats">
        {{ jobs.length }} jobs • {{ edges.length }} dependencies
        <span v-if="graphMetrics">
          • <span class="graph-metrics">{{ graphMetrics.successRate }} success</span>
        </span>
      </div>
      <div class="flex-text-block">
        <button
          type="button"
          @click="zoomIn"
          class="ui compact tiny icon button"
          :disabled="!canZoomIn"
          :title="canZoomIn ? 'Zoom in (Ctrl/Cmd + scroll on graph)' : 'Already at 100% zoom'"
        >
          <SvgIcon name="octicon-zoom-in" :size="12"/>
        </button>
        <button type="button" @click="resetView" class="ui compact tiny icon button" title="Reset view">
          <SvgIcon name="octicon-sync" :size="12"/>
        </button>
        <button type="button" @click="zoomOut" class="ui compact tiny icon button" title="Zoom out (Ctrl/Cmd + scroll on graph)">
          <SvgIcon name="octicon-zoom-out" :size="12"/>
        </button>
      </div>
    </div>

    <div
      class="graph-container"
      ref="graphContainer"
      @mousedown="handleMouseDown"
      @wheel="handleWheel"
      :class="{dragging: isDragging}"
    >
      <svg
        :width="graphWidth"
        :height="graphHeight"
        class="graph-svg"
        :style="{
          transform: `translate(${translateX}px, ${translateY}px) scale(${scale})`,
          transformOrigin: '0 0',
        }"
      >
        <path
          v-for="edge in routedEdges"
          :key="edge.key"
          :d="edge.path"
          fill="none"
          stroke="var(--color-secondary-alpha-50)"
          stroke-width="1.5"
          :class="['node-edge', { 'highlighted-edge': isEdgeHighlighted(edge) }]"
        />

        <g
          v-for="job in jobsWithLayout"
          :key="job.id"
          class="job-node-group"
          @click="onNodeClick(job, $event)"
          @mouseenter="handleNodeMouseEnter(job)"
          @mouseleave="handleNodeMouseLeave"
        >
          <title>{{ job.name }}</title>

          <rect
            :x="job.x"
            :y="job.y"
            :width="nodeWidth"
            :height="nodeHeight"
            rx="8"
            fill="var(--color-box-body)"
            stroke="var(--color-secondary)"
            stroke-width="1"
            class="job-rect"
          />

          <circle
            v-if="nodesWithIncomingEdge.has(job.id)"
            :cx="job.x"
            :cy="job.y + nodeHeight / 2"
            r="4.5"
            class="node-port"
          />

          <circle
            v-if="nodesWithOutgoingEdge.has(job.id)"
            :cx="job.x + nodeWidth"
            :cy="job.y + nodeHeight / 2"
            r="4.5"
            class="node-port"
          />

          <foreignObject
            :x="job.x + 10"
            :y="job.y + 16"
            width="20"
            height="20"
            class="job-status-fg-obj"
          >
            <div class="job-status-icon-wrap">
              <ActionStatusIcon :status="job.status" icon-variant="circle-fill"/>
            </div>
          </foreignObject>

          <foreignObject
            :x="job.x + 38"
            :y="job.y + 2"
            :width="nodeWidth - 44"
            :height="nodeHeight - 4"
          >
            <div class="job-text-wrap">
              <span class="job-name">{{ job.name }}</span>
              <span
                v-if="job.duration || job.status === 'success' || job.status === 'failure'"
                class="job-duration"
              >{{ job.duration }}</span>
            </div>
          </foreignObject>

        </g>
      </svg>
    </div>
  </div>
</template>

<style scoped>
.workflow-graph {
  flex: 1;
  display: flex;
  flex-direction: column;
}
.graph-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 14px;
  background: var(--color-box-header);
  border-bottom: 1px solid var(--color-secondary);
  gap: var(--gap-block);
  flex-wrap: wrap;
}

.graph-title {
  margin: 0;
  color: var(--color-text);
  font-size: 16px;
  font-weight: var(--font-weight-semibold);
  flex: 1;
  min-width: 200px;
}

.graph-stats {
  display: flex;
  align-items: baseline;
  column-gap: 8px;
  color: var(--color-text-light-1);
  font-size: 13px;
  white-space: nowrap;
}

.graph-metrics {
  color: var(--color-primary);
  font-weight: var(--font-weight-medium);
}

.graph-container {
  flex: 1;
  overflow: hidden;
  padding: 10px 14px 18px;
  border-radius: 0 0 var(--border-radius) var(--border-radius);
  cursor: grab;
  position: relative;
  background: var(--color-box-body);
}

.graph-container.dragging {
  cursor: grabbing;
}

.graph-svg {
  display: block;
  will-change: transform;
}

.graph-svg path {
  transition: all 0.2s ease;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.highlighted-edge {
  stroke-width: 2 !important;
  stroke: var(--color-workflow-edge-hover) !important;
}

.job-node-group {
  cursor: pointer;
  transition: all 0.2s ease;
}

.job-node-group:hover .job-rect {
  /* due to SVG rendering limitation, only one of fill and drop-shadow can work */
  fill: var(--color-hover);
  /* filter: drop-shadow(0 1px 3px var(--color-shadow-opaque)); */
}

.job-text-wrap {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 1px;
  padding: 4px 8px 4px 0;
  overflow: hidden;
}

.job-name {
  width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  font-weight: var(--font-weight-semibold);
  color: var(--color-text);
  user-select: none;
  pointer-events: none;
}

.job-duration {
  font-size: 10px;
  line-height: 1.2;
  color: var(--color-text-light-2);
  white-space: nowrap;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  user-select: none;
  pointer-events: none;
}

.job-status-fg-obj,
.job-status-icon-wrap {
  pointer-events: none;
}

.job-status-icon-wrap {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.node-port {
  fill: var(--color-box-body);
  stroke: var(--color-light-border);
  stroke-width: 1.25;
  opacity: 0.85;
  pointer-events: none;
}

.node-edge {
  transition: stroke-width 0.2s ease, opacity 0.2s ease;
  opacity: 0.75;
}
</style>
