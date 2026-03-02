<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue';
import {SvgIcon} from '../svg.ts';
import {localUserSettings} from "../modules/user-settings.ts";
import {debounce} from "throttle-debounce";
import type {ActionsJob, ActionsRunStatus} from '../modules/gitea-actions.ts';

interface JobNode {
  id: number;
  name: string;
  status: ActionsRunStatus;
  needs: string[];
  duration: string;

  index: number;

  x: number;
  y: number;
  level: number;
}

interface Edge {
  from: string;
  to: string;
  key: string;
}

interface BezierEdge extends Edge {
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
  jobs: ActionsJob[];
  currentJobIndex: number;
  runLink: string;
  workflowId: string;
}>()

const settingKeyStates = 'actions-graph-states';
const maxStoredStates = 10;

const scale = ref(1);
const translateX = ref(0);
const translateY = ref(0);
const isDragging = ref(false);
const dragStart = ref({ x: 0, y: 0 });
const lastMousePos = ref({ x: 0, y: 0 });
const graphContainer = ref<HTMLElement | null>(null);
const hoveredJobId = ref<number | null>(null);

const loadSavedState = () => {
  const allStates = localUserSettings.getJsonObject<Record<string, StoredState>>(settingKeyStates, {});
  const saved = allStates[props.workflowId];
  if (!saved) return;
  scale.value = saved.scale ?? scale.value;
  translateX.value = saved.translateX ?? translateX.value;
  translateY.value = saved.translateY ?? translateY.value;
}

const saveState = () => {
  // TODO: different repos might have the same workflowId, but at the moment, we don't have repo id
  // If overwritten occurs, acceptable, not too bad
  const allStates = localUserSettings.getJsonObject<Record<string, StoredState>>(settingKeyStates, {});
  allStates[props.workflowId] = {
    scale: scale.value,
    translateX: translateX.value,
    translateY: translateY.value,
    timestamp: Date.now(),
  };

  const sortedStates = Object.entries(allStates)
    .sort(([, a], [, b]) => b.timestamp - a.timestamp)
    .slice(0, maxStoredStates);

  const limitedStates = Object.fromEntries(sortedStates);
  localUserSettings.setJsonObject(settingKeyStates, limitedStates);
};

loadSavedState();
watch([translateX, translateY, scale], () => {
  debounce(500, saveState);
})

const nodeWidth = computed(() => {
  const maxNameLength = Math.max(...props.jobs.map(j => j.name.length));
  return Math.min(Math.max(140, maxNameLength * 8), 180);
});

const horizontalSpacing = computed(() => nodeWidth.value + 20);
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

      const levelWidth = (levelJobs.length - 1) * currentHorizontalSpacing;
      const startX = margin + (maxJobsPerLevel * currentHorizontalSpacing - levelWidth) / 2;

      levelJobs.forEach((job, jobIndex) => {
        result.push({
          id: job.id,
          name: job.name,
          status: job.status,
          needs: job.needs || [],
          duration: job.duration,

          index: props.jobs.findIndex(j => j.id === job.id),

          x: startX + jobIndex * currentHorizontalSpacing,
          y: margin + levelIndex * verticalSpacing,
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
      needs: job.needs || [],
      duration: job.duration,

      index: index,

      x: margin + index * (nodeWidth.value + 40),
      y: margin,
      level: 0,
    }));
  }
});

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
    for (const need of job.needs || []) {
      const targetJobs = jobsByJobId.get(need) || [];
      for (const targetJob of targetJobs) {
        edgesList.push({
          from: targetJob.name,
          to: job.name,
          key: `${targetJob.id}-${job.id}`,
        });
      }
    }
  }

  return edgesList;
});

const bezierEdges = computed<BezierEdge[]>(() => {
  const bezierEdgesList: BezierEdge[] = [];

  edges.value.forEach(edge => {
    const fromNode = jobsWithLayout.value.find(j => j.name === edge.from);
    const toNode = jobsWithLayout.value.find(j => j.name === edge.to);

    if (!fromNode || !toNode) {
      return;
    }

    const startX = fromNode.x + nodeWidth.value / 2;
    const startY = fromNode.y + nodeHeight;
    const endX = toNode.x + nodeWidth.value / 2;
    const endY = toNode.y;

    const levelDiff = toNode.level - fromNode.level;
    const curveStrength = 30 + Math.abs(levelDiff) * 15;

    const controlX1 = startX;
    const controlY1 = startY + curveStrength;
    const controlX2 = endX;
    const controlY2 = endY - curveStrength;

    const path = `M ${startX} ${startY} C ${controlX1} ${controlY1}, ${controlX2} ${controlY2}, ${endX} ${endY}`;

    bezierEdgesList.push({
      ...edge,
      path,
      fromNode,
      toNode,
    });
  });

  return bezierEdgesList;
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

const nodeHeight = 50;
const verticalSpacing = 120;
const margin = 40;

function zoomIn() {
  scale.value = Math.min(scale.value * 1.2, 3);
}

function zoomOut() {
  scale.value = Math.max(scale.value / 1.2, 0.5);
}

function resetView() {
  scale.value = 1;
  translateX.value = 0;
  translateY.value = 0;
}

function handleMouseDown(e: MouseEvent) {
  if (e.button !== 0 || e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return; // only left mouse button can drag
  const target = e.target as Element;
  // don't start the drag if the click is on an interactive element (e.g.: link, button) or text element
  const interactive = target.closest('div, p, a, span, button, input, text, .job-node-group');
  if (interactive?.closest('svg')) return;

  e.preventDefault();

  isDragging.value = true;
  dragStart.value = {
    x: e.clientX - translateX.value,
    y: e.clientY - translateY.value,
  };
  lastMousePos.value = { x: e.clientX, y: e.clientY };
  graphContainer.value!.style.cursor = 'grabbing';
}

function handleMouseMoveOnDocument(event: MouseEvent) {
  if (!isDragging.value) return;

  const dx = event.clientX - lastMousePos.value.x;
  const dy = event.clientY - lastMousePos.value.y;

  translateX.value += dx;
  translateY.value += dy;

  lastMousePos.value = { x: event.clientX, y: event.clientY };
}

function handleMouseUpOnDocument() {
  if (!isDragging.value) return;
  isDragging.value = false;
  graphContainer.value!.style.cursor = 'grab';
}

onMounted(() => {
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

function isEdgeHighlighted(edge: BezierEdge): boolean {
  if (!hoveredJobId.value) {
    return false;
  }

  const hoveredJob = jobsWithLayout.value.find(j => j.id === hoveredJobId.value);
  if (!hoveredJob) {
    return false;
  }

  return edge.from === hoveredJob.name || edge.to === hoveredJob.name;
}

function getNodeColor(status: ActionsRunStatus): string {
  if (status === 'success') {
    return 'var(--color-green-dark-2)';
  } else if (status === 'failure') {
    return 'var(--color-red-dark-2)';
  } else if (status === 'running') {
    return 'var(--color-yellow-dark-2)';
  } else if (status === 'blocked') {
    return 'var(--color-purple)';
  }
  return 'var(--color-text-light-3)';
}

function getStatusDotColor(status: ActionsRunStatus): string {
  if (status === 'success') {
    return 'var(--color-green)';
  } else if (status === 'failure') {
    return 'var(--color-red)';
  } else if (status === 'running') {
    return 'var(--color-yellow)';
  }
  return 'var(--color-text-light-2)';
}

function getEdgeColor(edge: BezierEdge): string {
  if (!edge.fromNode || !edge.toNode) {
    return 'var(--color-secondary)';
  }

  const fromStatus = edge.fromNode.status;
  const toStatus = edge.toNode.status;

  if (fromStatus === 'failure' || toStatus === 'failure') {
    return 'var(--color-red)';
  }

  if (fromStatus === 'running') {
    return 'var(--color-yellow)';
  }

  if (toStatus === 'running' && fromStatus === 'success') {
    return 'var(--color-primary)';
  }

  if (fromStatus === 'success' && toStatus === 'success') {
    return 'var(--color-green)';
  }

  if (fromStatus === 'success' && (toStatus === 'waiting' || toStatus === 'blocked')) {
    return 'var(--color-primary-light)';
  }

  if (fromStatus === 'waiting' || fromStatus === 'blocked') {
    return 'var(--color-text-light-2)';
  }

  if (fromStatus === 'cancelled' || toStatus === 'cancelled') {
    return 'var(--color-text-light-2)';
  }

  return 'var(--color-secondary)';
}

function getDisplayName(name: string): string {
  const maxChars = 26;
  if (name.length <= maxChars) {
    return name;
  }

  return name.substring(0, maxChars - 3) + '...';
}

function formatStatus(status: ActionsRunStatus): string {
  const statusMap: Record<ActionsRunStatus, string> = {
    skipped: 'Skipped',
    unknown: 'Unknown',
    success: 'Success',
    failure: 'Failed',
    running: 'Running',
    waiting: 'Waiting',
    cancelled: 'Cancelled',
    blocked: 'Blocked'
  };
  return statusMap[status] || status;
}

function getEdgeStyle(edge: BezierEdge) {
  if (!edge.fromNode || !edge.toNode) {
    return {
      'stroke': 'var(--color-secondary)',
      'stroke-width': '2',
      'opacity': '0.7',
    };
  }

  const fromStatus = edge.fromNode.status;
  const toStatus = edge.toNode.status;
  const isHighlighted = isEdgeHighlighted(edge);

  return {
    'stroke': getEdgeColor(edge),
    'stroke-width': isHighlighted ? '3' : getStrokeWidth(fromStatus, toStatus),
    'stroke-dasharray': getDashArray(fromStatus, toStatus),
    'opacity': isHighlighted ? 1 : getEdgeOpacity(fromStatus, toStatus),
    'transition': 'all 0.2s ease',
  };
}

function getStrokeWidth(fromStatus: ActionsRunStatus, toStatus: ActionsRunStatus): string {
  if (fromStatus === 'running' || toStatus === 'running') {
    return '3';
  }

  if (fromStatus === 'failure' || toStatus === 'failure') {
    return '2.5';
  }

  return '2';
}

function getDashArray(fromStatus: ActionsRunStatus, toStatus: ActionsRunStatus): string {
  if (fromStatus === 'waiting' || toStatus === 'waiting') {
    return '5,3';
  }

  if (fromStatus === 'blocked') {
    return '8,4';
  }

  if (fromStatus === 'cancelled' || toStatus === 'cancelled') {
    return '3,6';
  }

  return 'none';
}

function getEdgeOpacity(fromStatus: ActionsRunStatus, toStatus: ActionsRunStatus): number {
  if (fromStatus === 'success' && toStatus === 'success') {
    return 0.6;
  }

  if (fromStatus === 'failure' || toStatus === 'failure') {
    return 1;
  }

  if (fromStatus === 'running' || toStatus === 'running') {
    return 1;
  }

  return 0.8;
}

function getEdgeClass(edge: BezierEdge): string {
  if (!edge.fromNode || !edge.toNode) return '';

  const fromStatus = edge.fromNode.status;
  const toStatus = edge.toNode.status;

  const classes: string[] = ['node-edge'];

  if (fromStatus === 'running' || toStatus === 'running') {
    classes.push('running-edge');
  }

  if (fromStatus === 'success' && toStatus === 'success') {
    classes.push('success-edge');
  }

  if (fromStatus === 'failure' || toStatus === 'failure') {
    classes.push('failure-edge');
  }

  if (fromStatus === 'waiting' || toStatus === 'waiting') {
    classes.push('waiting-edge');
  }

  return classes.join(' ');
}

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
  if (job.index === props.currentJobIndex) return;

  const link = `${props.runLink}/jobs/${job.index}`;
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
        <span v-if="graphMetrics" class="graph-metrics">
          • {{ graphMetrics.successRate }} success
        </span>
      </div>
      <div class="flex-text-block">
        <button @click="zoomIn" class="ui compact tiny icon button" title="Zoom in">
          <SvgIcon name="octicon-zoom-in" :size="12"/>
        </button>
        <button @click="resetView" class="ui compact tiny icon button" title="Reset view">
          <SvgIcon name="octicon-sync" :size="12"/>
        </button>
        <button @click="zoomOut" class="ui compact tiny icon button" title="Zoom out">
          <SvgIcon name="octicon-zoom-out" :size="12"/>
        </button>
      </div>
    </div>

    <div
      class="graph-container"
      ref="graphContainer"
      @mousedown="handleMouseDown"
      :class="{ 'dragging': isDragging }"
    >
      <svg
        :width="graphWidth"
        :height="graphHeight"
        class="graph-svg"
        :style="{
          transform: `translate(${translateX}px, ${translateY}px) scale(${scale})`,
          transformOrigin: '0 0'
        }"
      >
        <path
          v-for="edge in bezierEdges"
          :key="edge.key"
          :d="edge.path"
          fill="none"
          v-bind="getEdgeStyle(edge)"
          :class="[
            getEdgeClass(edge),
            { 'highlighted-edge': isEdgeHighlighted(edge) }
          ]"
        />

        <g
          v-for="job in jobsWithLayout"
          :key="job.id"
          :class="{'current-job': job.index === currentJobIndex}"
          class="job-node-group"
          @click="onNodeClick(job, $event)"
          @mouseenter="handleNodeMouseEnter(job)"
          @mouseleave="handleNodeMouseLeave"
        >
          <rect
            :x="job.x"
            :y="job.y"
            :width="nodeWidth"
            :height="nodeHeight"
            rx="8"
            :fill="getNodeColor(job.status)"
            :stroke="job.index === currentJobIndex ? 'var(--color-primary)' : 'var(--color-card-border)'"
            :stroke-width="job.index === currentJobIndex ? '3' : '2'"
            class="job-rect"
          />

          <rect
            v-if="job.status === 'running'"
            :x="job.x"
            :y="job.y"
            :width="nodeWidth"
            :height="nodeHeight"
            rx="8"
            fill="url(#running-gradient)"
            opacity="0.3"
            class="running-background"
          />
          <text
            :x="job.x + 8"
            :y="job.y + 18"
            fill="white"
            font-size="12"
            text-anchor="start"
            class="job-name"
          >
            {{ getDisplayName(job.name) }}
          </text>

          <text
            v-if="job.duration || (job.status === 'success' || job.status === 'failure')"
            :x="job.x + nodeWidth - 10"
            :y="job.y + nodeHeight - 25"
            fill="rgba(255,255,255,0.7)"
            font-size="9"
            text-anchor="end"
            class="job-duration"
          >
            {{ job.duration }}
          </text>

          <text
            :x="job.x + nodeWidth - 10"
            :y="job.y + nodeHeight - 8"
            fill="rgba(255,255,255,0.9)"
            font-size="10"
            text-anchor="end"
            class="job-status"
          >
            {{ formatStatus(job.status) }}
          </text>

          <rect
            v-if="job.status === 'running'"
            :x="job.x + 2"
            :y="job.y + nodeHeight - 6"
            :width="(nodeWidth - 4) * 0.5"
            height="4"
            rx="2"
            :fill="getStatusDotColor('running')"
            class="progress-bar"
          >
            <animate
              attributeName="width"
              values="0; 100"
              dur="2s"
              repeatCount="indefinite"
              calcMode="spline"
              keySplines="0.4, 0, 0.2, 1"
            />
          </rect>

          <text
            v-if="job.needs?.length"
            :x="job.x + nodeWidth / 2"
            :y="job.y - 8"
            fill="var(--color-text-light-2)"
            font-size="10"
            text-anchor="middle"
            class="job-deps-label"
          >
            ← {{ job.needs.length }} deps
          </text>
        </g>

        <defs>
          <linearGradient id="running-gradient" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" :stop-color="getStatusDotColor('running')" stop-opacity="0.2"/>
            <stop offset="50%" :stop-color="getStatusDotColor('running')" stop-opacity="0.4"/>
            <stop offset="100%" :stop-color="getStatusDotColor('running')" stop-opacity="0.2"/>
          </linearGradient>
        </defs>
      </svg>
    </div>
  </div>
</template>

<style scoped>
.workflow-graph {
  position: relative;
}

.graph-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding: 6px 12px;
  border-bottom: 1px solid var(--color-secondary-alpha-20);
  gap: 15px;
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
  color: var(--color-text-light-2);
  font-size: 13px;
  white-space: nowrap;
}

.graph-metrics {
  color: var(--color-primary);
  font-weight: var(--font-weight-medium);
}

.graph-controls {
  display: flex;
  align-items: center;
  gap: 10px;
}

.graph-container {
  overflow: auto;
  padding: 12px;
  border-radius: 8px;
  cursor: grab;
  min-height: 300px;
  max-height: 600px;
  position: relative;
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
}

.highlighted-edge {
  stroke-width: 3 !important;
  opacity: 1 !important;
  stroke: var(--color-primary) !important;
}

.job-node-group {
  cursor: pointer;
  transition: all 0.2s ease;
  --node-width: v-bind(nodeWidth + "px");
}

.job-node-group:hover .job-rect {
  filter: brightness(1.1);
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  z-index: 10;
}

.job-node-group.current-job {
  cursor: default;
}

.job-node-group.current-job .job-rect {
  filter: drop-shadow(0 0 8px color-mix(in srgb, var(--color-primary) 30%, transparent));
}

.job-name {
  max-width: calc(var(--node-width, 150px) - 50px);
  text-overflow: ellipsis;
  overflow: hidden;
  white-space: nowrap;
  user-select: none;
  pointer-events: none;
}

.job-status,
.job-duration,
.job-deps-label {
  user-select: none;
  pointer-events: none;
}

@keyframes shimmer {
  0% {
    background-position: -200px 0;
  }
  100% {
    background-position: calc(200px + 100%) 0;
  }
}

.running-background {
  animation: shimmer 2s infinite linear;
  background-size: 200px 100%;
}

@keyframes flowRunning {
  0% {
    stroke-dashoffset: 20;
    stroke-opacity: 0.7;
  }
  50% {
    stroke-opacity: 1;
  }
  100% {
    stroke-dashoffset: 0;
    stroke-opacity: 0.7;
  }
}

@keyframes pulseFailure {
  0%, 100% {
    stroke-width: 2.5;
    opacity: 0.7;
  }
  50% {
    stroke-width: 3;
    opacity: 1;
    filter: drop-shadow(0 0 4px color-mix(in srgb, var(--color-red) 50%, transparent));
  }
}

@keyframes shimmerEdge {
  0% {
    stroke-dashoffset: 20;
  }
  100% {
    stroke-dashoffset: 0;
  }
}

.node-edge.running-edge {
  stroke-dasharray: 10, 5;
  animation: flowRunning 1s linear infinite;
}

.node-edge.failure-edge {
  animation: pulseFailure 0.8s ease-in-out infinite;
}

.node-edge.waiting-edge {
  stroke-dasharray: 5, 3;
  animation: shimmerEdge 2s linear infinite;
}

.node-edge.success-edge {
  transition: stroke-width 0.3s ease, opacity 0.3s ease;
}

.node-edge.success-edge:hover {
  stroke-width: 3;
  opacity: 1;
}

.progress-bar {
  animation: progressPulse 2s ease-in-out infinite;
}

@keyframes progressPulse {
  0%, 100% {
    opacity: 0.8;
  }
  50% {
    opacity: 1;
  }
}

@media (max-width: 768px) {
  .graph-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }

  .graph-stats {
    font-size: 12px;
  }

  .workflow-graph {
    padding: 15px;
  }
}
</style>
