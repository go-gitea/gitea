<script lang="ts" setup>
import {onMounted, useTemplateRef, computed, watch, nextTick} from 'vue';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {contrastColor} from '../utils/color.ts';

interface Label {
  id: number | string;
  name: string;
  color: string;
  description?: string;
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
        <span
          v-for="labelId in modelValue"
          :key="labelId"
          class="ui label"
          :style="`background-color: ${labels.find(l => String(l.id) === labelId)?.color}; color: ${getLabelTextColor(labels.find(l => String(l.id) === labelId)?.color)}`"
        >
          {{ labels.find(l => String(l.id) === labelId)?.name }}
        </span>
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
        <span
          class="ui label"
          :style="`background-color: ${label.color}; color: ${getLabelTextColor(label.color)}`"
        >
          {{ label.name }}
        </span>
      </div>
    </div>
  </div>

  <!-- Readonly Mode: Display Selected Labels -->
  <div v-else class="ui labels">
    <span v-if="!selectedLabels.length" class="text-muted">None</span>
    <span
      v-for="label in selectedLabels"
      :key="label.id"
      class="ui label"
      :style="`background-color: ${label.color}; color: ${getLabelTextColor(label.color)}`"
    >
      {{ label.name }}
    </span>
  </div>
</template>

<style scoped>
/* Label selector styles */
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

.ui.labels {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
}

.ui.labels .ui.label {
  margin: 0;
}

.text-muted {
  color: var(--color-text-light-2);
}
</style>
