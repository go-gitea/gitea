<script lang="ts" setup>
import {computed, useTemplateRef, watch} from 'vue';
import SvgIcon from './SvgIcon.vue';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {contrastColor} from '../utils/color.ts';
import type {Label} from '../types.ts';

const props = defineProps<{
  labels: Label[];
  selectedIds: number[];
  readonly: boolean;
  placeholder: string;
  fieldLabel: string;
}>();

const emit = defineEmits<{
  toggle: [id: number];
}>();

const elDropdown = useTemplateRef<HTMLElement>('elDropdown');
watch(elDropdown, (el) => {
  if (el) fomanticQuery(el).dropdown({action: 'nothing', fullTextSearch: true});
}, {flush: 'post'});

const selectedLabels = computed(() => props.labels.filter((l) => props.selectedIds.includes(l.id)));
const labelStyle = (label: Label) => ({backgroundColor: `#${label.color}`, color: contrastColor(`#${label.color}`)});
</script>

<template>
  <div v-if="readonly" class="flex-text-block tw-flex-wrap" :aria-label="fieldLabel">
    <span v-if="!selectedIds.length" class="tw-text-text-light-2">{{ placeholder }}</span>
    <span v-for="label in selectedLabels" :key="label.id" class="ui label" :style="labelStyle(label)">{{ label.name }}</span>
  </div>
  <div v-else ref="elDropdown" class="ui fluid multiple search selection dropdown" role="listbox" aria-multiselectable="true" :aria-label="fieldLabel" tabindex="0">
    <input type="hidden" :value="selectedIds.join(',')">
    <SvgIcon name="octicon-triangle-down" :size="14" class="dropdown icon"/>
    <div class="text" :class="{default: !selectedIds.length}">
      <template v-if="!selectedIds.length">{{ placeholder }}</template>
      <span v-for="label in selectedLabels" :key="label.id" class="ui label" :style="labelStyle(label)">{{ label.name }}</span>
    </div>
    <div class="menu">
      <div
        v-for="label in labels" :key="label.id"
        class="item" role="option" :data-value="label.id"
        :class="{active: selectedIds.includes(label.id)}" :aria-selected="selectedIds.includes(label.id)"
        @click.prevent="emit('toggle', label.id)"
      >
        <span class="ui label" :style="labelStyle(label)">{{ label.name }}</span>
      </div>
    </div>
  </div>
</template>
