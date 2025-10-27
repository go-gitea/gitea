<script setup lang="ts">
/* BubbleNode.vue
   This component is responsible for rendering ONE bubble (circle + labels).
   It does NOT know about the graph; it only gets coordinates, radius, and a
   zoom factor (k). When k or size changes, it re-evaluates what text fits.
   This keeps label logic independent from layout and D3. */

import { computed, watch, reactive } from "vue";

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
  (e:"dblclick", id:string, ev:MouseEvent): void;
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
  fsCount: 12, fsLabel: 12, fsSmall: 11,
  stackPx: 0,
});

/* Label text: singularize when needed so UI shows “Contributor” for 1 */
const labelText = computed(()=> props.contributors === 1 ? "Contributor" : "Contributors");

/* Recompute label visibility whenever r or k or updatedAt change. */
function recomputeFit(){
  const k = props.k, r = props.r;
  const Dpx = 2*r*k;                // bubble diameter on screen
  const pad = 12*k;                 // breathing room
  const availW = Dpx - 2*pad;
  const availH = Dpx - 2*pad;

  // count font scales with radius but clamped; other lines fixed
  const fsCount = Math.min(34, Math.max(10, r*0.95));
  const fsLabel = 12, fsSmall = 11;
  const gap1 = 6, gap2 = 6, updInnerGap = 6;

  // count width not used for visibility calculations
  const wLabel = labelText.value.length * fsLabel * 0.56;
  const hasUpd = !!props.updatedAt;
  const updLine1 = "Last updated";
  const updLine2 = props.updatedAt ?? "";
  const wUpd = Math.max(updLine1.length * fsSmall * 0.52, updLine2.length * fsSmall * 0.52);

  const showLabel = (wLabel <= availW) && (fsCount/2 + gap1 + fsLabel <= availH/2);
  const updatedBlockHeight = hasUpd ? (fsSmall * 2 + updInnerGap) : 0;
  const showUpdated = hasUpd && (wUpd <= availW) &&
                      (fsCount/2 + gap1 + (showLabel ? (fsLabel + gap2) : 0) + updatedBlockHeight <= availH/2);

  const stackPx = fsCount
                + (showLabel ? gap1 + fsLabel : 0)
                + (showUpdated ? ((showLabel ? gap2 : gap1) + (fsSmall*2 + updInnerGap)) : 0);
  const shiftPx = (stackPx/2 - fsCount/2);

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
const labelTransform = computed(() => `translate(0, ${-fit.shiftPx/props.k}) scale(${1/props.k})`);

const showButton = computed(() => {
  if (!props.isActive) return false;
  const pixelRadius = props.r * props.k;
  return pixelRadius >= 80;
});

const BUTTON_WIDTH = 300;
const BUTTON_HEIGHT = 92;

const buttonTransform = computed(() => {
  const buttonHeightWorld = BUTTON_HEIGHT / props.k;
  const marginWorld = 32 / props.k;
  const offsetWorld = props.r + (buttonHeightWorld / 2) + marginWorld;
  return `translate(0, ${offsetWorld}) scale(${1/props.k})`;
});

/* Pointer handlers relay events upward (so parent can focus / add / delete). */
function onClick(ev:MouseEvent){ emit("click", props.id, ev); }
function onDblClick(ev:MouseEvent){ ev.preventDefault(); emit("dblclick", props.id, ev); }
function onView(ev:MouseEvent){
  ev.preventDefault();
  ev.stopPropagation();
  emit("view", props.id, ev);
}

/* Keyboard navigation support */
function onKeyDown(ev:KeyboardEvent){
  if (ev.key === 'Enter' || ev.key === ' ') {
    ev.preventDefault();
    emit("click", props.id, ev as any);
  } else if (ev.key === 'Delete' && ev.altKey) {
    ev.preventDefault();
    const mouseEv = new MouseEvent('click', { altKey: true });
    emit("click", props.id, mouseEv);
  }
}
</script>

<template>
  <!-- One node group at (x,y); we let the parent group receive the world transform -->
  <g class="node cursor-pointer select-none" 
     :transform="gTransform"
     role="button"
     :aria-label="`Repository node with ${contributors} contributor${contributors === 1 ? '' : 's'}${updatedAt ? ', last updated ' + updatedAt : ''}. Press Enter to select, Alt+Delete to remove, or double-click to expand.`"
     :aria-pressed="isActive ? 'true' : 'false'"
     tabindex="0"
     @click="onClick" 
     @dblclick="onDblClick"
     @keydown="onKeyDown">
    <!-- Bubble circle with soft gradient & subtle stroke/shadow -->
    <circle class="node-circle" :r="r" fill="url(#bubbleGrad)"
            :stroke="isActive ? '#2563eb' : '#DBE2EA'" 
            :stroke-width="isActive ? '2.5' : '1.2'" 
            filter="url(#softShadow)"/>
    <!-- Labels: inversely scaled so font sizes stay constant on screen -->
    <g class="label-zoomfix" text-anchor="middle" :transform="labelTransform">
      <!-- Count is ALWAYS visible and centered -->
      <text class="count" dominant-baseline="central"
            :style="`font-size:${fit.fsCount}px`" fill="#1f2937" font-weight="600">
        {{ contributors }}
      </text>
      <!-- “Contributors/Contributor”: bold; only if fits -->
      <text v-if="fit.showLabel" class="lbl" :y="fit.fsCount/2 + 6" :style="`font-size:${fit.fsLabel}px`"
            fill="#475569" font-weight="700" dominant-baseline="hanging">{{ labelText }}</text>
      <!-- “Last updated …”: only if fits -->
      <g v-if="fit.showUpdated" class="upd" fill="#94a3b8" text-anchor="middle">
        <text dominant-baseline="hanging"
              :y="fit.fsCount/2 + (fit.showLabel ? (6 + fit.fsLabel + 6) : (6))"
              :style="`font-size:${fit.fsSmall}px`">Last updated</text>
        <text dominant-baseline="hanging"
              :y="fit.fsCount/2 + (fit.showLabel ? (6 + fit.fsLabel + 6) : (6)) + fit.fsSmall + 6"
              :style="`font-size:${fit.fsSmall}px`">{{ updatedAt }}</text>
      </g>
      <g v-if="showButton" 
         class="view-button" 
         :transform="buttonTransform" 
         role="button"
         aria-label="View article details"
         tabindex="0"
         @click="onView"
         @keydown.enter.prevent="onView"
         @keydown.space.prevent="onView">
        <rect :x="-(BUTTON_WIDTH/2)" :y="-(BUTTON_HEIGHT/2)" :width="BUTTON_WIDTH" :height="BUTTON_HEIGHT" rx="14" />
        <text dominant-baseline="middle" text-anchor="middle" y="1">{{ 'View article' }}</text>
      </g>
    </g>
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
  stroke: #3b82f6;
  stroke-width: 3;
  filter: drop-shadow(0 0 8px rgba(59, 130, 246, 0.6));
}
.view-button {
  cursor: pointer;
}
.view-button:focus {
  outline: none;
}
.view-button:focus rect {
  stroke: #ffffff;
  stroke-width: 3;
  filter: drop-shadow(0 0 8px rgba(255, 255, 255, 0.8));
}
.view-button:hover rect,
.view-button:focus rect {
  fill: #1d4ed8;
}
.view-button rect {
  fill: #2563eb;
  opacity: 0.95;
  transition: fill 0.2s ease;
}
.view-button text {
  fill: #ffffff;
  font-size: 20px;
  font-weight: 600;
  pointer-events: none;
}
</style>
