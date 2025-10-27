<script setup lang="ts">
/* BubbleNode.vue
   This component is responsible for rendering ONE bubble (circle + labels).
   It does NOT know about the graph; it only gets coordinates, radius, and a
   zoom factor (k). When k or size changes, it re-evaluates what text fits.
   This keeps label logic independent from layout and D3. */

import { computed, watch, reactive } from "vue";

/* ──────────────────────────────────────────────────────────────────────────────
   LABEL LAYOUT CONSTANTS (all values explained to avoid "magic numbers")
   ─────────────────────────────────────────────────────────────────────────── */

/* === FONT SIZING === */
const FONT_SIZE_COUNT_MIN = 10;      // Minimum font size for contributor count
const FONT_SIZE_COUNT_MAX = 34;      // Maximum font size for contributor count
const FONT_SIZE_COUNT_SCALE = 0.95;  // Scale factor: count font size = radius * scale
const FONT_SIZE_LABEL = 12;          // Fixed font size for "Contributor(s)" label
const FONT_SIZE_SMALL = 11;          // Font size for "Last updated" lines

/* === LABEL SPACING === */
const LABEL_PADDING = 12;            // Breathing room between bubble edge and labels
const LABEL_GAP_PRIMARY = 6;         // Gap between count and contributor label
const LABEL_GAP_SECONDARY = 6;       // Gap between contributor label and updated block
const LABEL_GAP_UPDATED_INNER = 6;   // Gap between two lines of updated text

/* === TEXT WIDTH ESTIMATION (for fit calculations) === */
const CHAR_WIDTH_RATIO_LABEL = 0.56; // Approximate width of label chars as ratio of font size
const CHAR_WIDTH_RATIO_SMALL = 0.52; // Approximate width of small text chars as ratio of font size

/* === BUTTON SIZING === */
const BUTTON_MARGIN_TOP = 24;        // Margin between copy and button in screen pixels (1.5rem = 24px)
const BUTTON_MIN_RADIUS = 80;        // Minimum bubble radius (in screen pixels) to show button

const props = defineProps<{
  id: string;
  x: number; y: number;          // world coordinates (graph space)
  r: number;                      // bubble radius (graph units)
  k: number;                      // current zoom scale (world→screen)
  contributors: number;           // primary number (always shown)
  updatedAt?: string;             // secondary line if visible
  isActive?: boolean;
}>();

/* Emits so the parent can wire up interactions without D3 binding. */
const emit = defineEmits<{
  (e:"click", id:string, ev:MouseEvent): void;
  (e:"view", id:string, ev:MouseEvent): void;
}>();

/* Label fit model in *screen pixels* so it looks consistent across zoom.
   We inverse-scale the label group by 1/k. */
const fit = reactive({
  showLabel: false,
  showUpdated: false,
  // vertical offset to keep the whole label block visually centered
  shiftPx: 0,
  // font sizes in px (on screen)
  fsCount: FONT_SIZE_LABEL,
  fsLabel: FONT_SIZE_LABEL,
  fsSmall: FONT_SIZE_SMALL,
  stackPx: 0,
});

/* Label text: singularize when needed so UI shows "Contributor" for 1 */
const labelText = computed(()=> props.contributors === 1 ? "Contributor" : "Contributors");

/* Recompute label visibility whenever r or k or updatedAt change. */
function recomputeFit(){
  const k = props.k, r = props.r;
  const Dpx = 2 * r * k;                    // bubble diameter on screen
  const pad = LABEL_PADDING * k;            // breathing room (scaled to zoom)
  const availW = Dpx - 2 * pad;
  const availH = Dpx - 2 * pad;

  // Count font scales with radius but clamped to min/max
  const fsCount = Math.min(FONT_SIZE_COUNT_MAX, Math.max(FONT_SIZE_COUNT_MIN, r * FONT_SIZE_COUNT_SCALE));
  const fsLabel = FONT_SIZE_LABEL;
  const fsSmall = FONT_SIZE_SMALL;

  // Estimate text widths using character width ratios
  const wLabel = labelText.value.length * fsLabel * CHAR_WIDTH_RATIO_LABEL;
  const hasUpd = !!props.updatedAt;
  const updLine1 = "Last updated";
  const updLine2 = props.updatedAt ?? "";
  const wUpd = Math.max(
    updLine1.length * fsSmall * CHAR_WIDTH_RATIO_SMALL, 
    updLine2.length * fsSmall * CHAR_WIDTH_RATIO_SMALL
  );

  // Check if label fits horizontally and vertically
  const showLabel = (wLabel <= availW) && (fsCount / 2 + LABEL_GAP_PRIMARY + fsLabel <= availH / 2);
  const updatedBlockHeight = hasUpd ? (fsSmall * 2 + LABEL_GAP_UPDATED_INNER) : 0;
  const showUpdated = hasUpd && (wUpd <= availW) &&
    (fsCount / 2 + LABEL_GAP_PRIMARY + 
     (showLabel ? (fsLabel + LABEL_GAP_SECONDARY) : 0) + updatedBlockHeight <= availH / 2);

  // Calculate total stack height and vertical centering shift
  const stackPx = fsCount
    + (showLabel ? LABEL_GAP_PRIMARY + fsLabel : 0)
    + (showUpdated ? ((showLabel ? LABEL_GAP_SECONDARY : LABEL_GAP_PRIMARY) + (fsSmall * 2 + LABEL_GAP_UPDATED_INNER)) : 0);
  const shiftPx = (stackPx / 2 - fsCount / 2);

  fit.showLabel   = showLabel;
  fit.showUpdated = showUpdated;
  fit.shiftPx     = shiftPx;
  fit.fsCount     = fsCount;
  fit.fsLabel     = fsLabel;
  fit.fsSmall     = fsSmall;
  fit.stackPx     = stackPx;
}

/* Run once and whenever driving props change. */
watch(() => [props.k, props.r, props.updatedAt, props.contributors], recomputeFit, { immediate: true });

/* Convenience computed transform strings */
const gTransform = computed(() => `translate(${props.x},${props.y})`);

const showButton = computed(() => {
  if (!props.isActive) return false;
  const pixelRadius = props.r * props.k;
  return pixelRadius >= BUTTON_MIN_RADIUS;
});

/* Pointer handlers relay events upward (so parent can focus). */
function onClick(ev:MouseEvent){ emit("click", props.id, ev); }
function onView(ev:MouseEvent | KeyboardEvent){
  ev.preventDefault();
  ev.stopPropagation();
  // Convert KeyboardEvent to MouseEvent-like object for consistency
  const mouseEv = ev as MouseEvent;
  emit("view", props.id, mouseEv);
}

/* Keyboard navigation support */
function onKeyDown(ev:KeyboardEvent){
  if (ev.key === 'Enter' || ev.key === ' ') {
    ev.preventDefault();
    emit("click", props.id, ev as any);
  }
}
</script>

<template>
  <!-- One node group at (x,y); we let the parent group receive the world transform -->
  <g class="node cursor-pointer select-none" 
     :transform="gTransform"
     role="button"
     :aria-label="`Repository node with ${contributors} contributor${contributors === 1 ? '' : 's'}${updatedAt ? ', last updated ' + updatedAt : ''}. Press Enter to select.`"
     :aria-pressed="isActive ? 'true' : 'false'"
     tabindex="0"
     @click="onClick" 
     @keydown="onKeyDown">
    <!-- Bubble circle with soft gradient & subtle stroke/shadow -->
    <circle class="node-circle" :r="r" fill="url(#bubbleGrad)"
            :stroke="isActive ? 'var(--color-primary)' : '#DBE2EA'" 
            :stroke-width="1" 
            filter="url(#softShadow)"/>
    
    <!-- HTML Labels: using foreignObject for efficient text rendering -->
    <!-- Calculate the size needed for the foreignObject container -->
    <foreignObject 
      :x="-r" 
      :y="-r" 
      :width="r * 2" 
      :height="r * 2"
      :transform="`scale(${1/k})`"
      style="overflow: visible; pointer-events: none;">
      <div xmlns="http://www.w3.org/1999/xhtml" class="html-label-wrapper">
        <!-- Count is ALWAYS visible and centered -->
        <div class="count" :style="`font-size: ${fit.fsCount}px;`">
          {{ contributors }}
        </div>
        
        <!-- "Contributors/Contributor": only if fits -->
        <div v-if="fit.showLabel" class="label" 
             :style="`font-size: ${fit.fsLabel}px; margin-top: ${LABEL_GAP_PRIMARY}px;`">
          {{ labelText }}
        </div>
        
        <!-- "Last updated …": only if fits -->
        <div v-if="fit.showUpdated" class="updated" 
             :style="`font-size: ${fit.fsSmall}px; margin-top: ${fit.showLabel ? LABEL_GAP_SECONDARY : LABEL_GAP_PRIMARY}px;`">
          <div>Last updated</div>
          <div :style="`margin-top: ${LABEL_GAP_UPDATED_INNER}px;`">{{ updatedAt }}</div>
        </div>
        
        <!-- View article button: only if active and bubble is large enough -->
        <button v-if="showButton" 
                class="view-button"
                :style="`margin-top: ${BUTTON_MARGIN_TOP}px;`"
                @click="onView"
                @keydown.enter.prevent="onView"
                @keydown.space.prevent="onView"
                aria-label="View article details">
          View article
        </button>
      </div>
    </foreignObject>
  </g>
</template>

<style scoped>
.node-circle {
  transition: stroke 0.2s ease, stroke-width 0.2s ease;
}
.node:focus {
  outline: none;
}
.node:focus .node-circle {
  stroke: var(--color-primary);
  stroke-width: 1;
}

/* HTML Label Wrapper - efficient text rendering */
.html-label-wrapper {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  pointer-events: none;
  height: 100%;
  width: 100%;
}

/* Count: always visible, bold and prominent */
.html-label-wrapper .count {
  color: #1f2937;
  font-weight: 600;
  line-height: 1;
  pointer-events: none;
}

/* Label text: "Contributor(s)" */
.html-label-wrapper .label {
  color: #475569;
  font-weight: 700;
  line-height: 1;
  pointer-events: none;
}

/* Updated date information */
.html-label-wrapper .updated {
  color: #94a3b8;
  line-height: 1;
  pointer-events: none;
}

/* View article button - HTML button with proper styling */
.html-label-wrapper .view-button {
  background-color: var(--color-primary);
  color: #ffffff;
  font-size: 14px;
  font-weight: 600;
  padding: 0.38rem 0.5rem;
  border: none;
  border-radius: 0.375rem;
  cursor: pointer;
  opacity: 0.95;
  transition: background-color 0.2s ease, opacity 0.2s ease;
  pointer-events: auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  white-space: nowrap;
}

.html-label-wrapper .view-button:hover {
  background-color: var(--color-primary-dark, #1d4ed8);
  opacity: 1;
}

.html-label-wrapper .view-button:focus {
  outline: 2px solid #ffffff;
  outline-offset: 2px;
  background-color: var(--color-primary);
}

.html-label-wrapper .view-button:active {
  transform: scale(0.98);
}
</style>
