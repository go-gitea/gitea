<script lang="ts" setup>
import {onMounted, useTemplateRef, computed, watch, nextTick} from 'vue';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {contrastColor} from '../utils/color.ts';

interface Label {
  id: number | string;
  name: string;
  color: string;
  description?: string;
  exclusive?: boolean;
  exclusiveOrder?: number;
}

const props = withDefaults(defineProps<{
  modelValue: string[];
  labels: Label[];
  placeholder?: string;
  readonly?: boolean;
  multiple?: boolean;
}>(), {
  placeholder: 'Select labels...',
  readonly: false,
  multiple: true,
});

const emit = defineEmits<{
  'update:modelValue': [value: string[]];
}>();

const elDropdown = useTemplateRef('elDropdown');

// Get selected labels for display
const selectedLabels = computed(() => {
  return props.labels.filter((label) =>
    props.modelValue.includes(String(label.id)),
  );
});

// Get contrast color for label text
const getLabelTextColor = (hexColor: string) => {
  return contrastColor(hexColor);
};

// Convert hex color to RGB
const hexToRGB = (hex: string): {r: number; g: number; b: number} => {
  const color = hex.replace(/^#/, '');
  return {
    r: Number.parseInt(color.substring(0, 2), 16),
    g: Number.parseInt(color.substring(2, 4), 16),
    b: Number.parseInt(color.substring(4, 6), 16),
  };
};

// Get relative luminance of a color
const getRelativeLuminance = (hex: string): number => {
  const {r, g, b} = hexToRGB(hex);
  const rsRGB = r / 255;
  const gsRGB = g / 255;
  const bsRGB = b / 255;

  const rLinear = rsRGB <= 0.03928 ? rsRGB / 12.92 : Math.pow((rsRGB + 0.055) / 1.055, 2.4);
  const gLinear = gsRGB <= 0.03928 ? gsRGB / 12.92 : Math.pow((gsRGB + 0.055) / 1.055, 2.4);
  const bLinear = bsRGB <= 0.03928 ? bsRGB / 12.92 : Math.pow((bsRGB + 0.055) / 1.055, 2.4);

  return 0.2126 * rLinear + 0.7152 * gLinear + 0.0722 * bLinear;
};

// Get scope and item colors for exclusive labels
const getScopeColors = (baseColor: string): {scopeColor: string; itemColor: string} => {
  const luminance = getRelativeLuminance(baseColor);
  const contrast = 0.01 + luminance * 0.03;
  const darken = contrast + Math.max(luminance + contrast - 1.0, 0.0);
  const lighten = contrast + Math.max(contrast - luminance, 0.0);
  const darkenFactor = Math.max(luminance - darken, 0.0) / Math.max(luminance, 1.0 / 255.0);
  const lightenFactor = Math.min(luminance + lighten, 1.0) / Math.max(luminance, 1.0 / 255.0);

  const {r, g, b} = hexToRGB(baseColor);

  const scopeR = Math.min(Math.round(r * darkenFactor), 255);
  const scopeG = Math.min(Math.round(g * darkenFactor), 255);
  const scopeB = Math.min(Math.round(b * darkenFactor), 255);

  const itemR = Math.min(Math.round(r * lightenFactor), 255);
  const itemG = Math.min(Math.round(g * lightenFactor), 255);
  const itemB = Math.min(Math.round(b * lightenFactor), 255);

  const scopeColor = `#${scopeR.toString(16).padStart(2, '0')}${scopeG.toString(16).padStart(2, '0')}${scopeB.toString(16).padStart(2, '0')}`;
  const itemColor = `#${itemR.toString(16).padStart(2, '0')}${itemG.toString(16).padStart(2, '0')}${itemB.toString(16).padStart(2, '0')}`;

  return {scopeColor, itemColor};
};

// Get exclusive scope from label name
const getExclusiveScope = (label: Label): string => {
  if (!label.exclusive) return '';
  const lastIndex = label.name.lastIndexOf('/');
  if (lastIndex === -1 || lastIndex === 0 || lastIndex === label.name.length - 1) {
    return '';
  }
  return label.name.substring(0, lastIndex);
};

// Get label scope part (before the '/')
const getLabelScope = (label: Label): string => {
  const scope = getExclusiveScope(label);
  return scope || '';
};

// Get label item part (after the '/')
const getLabelItem = (label: Label): string => {
  const scope = getExclusiveScope(label);
  if (!scope) return label.name;
  return label.name.substring(scope.length + 1);
};

// Toggle label selection
const toggleLabel = (labelId: string) => {
  if (props.readonly) return;

  const currentValues = [...props.modelValue];
  const index = currentValues.indexOf(labelId);

  if (index > -1) {
    currentValues.splice(index, 1);
  } else {
    if (props.multiple) {
      currentValues.push(labelId);
    } else {
      // Single selection mode: replace with new selection
      currentValues.length = 0;
      currentValues.push(labelId);
    }
  }

  emit('update:modelValue', currentValues);
};

// Check if a label is selected
const isLabelSelected = (labelId: string) => {
  return props.modelValue.includes(labelId);
};

// Initialize Fomantic UI dropdown
const initDropdown = async () => {
  if (props.readonly || !elDropdown.value) return;

  await nextTick();
  fomanticQuery(elDropdown.value).dropdown({
    action: 'nothing', // Don't hide on selection for multiple selection
    fullTextSearch: true,
  });
};

// Watch for readonly changes to reinitialize
watch(() => props.readonly, async (newVal) => {
  if (!newVal) {
    await initDropdown();
  }
});

onMounted(async () => {
  await initDropdown();
});
</script>

<template>
  <!-- Edit Mode: Dropdown -->
  <div v-if="!readonly" ref="elDropdown" class="ui fluid multiple search selection dropdown label-dropdown">
    <input type="hidden" :value="modelValue.join(',')">
    <i class="dropdown icon"/>
    <div class="text" :class="{ default: !modelValue.length }">
      <span v-if="!modelValue.length">{{ placeholder }}</span>
      <template v-else>
        <template v-for="labelId in modelValue" :key="labelId">
          <template v-if="labels.find(l => String(l.id) === labelId)">
            <!-- Regular label (no exclusive scope) -->
            <span
              v-if="!labels.find(l => String(l.id) === labelId).exclusive || !getLabelScope(labels.find(l => String(l.id) === labelId))"
              class="ui label"
              :style="`background-color: ${labels.find(l => String(l.id) === labelId).color}; color: ${getLabelTextColor(labels.find(l => String(l.id) === labelId).color)}`"
            >
              {{ labels.find(l => String(l.id) === labelId).name }}
            </span>
            <!-- Exclusive label with order: scope | item | order -->
            <span
              v-else-if="labels.find(l => String(l.id) === labelId).exclusiveOrder && labels.find(l => String(l.id) === labelId).exclusiveOrder > 0"
              class="ui label scope-parent"
            >
              <div class="ui label scope-left" :style="`color: ${getLabelTextColor(labels.find(l => String(l.id) === labelId).color)} !important; background-color: ${getScopeColors(labels.find(l => String(l.id) === labelId).color).scopeColor} !important`">
                {{ getLabelScope(labels.find(l => String(l.id) === labelId)) }}
              </div>
              <div class="ui label scope-middle" :style="`color: ${getLabelTextColor(labels.find(l => String(l.id) === labelId).color)} !important; background-color: ${getScopeColors(labels.find(l => String(l.id) === labelId).color).itemColor} !important`">
                {{ getLabelItem(labels.find(l => String(l.id) === labelId)) }}
              </div>
              <div class="ui label scope-right">
                {{ labels.find(l => String(l.id) === labelId).exclusiveOrder }}
              </div>
            </span>
            <!-- Exclusive label without order: scope | item -->
            <span
              v-else
              class="ui label scope-parent"
            >
              <div class="ui label scope-left" :style="`color: ${getLabelTextColor(labels.find(l => String(l.id) === labelId).color)} !important; background-color: ${getScopeColors(labels.find(l => String(l.id) === labelId).color).scopeColor} !important`">
                {{ getLabelScope(labels.find(l => String(l.id) === labelId)) }}
              </div>
              <div class="ui label scope-right" :style="`color: ${getLabelTextColor(labels.find(l => String(l.id) === labelId).color)} !important; background-color: ${getScopeColors(labels.find(l => String(l.id) === labelId).color).itemColor} !important`">
                {{ getLabelItem(labels.find(l => String(l.id) === labelId)) }}
              </div>
            </span>
          </template>
        </template>
      </template>
    </div>
    <div class="menu">
      <div
        v-for="label in labels"
        :key="label.id"
        class="item"
        :data-value="String(label.id)"
        :class="{ active: isLabelSelected(String(label.id)), selected: isLabelSelected(String(label.id)) }"
        @click.prevent="toggleLabel(String(label.id))"
      >
        <!-- Regular label (no exclusive scope) -->
        <span
          v-if="!label.exclusive || !getLabelScope(label)"
          class="ui label"
          :style="`background-color: ${label.color}; color: ${getLabelTextColor(label.color)}`"
        >
          {{ label.name }}
        </span>
        <!-- Exclusive label with order: scope | item | order -->
        <span
          v-else-if="label.exclusiveOrder && label.exclusiveOrder > 0"
          class="ui label scope-parent"
          :title="label.description"
        >
          <div class="ui label scope-left" :style="`color: ${getLabelTextColor(label.color)} !important; background-color: ${getScopeColors(label.color).scopeColor} !important`">
            {{ getLabelScope(label) }}
          </div>
          <div class="ui label scope-middle" :style="`color: ${getLabelTextColor(label.color)} !important; background-color: ${getScopeColors(label.color).itemColor} !important`">
            {{ getLabelItem(label) }}
          </div>
          <div class="ui label scope-right">
            {{ label.exclusiveOrder }}
          </div>
        </span>
        <!-- Exclusive label without order: scope | item -->
        <span
          v-else
          class="ui label scope-parent"
          :title="label.description"
        >
          <div class="ui label scope-left" :style="`color: ${getLabelTextColor(label.color)} !important; background-color: ${getScopeColors(label.color).scopeColor} !important`">
            {{ getLabelScope(label) }}
          </div>
          <div class="ui label scope-right" :style="`color: ${getLabelTextColor(label.color)} !important; background-color: ${getScopeColors(label.color).itemColor} !important`">
            {{ getLabelItem(label) }}
          </div>
        </span>
      </div>
    </div>
  </div>

  <!-- Readonly Mode: Display Selected Labels -->
  <div v-else class="ui labels">
    <span v-if="!selectedLabels.length" class="text-muted">None</span>
    <template v-for="label in selectedLabels" :key="label.id">
      <!-- Regular label (no exclusive scope) -->
      <span
        v-if="!label.exclusive || !getLabelScope(label)"
        class="ui label"
        :style="`background-color: ${label.color}; color: ${getLabelTextColor(label.color)}`"
      >
        {{ label.name }}
      </span>
      <!-- Exclusive label with order: scope | item | order -->
      <span
        v-else-if="label.exclusiveOrder && label.exclusiveOrder > 0"
        class="ui label scope-parent"
        :title="label.description"
      >
        <div class="ui label scope-left" :style="`color: ${getLabelTextColor(label.color)} !important; background-color: ${getScopeColors(label.color).scopeColor} !important`">
          {{ getLabelScope(label) }}
        </div>
        <div class="ui label scope-middle" :style="`color: ${getLabelTextColor(label.color)} !important; background-color: ${getScopeColors(label.color).itemColor} !important`">
          {{ getLabelItem(label) }}
        </div>
        <div class="ui label scope-right">
          {{ label.exclusiveOrder }}
        </div>
      </span>
      <!-- Exclusive label without order: scope | item -->
      <span
        v-else
        class="ui label scope-parent"
        :title="label.description"
      >
        <div class="ui label scope-left" :style="`color: ${getLabelTextColor(label.color)} !important; background-color: ${getScopeColors(label.color).scopeColor} !important`">
          {{ getLabelScope(label) }}
        </div>
        <div class="ui label scope-right" :style="`color: ${getLabelTextColor(label.color)} !important; background-color: ${getScopeColors(label.color).itemColor} !important`">
          {{ getLabelItem(label) }}
        </div>
      </span>
    </template>
  </div>
</template>

<style>
/* Label selector specific styles - not scoped to allow global label.css to work */
.label-dropdown.ui.dropdown .menu > .item.active,
.label-dropdown.ui.dropdown .menu > .item.selected {
  background: var(--color-active);
  font-weight: var(--font-weight-normal);
}

.label-dropdown.ui.dropdown .menu > .item .ui.label {
  margin: 0;
}

.label-dropdown.ui.dropdown > .text > .ui.label,
.label-dropdown.ui.dropdown > .text > .ui.label.scope-parent {
  margin: 0.125rem;
}

.text-muted {
  color: var(--color-text-light-2);
}
</style>
