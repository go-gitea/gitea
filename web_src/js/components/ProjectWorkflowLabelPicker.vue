<script lang="ts" setup>
import {computed, useTemplateRef, watch} from 'vue';
import SvgIcon from './SvgIcon.vue';
import {createTippy} from '../modules/tippy.ts';
import type {ProjectLabel} from './ProjectWorkflowStore.ts';

const props = defineProps<{
  labels: ProjectLabel[];
  selectedIds: number[];
  readonly: boolean;
  placeholder: string;
  fieldLabel: string;
}>();

const emit = defineEmits<{
  toggle: [id: number];
}>();

const elTrigger = useTemplateRef<HTMLButtonElement>('elTrigger');
const elMenu = useTemplateRef<HTMLDivElement>('elMenu');
watch(elTrigger, (el, _, onCleanup) => {
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
  });
  onCleanup(() => tippy.destroy());
}, {flush: 'post'});

const selectedLabels = computed(() => props.labels.filter((l) => props.selectedIds.includes(l.id)));
</script>

<template>
  <div class="flex-text-block tw-flex-wrap" role="group" :aria-label="fieldLabel">
    <span v-if="!selectedLabels.length" class="tw-text-text-light-2">{{ placeholder }}</span>
    <!-- eslint-disable-next-line vue/no-v-html -->
    <span v-for="label in selectedLabels" :key="label.id" v-html="label.html"/>
    <template v-if="!readonly">
      <button ref="elTrigger" type="button" class="ui mini basic icon button" :aria-label="fieldLabel">
        <SvgIcon name="octicon-gear"/>
      </button>
      <div ref="elMenu" class="tippy-target">
        <div role="menu" :aria-label="fieldLabel">
          <button
            v-for="label in labels" :key="label.id"
            type="button" class="item" role="menuitemcheckbox"
            :aria-checked="selectedIds.includes(label.id)" @click="emit('toggle', label.id)"
          >
            <SvgIcon name="octicon-check" :class="{'tw-invisible': !selectedIds.includes(label.id)}"/>
            <!-- eslint-disable-next-line vue/no-v-html -->
            <span v-html="label.html"/>
          </button>
        </div>
      </div>
    </template>
  </div>
</template>
