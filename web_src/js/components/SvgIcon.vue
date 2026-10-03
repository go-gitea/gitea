<script lang="ts" setup>
import {computed} from 'vue';
import {svgParseOuterInner, type SvgName} from '../svg.ts';
import {html, htmlRaw} from '../utils/html.ts';

const props = withDefaults(defineProps<{
  name: SvgName,
  size?: number,
  symbolId?: string,
}>(), {
  size: 16,
  symbolId: undefined,
});

const icon = computed(() => {
  let {svgOuter, svgInnerHtml} = svgParseOuterInner(props.name);
  const attrs: Record<string, string | number> = {};
  for (const attr of svgOuter.attributes) {
    if (attr.name === 'class') continue;
    attrs[attr.name] = attr.value;
  }
  attrs.width = props.size;
  attrs.height = props.size;

  const classes = Array.from(svgOuter.classList);
  if (props.symbolId) {
    classes.push('tw-hidden', 'svg-symbol-container');
    svgInnerHtml = html`<symbol id="${props.symbolId}" viewBox="${attrs.viewBox}">${htmlRaw(svgInnerHtml)}</symbol>`;
  }
  attrs.innerHTML = svgInnerHtml; // the icons are bundled, they carry no user input
  return {attrs, classes};
});
</script>

<template>
  <svg v-bind="icon.attrs" :class="icon.classes"/>
</template>
