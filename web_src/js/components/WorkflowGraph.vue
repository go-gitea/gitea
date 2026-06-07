<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue';
import {SvgIcon} from '../svg.ts';
import ActionStatusIcon from './ActionStatusIcon.vue';
import {localUserSettings} from '../modules/user-settings.ts';
import {isPlainClick} from '../utils/dom.ts';
import {trN} from '../modules/i18n.ts';
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
  workflowLink?: string;
  triggerEvent?: string;
  locale: Record<string, string>;
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

const graphModel = computed(() => createWorkflowGraphModel(props.jobs, expandedMatrixKeys.value));
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

const graphStats = computed(() => [
  trN(props.jobs.length, props.locale.graphJobsCount1, props.locale.graphJobsCountN),
  trN(edges.value.length, props.locale.graphDependenciesCount1, props.locale.graphDependenciesCountN),
  props.locale.graphSuccessRate.replace('%s', successRateLabel.value),
].join(' • '));

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
  const target = 'jobs' in job ? job.jobs[0]! : job;
  // Reusable callers have no per-job detail page; clicking them is a no-op so the graph
  // doesn't lead users to a dead destination.
  if (target.isReusableCaller) return;
  const link = `${props.runLink}/jobs/${target.id}`;
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
      <div class="graph-workflow-info">
        <a v-if="workflowLink" class="graph-workflow-name silenced" :href="workflowLink">{{ workflowId }}</a>
        <span v-else class="graph-workflow-name">{{ workflowId }}</span>
        <div v-if="triggerEvent" class="graph-workflow-trigger">on: {{ triggerEvent }}</div>
      </div>
      <div class="graph-stats">{{ graphStats }}</div>
      <div class="flex-text-block graph-controls">
        <button
          type="button"
          @click="zoomIn"
          class="ui compact tiny icon button"
          :disabled="!canZoomIn"
          :title="canZoomIn ? locale.graphZoomIn : locale.graphZoomMax"
        >
          <SvgIcon name="octicon-zoom-in" :size="12"/>
        </button>
        <button type="button" @click="resetView" class="ui compact tiny icon button" :title="locale.graphResetView">
          <SvgIcon name="octicon-sync" :size="12"/>
        </button>
        <button type="button" @click="zoomOut" class="ui compact tiny icon button" :title="locale.graphZoomOut">
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
        :class="{ 'has-hover': hoveredGraphId !== null }"
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
              <div class="matrix-panel" xmlns="http://www.w3.org/1999/xhtml">
                <div class="matrix-panel-label" @click.stop="toggleMatrixExpanded(job.matrixKey!)">Matrix: {{ job.matrixKey }}</div>
                <div
                  v-if="!isMatrixExpanded(job.matrixKey!)"
                  class="matrix-panel-collapsed"
                  @click.stop="toggleMatrixExpanded(job.matrixKey!)"
                >
                  <div class="matrix-panel-summary-row">
                    <ActionStatusIcon :status="job.status" icon-variant="circle-fill"/>
                    <span class="matrix-panel-summary">{{ job.jobs.length }} jobs completed</span>
                  </div>
                  <span class="matrix-panel-toggle">Show all jobs</span>
                </div>
                <div v-else class="matrix-panel-jobs">
                  <div
                    v-for="ch in job.jobs"
                    :key="ch.id"
                    class="graph-list-row"
                    @mouseenter="handleNodeMouseEnter(job.id)"
                    @click.stop="onNodeClick(ch, $event)"
                  >
                    <div class="graph-list-row-main">
                      <ActionStatusIcon :status="ch.status" icon-variant="circle-fill"/>
                      <span class="graph-list-row-name">{{ ch.name }}</span>
                    </div>
                    <span class="graph-list-row-duration">{{ ch.duration }}</span>
                  </div>
                </div>
              </div>
            </foreignObject>
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
                    <ActionStatusIcon :status="ch.status" icon-variant="circle-fill"/>
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
            :class="{ 'related-node': isNodeHighlighted(job.id), 'caller-node': job.jobs[0]!.isReusableCaller }"
            @click="onNodeClick(job, $event)"
            @mouseenter="handleNodeMouseEnter(job.id)"
            @mouseleave="handleNodeMouseLeave"
          >
            <title>{{ job.name }}</title>
            <rect :x="job.x" :y="job.y" :width="nodeWidth" :height="job.displayHeight" rx="6" class="job-rect"/>
            <foreignObject :x="job.x + 10" :y="job.y + 6" :width="nodeWidth - 20" :height="job.displayHeight - 12">
              <div class="job-row job-card" xmlns="http://www.w3.org/1999/xhtml">
                <div class="job-row-main">
                  <ActionStatusIcon :status="job.status" icon-variant="circle-fill"/>
                  <span class="job-name">{{ job.name }}</span>
                </div>
                <span class="job-duration">{{ job.duration }}</span>
              </div>
            </foreignObject>
            <circle v-if="nodesWithIncomingEdge.has(job.id)" :cx="job.x" :cy="boxCenterY(job)" r="3.5" class="node-port"/>
            <circle v-if="nodesWithOutgoingEdge.has(job.id)" :cx="job.x + nodeWidth" :cy="boxCenterY(job)" r="3.5" class="node-port"/>
          </g>
        </template>

        <!-- Highlighted edges render on top of nodes so they remain visible across dimmed boxes. -->
        <g class="highlighted-edge-layer">
          <path
            v-for="edge in splitRoutedEdges.highlighted"
            :key="`highlight-${edge.key}`"
            :d="edge.path"
            fill="none"
            class="node-edge highlighted-edge"
          />
          <template v-for="edge in splitRoutedEdges.highlighted" :key="`highlight-port-${edge.key}`">
            <circle :cx="edge.fromNode.x + nodeWidth" :cy="boxCenterY(edge.fromNode)" r="3.5" class="node-port highlighted-port"/>
            <circle :cx="edge.toNode.x" :cy="boxCenterY(edge.toNode)" r="3.5" class="node-port highlighted-port"/>
          </template>
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
  padding: 16px 16px 8px;
  background: var(--color-console-bg);
  gap: var(--gap-block);
  flex-wrap: wrap;
}

.graph-workflow-info {
  min-width: 0;
}

.graph-workflow-name {
  display: block;
  color: var(--color-text);
  font-size: 16px;
  font-weight: var(--font-weight-semibold);
  line-height: 1.25;
}

.graph-workflow-trigger {
  margin-top: 4px;
  color: var(--color-text-light-2);
  font-size: 12px;
  line-height: 1.4;
}

.graph-stats {
  display: flex;
  align-items: baseline;
  column-gap: 8px;
  color: var(--color-text-light-1);
  font-size: 13px;
  white-space: nowrap;
  margin-left: auto;
  padding: 0 16px;
}

.graph-controls {
  flex-shrink: 0;
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
  stroke-width: 1.5;
  opacity: 0.9;
}

.highlighted-edge {
  stroke: var(--color-primary);
  stroke-width: 2;
}

.job-node-group {
  cursor: pointer;
  transition: opacity 0.15s ease;
}

.job-node-group.caller-node {
  cursor: default;
}

.job-node-group:hover .job-rect,
.job-node-group.related-node .job-rect {
  stroke: var(--color-primary);
  stroke-width: 1.5;
  fill: var(--color-primary-alpha-10);
}

.graph-svg.has-hover .job-node-group:not(.related-node) {
  opacity: 0.2;
}

.graph-svg.has-hover .node-edge:not(.highlighted-edge) {
  opacity: 0.15;
}

.highlighted-edge-layer {
  pointer-events: none;
}

.highlighted-port {
  fill: var(--color-primary);
  stroke: var(--color-primary);
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
  padding: 6px 10px 8px;
}

.matrix-panel-label {
  font-size: 10px;
  font-weight: var(--font-weight-medium);
  color: var(--color-text-light-2);
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}

.matrix-panel-collapsed {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 2px 0 0 2px;
  cursor: pointer;
}

.matrix-panel-summary-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.matrix-panel-summary {
  font-size: 12px;
  font-weight: var(--font-weight-semibold);
  line-height: 1.3;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.matrix-panel-toggle {
  font-size: 11px;
  color: var(--color-text-light-2);
  padding-left: 24px;
  cursor: pointer;
}

.matrix-panel-toggle:hover {
  color: var(--color-primary);
  text-decoration: underline;
}

.matrix-panel-jobs {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 4px 0 0 2px;
  overflow-y: auto;
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
