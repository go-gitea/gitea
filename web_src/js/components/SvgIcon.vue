<script lang="ts" setup>
import {computed} from 'vue';
import {svgParseOuterInner, type SvgName} from '../svg.ts';
import {html, htmlRaw} from '../utils/html.ts';

const props = withDefaults(defineProps<{
  name?: SvgName,
  useHref?: string, // render svg body as `<use href>`
  size?: number,
  symbolId?: string,
}>(), {
  name: undefined,
  useHref: undefined,
  size: 16,
  symbolId: undefined,
});

const icon = computed(() => {
  if (props.useHref) {
    const attrs = {width: props.size, height: props.size, 'aria-hidden': 'true', innerHTML: html`<use href="${props.useHref}"></use>`};
    return {attrs, classes: []};
  }
  let {svgOuter, svgInnerHtml} = svgParseOuterInner(props.name!);
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
