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
  const attrs: Record<string, string | number> = {};
  const classes: string[] = [];
  let svgInnerHtml: string;

  if (props.useHref) {
    attrs['aria-hidden'] = 'true';
    svgInnerHtml = html`<use href="${props.useHref}"></use>`;
  } else {
    const {svgOuter, svgInnerHtml: bundled} = svgParseOuterInner(props.name!);
    for (const attr of svgOuter.attributes) {
      if (attr.name !== 'class') attrs[attr.name] = attr.value;
    }
    classes.push(...svgOuter.classList);
    svgInnerHtml = bundled;
    if (props.symbolId) {
      classes.push('tw-hidden', 'svg-symbol-container');
      svgInnerHtml = html`<symbol id="${props.symbolId}" viewBox="${attrs.viewBox}">${htmlRaw(svgInnerHtml)}</symbol>`;
    }
  }

  attrs.width = props.size;
  attrs.height = props.size;
  attrs.innerHTML = svgInnerHtml; // bundled icon content, or an escaped reference to one
  return {attrs, classes};
});
</script>

<template>
  <svg v-bind="icon.attrs" :class="icon.classes"/>
</template>
