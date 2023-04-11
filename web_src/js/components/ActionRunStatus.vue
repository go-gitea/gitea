<template>
  <SvgIcon name="octicon-check-circle-fill" class="ui text green" :size="size" :class-name="className" v-if="status === 'success'"/>
  <SvgIcon name="octicon-skip" class="ui text grey" :size="size" :class-name="className" v-else-if="status === 'skipped'"/>
  <SvgIcon name="octicon-clock" class="ui text yellow" :size="size" :class-name="className" v-else-if="status === 'waiting'"/>
  <SvgIcon name="octicon-blocked" class="ui text yellow" :size="size" :class-name="className" v-else-if="status === 'blocked'"/>
  <SvgIcon name="octicon-meter" class="ui text yellow" :size="size" :class-name="'job-status-rotate ' + className" v-else-if="status === 'running'"/>
  <SvgIcon name="octicon-x-circle-fill" class="ui text red" :size="size" v-else/>
</template>

<script>
import {SvgIcon} from '../svg.js';
import {createApp} from 'vue';

const sfc = {
  components: {SvgIcon},
  props: {
    status: {
      type: String,
      required: true
    },
    size: {
      type: Number,
      default: 16
    },
    className: {
      type: String,
      default: ''
    }
  },
};

export default sfc;

export function initActionRunStatus() {
  const els = document.getElementsByClassName('action-run-status');
  if (!els) return;
  for (const el of els) {
    const view = createApp(sfc, {
      status: el.getAttribute('data-status'),
      size: el.getAttribute('data-size'),
      className: el.getAttribute('data-class-name'),
    });
    view.mount(el);
  }
}
</script>
