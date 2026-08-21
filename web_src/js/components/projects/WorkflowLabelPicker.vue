<script lang="ts" setup>
import {onMounted, watch, nextTick, useTemplateRef} from 'vue';
import type {ProjectLabel} from './WorkflowStore.ts';
import {fomanticQuery} from '../../modules/fomantic/base.ts';
import {contrastColor} from '../../utils/color.ts';

const props = defineProps<{
  labels: ProjectLabel[];
  selectedIds: string[];
  placeholder: string;
  fieldLabel: string;
  readonly: boolean;
}>();

const emit = defineEmits<{
  toggle: [labelId: string];
}>();

const elDropdown = useTemplateRef<HTMLElement>('elDropdown');

const initDropdown = () => {
  if (elDropdown.value) {
    fomanticQuery(elDropdown.value).dropdown({
      action: 'nothing',
      fullTextSearch: true,
    });
  }
};

// Re-initialise when switching from read-only to edit mode.
watch(() => props.readonly, async (isReadonly) => {
  if (!isReadonly) {
    await nextTick();
    initDropdown();
  }
});

onMounted(() => {
  if (!props.readonly) initDropdown();
});

const findLabel = (labelId: string) => props.labels.find((label) => String(label.id) === labelId);

// Label colours arrive from the API without a leading "#" (services/convert
// strips it), but inline CSS and colord() both need one.
const toCssColor = (color: string | undefined): string => {
  if (!color) return '';
  return color.startsWith('#') ? color : `#${color}`;
};

const labelStyle = (color: string | undefined): string => {
  const cssColor = toCssColor(color);
  if (!cssColor) return '';
  return `background-color: ${cssColor}; color: ${contrastColor(cssColor)}`;
};

const labelStyleById = (labelId: string): string => labelStyle(findLabel(labelId)?.color);
const labelName = (labelId: string): string => findLabel(labelId)?.name ?? '';
</script>

<template>
  <!-- Edit mode: Fomantic UI multi-select dropdown -->
  <div
    v-if="!readonly"
    ref="elDropdown"
    class="ui fluid multiple search selection dropdown custom label-dropdown"
    role="listbox"
    aria-multiselectable="true"
    :aria-label="fieldLabel"
    tabindex="0"
  >
    <input type="hidden" :value="selectedIds.join(',')">
    <i class="dropdown icon"/>
    <div class="text" :class="{ default: !selectedIds.length }">
      <span v-if="!selectedIds.length">{{ placeholder }}</span>
      <template v-else>
        <span
          v-for="id in selectedIds" :key="id"
          class="ui label"
          :style="labelStyleById(id)"
        >{{ labelName(id) }}</span>
      </template>
    </div>
    <div class="menu">
      <div
        v-for="label in labels" :key="label.id"
        class="item"
        role="option"
        :data-value="String(label.id)"
        :aria-selected="selectedIds.includes(String(label.id))"
        :class="{ active: selectedIds.includes(String(label.id)), selected: selectedIds.includes(String(label.id)) }"
        @click.prevent="emit('toggle', String(label.id))"
      >
        <span class="ui label" :style="labelStyle(label.color)">
          {{ label.name }}
        </span>
      </div>
    </div>
  </div>

  <!-- Read-only mode: plain label chips -->
  <div v-else class="ui list labels-list" :aria-label="fieldLabel">
    <span v-if="!selectedIds.length" class="text-muted">{{ placeholder }}</span>
    <span
      v-for="id in selectedIds" :key="id"
      class="ui label"
      :style="labelStyleById(id)"
    >{{ labelName(id) }}</span>
  </div>
</template>

<style scoped>
.label-dropdown.ui.dropdown .menu > .item.active,
.label-dropdown.ui.dropdown .menu > .item.selected {
  background: var(--color-active);
  font-weight: var(--font-weight-normal);
}

.label-dropdown.ui.dropdown .menu > .item .ui.label {
  margin: 0;
}

.label-dropdown.ui.dropdown > .text > .ui.label {
  margin: 0.125rem;
}

.text-muted {
  color: var(--color-text-light-2);
}
</style>
