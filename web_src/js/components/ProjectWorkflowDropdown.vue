<script lang="ts" setup>
import {computed, useId} from 'vue';
import SvgIcon from './SvgIcon.vue';
import {closestDropdown} from '../modules/dropdown.ts';
import type {VNode} from 'vue';

type Value = string | number;
type Option = {value: Value; text: string; html?: string};

const props = defineProps<{
  label: string;
  options: Option[];
  readonly: boolean;
  placeholder?: string;
  filterText?: string;
  clearText?: string;
}>();

const model = defineModel<Value | Value[]>({required: true});

const labelId = useId();
const multiple = computed(() => Array.isArray(model.value));
const isSelected = (option: Option) => Array.isArray(model.value) ? model.value.includes(option.value) : model.value === option.value;
const selected = computed(() => props.options.filter(isSelected));

function choose(value: Value) {
  const values = model.value;
  if (Array.isArray(values)) {
    model.value = values.includes(value) ? values.filter((selectedValue) => selectedValue !== value) : [...values, value];
  } else {
    model.value = value;
  }
}

function refreshPopup({el}: VNode) {
  const dropdown = closestDropdown(el as HTMLElement);
  if (dropdown?.container.hasAttribute('data-shown')) dropdown.updateState();
}
</script>

<template>
  <div class="field">
    <label :id="labelId">{{ label }}</label>
    <div v-if="readonly" class="flex-text-block tw-flex-wrap" role="group" :aria-labelledby="labelId">
      <template v-if="!multiple">{{ selected[0]!.text }}</template>
      <span v-else-if="!selected.length" class="tw-text-text-light-2">{{ placeholder }}</span>
      <template v-else>
        <!-- eslint-disable-next-line vue/no-v-html -->
        <span v-for="option in selected" :key="String(option.value)" v-html="option.html"/>
      </template>
    </div>
    <div v-else class="dropdown-container" :data-keep-open="multiple ? '' : null">
      <button type="button" class="dropdown-trigger dropdown-field">
        <span class="dropdown-text" :class="{'tw-text-text-light-2': multiple && !selected.length}">
          <template v-if="!multiple">{{ selected[0]!.text }}</template>
          <template v-else-if="!selected.length">{{ placeholder }}</template>
          <span v-else class="flex-text-inline tw-flex-wrap">
            <!-- eslint-disable-next-line vue/no-v-html -->
            <span v-for="option in selected" :key="String(option.value)" v-html="option.html"/>
          </span>
        </span>
        <SvgIcon name="octicon-triangle-down" :size="14"/>
      </button>
      <div class="dropdown-popup dropdown-popup-flex" @vue:updated="refreshPopup">
        <div v-if="multiple" class="ui small fluid action input">
          <input class="dropdown-filter" type="text" :placeholder="filterText" :aria-label="filterText">
          <button type="button" class="ui basic button" @click="model = []">{{ clearText }}</button>
        </div>
        <button
          v-for="option in options" :key="option.value"
          type="button" class="dropdown-item" :role="multiple ? 'menuitemcheckbox' : 'menuitemradio'"
          :aria-checked="isSelected(option)" @click="choose(option.value)"
        >
          <SvgIcon name="octicon-check" :class="{'tw-invisible': !isSelected(option)}"/>
          <!-- eslint-disable-next-line vue/no-v-html -->
          <span v-if="multiple" v-html="option.html"/>
          <template v-else>{{ option.text }}</template>
        </button>
      </div>
    </div>
  </div>
</template>
