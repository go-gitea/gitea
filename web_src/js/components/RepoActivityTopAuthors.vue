<script lang="ts" setup>
import {onMounted, shallowRef, useTemplateRef, type ShallowRef} from 'vue';

const barSlotWidth = 40; // horizontal space allotted per author
const chartHeight = 100;
const innerChartHeight = chartHeight - 28; // 28 = avatar/x-axis label row (20) + 8px padding
const barMidPoint = barSlotWidth / 2;
const barWidth = barSlotWidth - 2; // 2px gap between bars
const avatarSize = 20;
const labelInsideThreshold = 22; // bars at least this tall carry the commit count inside them

const colors = shallowRef({
  barColor: 'green',
  textColor: 'black',
  textAltColor: 'white',
});

type ActivityAuthorData = {
  avatar_link: string;
  commits: number;
  home_link: string;
  login: string;
  name: string;
}

const activityTopAuthors: Array<ActivityAuthorData> = window.config.pageData.repoActivityTopAuthors || [];

const graphWidth = activityTopAuthors.length * barSlotWidth;
const maxCommits = Math.max(...activityTopAuthors.map((author) => author.commits));

const bars = activityTopAuthors.map((author, index) => {
  const height = author.commits / maxCommits * innerChartHeight;
  return {
    author,
    index,
    x: index * barSlotWidth,
    height,
    yOffset: innerChartHeight - height,
    labelInside: height >= labelInsideThreshold,
  };
});

const styleElement = useTemplateRef('styleElement') as Readonly<ShallowRef<HTMLDivElement>>;
const altStyleElement = useTemplateRef('altStyleElement') as Readonly<ShallowRef<HTMLDivElement>>;

onMounted(() => {
  const refStyle = window.getComputedStyle(styleElement.value);
  const refAltStyle = window.getComputedStyle(altStyleElement.value);

  colors.value = {
    barColor: refStyle.backgroundColor,
    textColor: refStyle.color,
    textAltColor: refAltStyle.color,
  };
});
</script>

<template>
  <div>
    <div class="activity-bar-graph tw-w-0 tw-h-0" ref="styleElement"/>
    <div class="activity-bar-graph-alt tw-w-0 tw-h-0" ref="altStyleElement"/>
    <svg :width="graphWidth" :height="chartHeight">
      <g v-for="bar in bars" :key="bar.index" :transform="`translate(${bar.x},0)`">
        <title>{{ bar.author.name }}</title>
        <rect :width="barWidth" :height="bar.height" :x="2" :y="bar.yOffset" :style="{fill: colors.barColor}"/>
        <text
          :x="barMidPoint"
          :y="bar.yOffset"
          :dy="bar.labelInside ? '15px' : '-5px'"
          text-anchor="middle"
          :style="{fill: bar.labelInside ? colors.textAltColor : colors.textColor, font: '10px sans-serif'}"
        >{{ bar.author.commits }}</text>
        <a v-if="bar.author.home_link" :href="bar.author.home_link">
          <image :x="barMidPoint - avatarSize / 2" :y="innerChartHeight + 4" :height="avatarSize" :width="avatarSize" :href="bar.author.avatar_link"/>
        </a>
        <image v-else :x="barMidPoint - avatarSize / 2" :y="innerChartHeight + 4" :height="avatarSize" :width="avatarSize" :href="bar.author.avatar_link"/>
        <line class="axis-line" :x1="barMidPoint" :x2="barMidPoint" :y1="innerChartHeight + 3" :y2="innerChartHeight"/>
      </g>
      <line class="axis-line" :x1="2" :x2="graphWidth" :y1="innerChartHeight" :y2="innerChartHeight"/>
    </svg>
  </div>
</template>

<style scoped>
svg {
  display: block; /* avoid the inline-baseline gap so the reserved container height matches exactly */
}
.axis-line {
  stroke: var(--color-secondary-alpha-60);
  stroke-width: 1;
}
</style>
