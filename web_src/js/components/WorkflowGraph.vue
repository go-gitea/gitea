<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue';
import {SvgIcon} from '../svg.ts';
import ActionRunStatus from './ActionRunStatus.vue';
import {localUserSettings} from '../modules/user-settings.ts';
import {isPlainClick} from '../utils/dom.ts';
import {debounce} from 'throttle-debounce';
import type {ActionsJob} from '../modules/gitea-actions.ts';
import type {ActionRunViewStore} from './ActionRunView.ts';
import {
  boxBottom,
  boxCenterY,
  computeGraphHighlightState,
  createWorkflowGraphModel,
  getWorkflowGraphLayoutOptions,
  type GraphNode,
  type RoutedEdge,
  type WorkflowGraphModel,
} from './WorkflowGraph.utils.ts';

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
}>();

const settingKeyStates = 'actions-graph-states';
const maxStoredStates = 10;
const layout = getWorkflowGraphLayoutOptions();

const scale = ref(1);
const translateX = ref(0);
const translateY = ref(0);
const isDragging = ref(false);
const lastMousePos = ref({x: 0, y: 0});
const graphContainer = ref<HTMLElement | null>(null);
const hoveredGraphId = ref<string | null>(null);

const stateKey = () => `${props.store.viewData.currentRun.repoId}-${props.workflowId}`;
const expandedMatrixKeys = ref<Set<string>>(new Set());

function isMatrixExpanded(key: string): boolean {
  return expandedMatrixKeys.value.has(key);
}

function toggleMatrixExpanded(key: string) {
  const next = new Set(expandedMatrixKeys.value);
  if (next.has(key)) next.delete(key);
  else next.add(key);
  expandedMatrixKeys.value = next;
}

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

const graphModel = ref<WorkflowGraphModel>({nodes: [], edges: [], routedEdges: [], sharedSegments: [], adjacency: {incomingByNodeId: new Map(), outgoingByNodeId: new Map()}});
watch([() => props.jobs, expandedMatrixKeys], async ([jobs, keys]) => {
  graphModel.value = await createWorkflowGraphModel(jobs, keys);
}, {immediate: true});
const jobsWithLayout = computed(() => graphModel.value.nodes);
const edges = computed(() => graphModel.value.edges);
const routedEdges = computed<RoutedEdge[]>(() => graphModel.value.routedEdges);

const nodeWidth = layout.nodeWidth;
const graphWidth = computed(() => {
  if (jobsWithLayout.value.length === 0) return 800;
  const maxX = Math.max(...jobsWithLayout.value.map((job) => job.x + nodeWidth));
  return maxX + layout.margin * 2;
});

const graphHeight = computed(() => {
  if (jobsWithLayout.value.length === 0) return 400;
  const maxY = Math.max(...jobsWithLayout.value.map((job) => boxBottom(job)));
  return maxY + layout.margin * 2;
});

const successRateLabel = computed(() => {
  if (props.jobs.length === 0) return '0%';
  const successCount = props.jobs.filter((job) => job.status === 'success').length;
  return `${((successCount / props.jobs.length) * 100).toFixed(0)}%`;
});

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
  const target = e.target as Element;
  const interactive = target.closest('div, p, a, span, button, input, text, .job-node-group');
  if (interactive?.closest('svg')) return;

  e.preventDefault();
  isDragging.value = true;
  lastMousePos.value = {x: e.clientX, y: e.clientY};
  if (graphContainer.value) graphContainer.value.style.cursor = 'grabbing';
}

function handleMouseMoveOnDocument(event: MouseEvent) {
  if (!isDragging.value) return;

  translateX.value += event.clientX - lastMousePos.value.x;
  translateY.value += event.clientY - lastMousePos.value.y;
  lastMousePos.value = {x: event.clientX, y: event.clientY};
}

function handleMouseUpOnDocument() {
  if (!isDragging.value) return;
  isDragging.value = false;
  if (graphContainer.value) graphContainer.value.style.cursor = 'grab';
}

function handleWheel(event: WheelEvent) {
  if (!event.ctrlKey && !event.metaKey) return;
  event.preventDefault();
  const zoomFactor = Math.exp(-event.deltaY * 0.0015);
  zoomTo(scale.value * zoomFactor);
}

onMounted(() => {
  loadSavedState();
  watch([translateX, translateY, scale], debounce(500, saveState));
  document.addEventListener('mousemove', handleMouseMoveOnDocument);
  document.addEventListener('mouseup', handleMouseUpOnDocument);
});

onUnmounted(() => {
  document.removeEventListener('mousemove', handleMouseMoveOnDocument);
  document.removeEventListener('mouseup', handleMouseUpOnDocument);
});

function handleNodeMouseEnter(id: string) {
  hoveredGraphId.value = id;
}

function handleNodeMouseLeave() {
  hoveredGraphId.value = null;
}

const highlightState = computed(() => computeGraphHighlightState(hoveredGraphId.value, graphModel.value.adjacency));

function isNodeHighlighted(nodeId: string): boolean {
  return highlightState.value.nodeIds.has(nodeId);
}

function isEdgeHighlighted(edge: RoutedEdge): boolean {
  return highlightState.value.edgeKeys.has(edge.key);
}

const splitRoutedEdges = computed(() => {
  const highlighted: RoutedEdge[] = [];
  const dimmed: RoutedEdge[] = [];
  for (const edge of routedEdges.value) (isEdgeHighlighted(edge) ? highlighted : dimmed).push(edge);
  return {highlighted, dimmed};
});

const nodesWithIncomingEdge = computed(() => new Set(graphModel.value.adjacency.incomingByNodeId.keys()));
const nodesWithOutgoingEdge = computed(() => new Set(graphModel.value.adjacency.outgoingByNodeId.keys()));

function onNodeClick(job: GraphNode | ActionsJob, event: MouseEvent) {
  const jobId = 'jobs' in job ? job.jobs[0]!.id : job.id;
  const link = `${props.runLink}/jobs/${jobId}`;
  if (event.ctrlKey || event.metaKey) {
    window.open(link, '_blank');
    return;
  }
  window.location.href = link;
}
</script>

<template>
  <div v-if="jobs.length > 0" class="workflow-graph">
    <div class="graph-header">
      <h4 class="graph-title">Workflow Dependencies</h4>
      <div class="graph-stats">
        {{ jobs.length }} jobs • {{ edges.length }} dependencies
        • <span class="graph-metrics">{{ successRateLabel }} success</span>
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
      ref="graphContainer"
      class="graph-container"
      :class="{dragging: isDragging}"
      @mousedown="handleMouseDown"
      @wheel="handleWheel"
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
        <defs>
          <mask :id="`workflow-graph-edge-mask-${workflowId}`">
            <rect :width="graphWidth" :height="graphHeight" fill="white"/>
            <rect
              v-for="job in jobsWithLayout"
              :key="`mask-${job.id}`"
              :x="job.x"
              :y="job.y"
              :width="nodeWidth"
              :height="job.displayHeight"
              rx="6"
              fill="black"
            />
          </mask>
        </defs>

        <g :mask="`url(#workflow-graph-edge-mask-${workflowId})`">
          <path
            v-for="edge in splitRoutedEdges.dimmed"
            :key="edge.key"
            :d="edge.path"
            fill="none"
            class="node-edge"
          />
          <path
            v-for="edge in splitRoutedEdges.highlighted"
            :key="`highlight-${edge.key}`"
            :d="edge.path"
            fill="none"
            class="node-edge highlighted-edge"
          />
        </g>

        <template v-for="job in jobsWithLayout" :key="job.id">
          <g
            v-if="job.type === 'matrix'"
            class="job-node-group matrix-job-group"
            :class="{ 'related-node': isNodeHighlighted(job.id) }"
            @mouseenter="handleNodeMouseEnter(job.id)"
            @mouseleave="handleNodeMouseLeave"
          >
            <title>Matrix: {{ job.matrixKey }}</title>
            <rect :x="job.x" :y="job.y" :width="nodeWidth" :height="job.displayHeight" rx="6" class="job-rect"/>
            <foreignObject :x="job.x" :y="job.y" :width="nodeWidth" :height="job.displayHeight" class="matrix-foreign-object">
              <div class="matrix-panel" xmlns="http://www.w3.org/1999/xhtml" @click.stop="toggleMatrixExpanded(job.matrixKey!)">
                <template v-if="!isMatrixExpanded(job.matrixKey!)">
                  <div class="matrix-panel-collapsed">
                    <ActionRunStatus :status="job.status"/>
                    <span class="matrix-panel-summary">{{ job.jobs!.length }} jobs completed</span>
                  </div>
                  <span class="matrix-panel-toggle">Show all jobs</span>
                </template>
                <template v-else>
                  <div class="matrix-panel-jobs">
                    <div
                      v-for="ch in job.jobs"
                      :key="ch.id"
                      class="graph-list-row"
                      @mouseenter="handleNodeMouseEnter(job.id)"
                      @click.stop="onNodeClick(ch, $event)"
                    >
                      <div class="graph-list-row-main">
                        <ActionRunStatus :status="ch.status"/>
                        <span class="graph-list-row-name">{{ ch.name }}</span>
                      </div>
                      <span class="graph-list-row-duration">{{ ch.duration }}</span>
                    </div>
                  </div>
                  <span class="matrix-panel-toggle">Hide jobs</span>
                </template>
              </div>
            </foreignObject>
            <text :x="job.x + 12" :y="job.y + 4" class="matrix-label-text">Matrix: {{ job.matrixKey }}</text>
            <circle v-if="nodesWithIncomingEdge.has(job.id)" :cx="job.x" :cy="boxCenterY(job)" r="3.5" class="node-port"/>
            <circle v-if="nodesWithOutgoingEdge.has(job.id)" :cx="job.x + nodeWidth" :cy="boxCenterY(job)" r="3.5" class="node-port"/>
          </g>

          <g
            v-else-if="job.type === 'group'"
            class="job-node-group grouped-job-group"
            :class="{ 'related-node': isNodeHighlighted(job.id) }"
            @mouseenter="handleNodeMouseEnter(job.id)"
            @mouseleave="handleNodeMouseLeave"
          >
            <title>{{ job.name }}</title>
            <rect :x="job.x" :y="job.y" :width="nodeWidth" :height="job.displayHeight" rx="6" class="job-rect"/>
            <foreignObject :x="job.x" :y="job.y" :width="nodeWidth" :height="job.displayHeight" class="matrix-foreign-object">
              <div class="grouped-panel" xmlns="http://www.w3.org/1999/xhtml" @click.stop>
                <div
                  v-for="ch in job.jobs"
                  :key="ch.id"
                  class="graph-list-row"
                  @mouseenter="handleNodeMouseEnter(job.id)"
                  @click="onNodeClick(ch, $event)"
                >
                  <div class="graph-list-row-main">
                    <ActionRunStatus :status="ch.status"/>
                    <span class="graph-list-row-name">{{ ch.name }}</span>
                  </div>
                  <span class="graph-list-row-duration">{{ ch.duration }}</span>
                </div>
              </div>
            </foreignObject>
            <circle v-if="nodesWithIncomingEdge.has(job.id)" :cx="job.x" :cy="boxCenterY(job)" r="3.5" class="node-port"/>
            <circle v-if="nodesWithOutgoingEdge.has(job.id)" :cx="job.x + nodeWidth" :cy="boxCenterY(job)" r="3.5" class="node-port"/>
          </g>

          <g
            v-else
            class="job-node-group"
            :class="{ 'related-node': isNodeHighlighted(job.id) }"
            @click="onNodeClick(job, $event)"
            @mouseenter="handleNodeMouseEnter(job.id)"
            @mouseleave="handleNodeMouseLeave"
          >
            <title>{{ job.name }}</title>
            <rect :x="job.x" :y="job.y" :width="nodeWidth" :height="job.displayHeight" rx="6" class="job-rect"/>
            <foreignObject :x="job.x + 10" :y="job.y + 6" :width="nodeWidth - 20" :height="job.displayHeight - 12">
              <div class="job-row job-card" xmlns="http://www.w3.org/1999/xhtml">
                <div class="job-row-main">
                  <ActionRunStatus :status="job.status"/>
                  <span class="job-name">{{ job.name }}</span>
                </div>
                <span class="job-duration">{{ job.duration }}</span>
              </div>
            </foreignObject>
            <circle v-if="nodesWithIncomingEdge.has(job.id)" :cx="job.x" :cy="boxCenterY(job)" r="3.5" class="node-port"/>
            <circle v-if="nodesWithOutgoingEdge.has(job.id)" :cx="job.x + nodeWidth" :cy="boxCenterY(job)" r="3.5" class="node-port"/>
          </g>
        </template>
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
  overflow: auto;
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
  transition: stroke-width 0.2s ease, opacity 0.2s ease;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.node-edge {
  stroke: var(--color-secondary-dark-2);
  stroke-width: 2;
  opacity: 0.9;
}

.highlighted-edge {
  stroke: var(--color-primary);
  stroke-width: 2;
}

.job-node-group {
  cursor: pointer;
}

.job-node-group:hover .job-rect,
.job-node-group.related-node .job-rect {
  stroke: var(--color-primary);
  stroke-width: 1.5;
  fill: var(--color-hover);
}

.job-rect {
  fill: var(--color-box-body);
  stroke: var(--color-secondary);
  stroke-width: 1;
}

.matrix-foreign-object {
  pointer-events: auto;
  overflow: visible;
}

.matrix-panel,
.grouped-panel {
  width: 100%;
  height: 100%;
  box-sizing: border-box;
  border-radius: 6px;
  background: transparent;
  pointer-events: auto;
  user-select: none;
}

.matrix-panel {
  display: flex;
  flex-direction: column;
  cursor: pointer;
  padding-top: 14px;
}

.matrix-label-text {
  font-size: 11px;
  font-family: var(--fonts-regular);
  fill: var(--color-text-light-2);
  paint-order: stroke;
  stroke: var(--color-box-body);
  stroke-width: 8px;
  stroke-linejoin: round;
  pointer-events: none;
}

.matrix-panel-collapsed {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 12px;
}

.matrix-panel-summary {
  font-size: 12px;
  font-weight: var(--font-weight-semibold);
  color: var(--color-text);
}

.matrix-panel-toggle {
  font-size: 11px;
  color: var(--color-text-light-2);
  padding: 2px 12px 0;
}

.matrix-panel-jobs {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 2px 6px 6px;
}

.grouped-panel {
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 6px;
  gap: 2px;
}

.graph-list-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 24px;
  padding: 1px 6px;
  border-radius: 5px;
}

.graph-list-row:hover {
  background: var(--color-hover);
}

.graph-list-row-main,
.job-row-main {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.graph-list-row-name,
.job-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
  font-weight: var(--font-weight-semibold);
  color: var(--color-text);
}

.graph-list-row-duration,
.job-duration {
  flex: 0 0 auto;
  font-size: 10px;
  color: var(--color-text-light-2);
  white-space: nowrap;
}

.job-row {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.job-card {
  border-radius: 6px;
  padding: 0 2px;
}

.node-port {
  fill: var(--color-secondary-dark-2);
  stroke: var(--color-box-body);
  stroke-width: 1.25;
  opacity: 0.9;
  pointer-events: none;
}

.job-node-group.related-node .node-port {
  fill: var(--color-primary);
}
</style>
