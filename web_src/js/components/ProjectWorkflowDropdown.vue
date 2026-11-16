<script lang="ts" setup>
import {computed, shallowRef, useId, useTemplateRef, watch} from 'vue';
import SvgIcon from './SvgIcon.vue';
import {createTippy} from '../modules/tippy.ts';

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
const valueId = useId();
const multiple = computed(() => Array.isArray(model.value));
const isSelected = (option: Option) => multiple.value ? (model.value as Value[]).includes(option.value) : model.value === option.value;
const selected = computed(() => props.options.filter(isSelected));
const filter = shallowRef('');
const matches = (option: Option) => option.text.toLowerCase().includes(filter.value.toLowerCase());

const elTrigger = useTemplateRef<HTMLButtonElement>('elTrigger');
const elMenu = useTemplateRef<HTMLDivElement>('elMenu');
const elFilter = useTemplateRef<HTMLInputElement>('elFilter');
watch(elTrigger, (el, _oldEl, onCleanup) => {
  if (!el) return;
  const tippy = createTippy(el, {
    content: elMenu.value!,
    trigger: 'click',
    interactive: true,
    hideOnClick: true,
    placement: 'bottom-start',
    theme: 'menu',
    arrow: false,
    limitSizeToViewport: {vertical: true},
    onMount: () => elFilter.value?.focus(), // onShown only fires with animations
    onHidden: () => { filter.value = '' },
  });
  onCleanup(() => tippy.destroy());
}, {flush: 'post'});

function choose(option: Option) {
  if (!multiple.value) {
    model.value = option.value;
    elTrigger.value!._tippy.hide();
    return;
  }
  const values = model.value as Value[];
  model.value = isSelected(option) ? values.filter((v) => v !== option.value) : [...values, option.value];
}

const clear = () => { model.value = [] };
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
    <template v-else>
      <button ref="elTrigger" type="button" class="ui selection dropdown custom" :aria-labelledby="`${labelId} ${valueId}`">
        <span :id="valueId" class="text" :class="{default: multiple && !selected.length}">
          <template v-if="!multiple">{{ selected[0]!.text }}</template>
          <template v-else-if="!selected.length">{{ placeholder }}</template>
          <span v-else class="flex-text-inline tw-flex-wrap">
            <!-- eslint-disable-next-line vue/no-v-html -->
            <span v-for="option in selected" :key="String(option.value)" v-html="option.html"/>
          </span>
        </span>
        <SvgIcon name="octicon-triangle-down" :size="14" class="dropdown icon"/>
      </button>
      <!-- not .tippy-target: its "display: unset" puts the menu in an inline box, which Chromium can hit-test instead of the items -->
      <div class="tw-hidden">
        <div ref="elMenu">
          <div v-if="multiple" class="tw-px-2 tw-pt-1 tw-pb-2">
            <div class="ui small fluid icon input">
              <i class="icon"><SvgIcon name="octicon-search"/></i>
              <input
                ref="elFilter" v-model="filter" type="text" :placeholder="filterText" :aria-label="filterText"
                @keydown.space.stop @keydown.enter.stop @keydown.left.stop @keydown.right.stop
              >
            </div>
          </div>
          <div role="menu" :aria-label="label">
            <template v-if="multiple">
              <button type="button" class="item" role="menuitem" @click="clear">{{ clearText }}</button>
              <div class="divider" role="separator"/>
            </template>
            <button
              v-for="option in options" v-show="matches(option)" :key="String(option.value)"
              type="button" class="item" :role="multiple ? 'menuitemcheckbox' : 'menuitemradio'"
              :aria-checked="isSelected(option)" @click="choose(option)"
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
  </div>
</template>
