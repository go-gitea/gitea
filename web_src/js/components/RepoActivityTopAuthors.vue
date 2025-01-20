<script lang="ts" setup>
// @ts-expect-error - module exports no types
import {VueBarGraph} from 'vue-bar-graph';
import {computed, onMounted, ref} from 'vue';

const colors = ref({
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

const graphPoints = computed(() => {
  return activityTopAuthors.map((item) => {
    return {
      value: item.commits,
      label: item.name,
    };
  });
});

const graphAuthors = computed(() => {
  return activityTopAuthors.map((item, idx: number) => {
    return {
      position: idx + 1,
      ...item,
    };
  });
});

const graphWidth = computed(() => {
  return activityTopAuthors.length * 40;
});

const styleElement = ref<HTMLElement | null>(null);
const altStyleElement = ref<HTMLElement | null>(null);

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
    <div class="activity-bar-graph" ref="styleElement" style="width: 0; height: 0;"/>
    <div class="activity-bar-graph-alt" ref="altStyleElement" style="width: 0; height: 0;"/>
    <vue-bar-graph
      :points="graphPoints"
      :show-x-axis="true"
      :show-y-axis="false"
      :show-values="true"
      :width="graphWidth"
      :bar-color="colors.barColor"
      :text-color="colors.textColor"
      :text-alt-color="colors.textAltColor"
      :height="100"
      :label-height="20"
    >
      <template #label="opt">
        <g v-for="(author, idx) in graphAuthors" :key="author.position">
          <a
            v-if="opt.bar.index === idx && author.home_link"
            :href="author.home_link"
          >
            <image
              :x="`${opt.bar.midPoint - 10}px`"
              :y="`${opt.bar.yLabel}px`"
              height="20"
              width="20"
              :href="author.avatar_link"
            />
          </a>
          <image
            v-else-if="opt.bar.index === idx"
            :x="`${opt.bar.midPoint - 10}px`"
            :y="`${opt.bar.yLabel}px`"
            height="20"
            width="20"
            :href="author.avatar_link"
          />
        </g>
      </template>
      <template #title="opt">
        <tspan v-for="(author, idx) in graphAuthors" :key="author.position">
          <tspan v-if="opt.bar.index === idx">
            {{ author.name }}
          </tspan>
        </tspan>
      </template>
    </vue-bar-graph>
  </div>
</template>
