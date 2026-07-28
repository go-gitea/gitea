<script lang="ts" setup>
import {svg} from '../../svg.ts';
import type {WorkflowEvent} from './WorkflowStore.ts';

const props = defineProps<{
  workflows: WorkflowEvent[];
  selectedId: string | null;
  heading: string;
  getDisplayName: (item: WorkflowEvent, index: number) => string;
  getStatusClass: (item: WorkflowEvent) => string;
}>();

const emit = defineEmits<{
  select: [item: WorkflowEvent];
}>();
</script>

<template>
  <div class="workflow-sidebar">
    <div class="sidebar-header">
      <h3>{{ heading }}</h3>
    </div>
    <div class="sidebar-content">
      <div class="workflow-items">
        <div
          v-for="(item, index) in workflows"
          :key="`workflow-${item.event_id}-${item.is_configured ? 'configured' : 'unconfigured'}`"
          class="workflow-item"
          :class="{ active: selectedId === item.event_id }"
          @click="emit('select', item)"
        >
          <div class="workflow-content">
            <div class="workflow-info">
              <span class="status-indicator">
                <!-- eslint-disable-next-line vue/no-v-html -->
                <span v-html="svg('octicon-dot-fill')" :class="getStatusClass(item)"/>
              </span>
              <div class="workflow-details">
                <div class="workflow-title">{{ getDisplayName(item, index) }}</div>
                <div v-if="item.summary" class="workflow-subtitle">{{ item.summary }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.workflow-sidebar {
  width: 350px;
  flex-shrink: 0;
  background: var(--color-secondary-bg);
  border-right: 1px solid var(--color-secondary);
  display: flex;
  flex-direction: column;
}

.sidebar-header {
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--color-secondary);
  background: var(--color-secondary-bg);
}

.sidebar-header h3 {
  margin: 0;
  color: var(--color-text);
  font-size: 1.1rem;
  font-weight: 600;
}

.sidebar-content {
  flex: 1;
  padding: 1rem;
  overflow-y: auto;
}

.workflow-items {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.workflow-item {
  padding: 0.75rem 1rem;
  cursor: pointer;
  transition: all 0.2s ease;
  border-radius: 6px;
  margin-bottom: 0.25rem;
}

.workflow-item:hover { background: var(--color-hover); }

.workflow-item.active {
  background: var(--color-active);
  border-left: 3px solid var(--color-primary);
}

.workflow-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
}

.workflow-info {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}

.workflow-details {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.workflow-title {
  font-weight: 500;
  color: var(--color-text);
  font-size: 0.9rem;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workflow-subtitle {
  font-size: 0.75rem;
  color: var(--color-text-light-2);
  line-height: 1.2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-indicator { flex-shrink: 0; }

/* Status dot colours. The SVG is injected via v-html (not a Vue template node)
   so scoped CSS cannot target it directly. Instead set `color` on the parent
   span — the global .svg rule has `fill:currentColor`, so the fill inherits
   through normal CSS cascade from the span's `color` value. */
.status-inactive { color: var(--color-text-light-2); }
.status-active   { color: var(--color-green); }
.status-disabled { color: var(--color-red); }
</style>
