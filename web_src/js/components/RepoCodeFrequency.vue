<script lang="ts" setup>
import SvgIcon from './SvgIcon.vue';
import {
  Chart,
  Legend,
  PointElement,
  LineElement,
  Filler,
  type ChartOptions,
  type ChartData,
} from 'chart.js';
import {GET} from '../modules/fetch.ts';
import ChartCanvas from './ChartCanvas.vue';
import {
  startDaysBetween,
  firstStartDateAfterDate,
  fillEmptyStartDaysWithZeroes,
  type DayData,
  type DayDataObject,
} from '../utils/time.ts';
import {chartJsColors} from '../utils/color.ts';
import {errorMessage} from '../modules/errors.ts';
import {sleep} from '../utils.ts';
import {computed, onMounted, shallowRef} from 'vue';

const {pageData} = window.config;

Chart.register(
  Legend,
  PointElement,
  LineElement,
  Filler,
);

defineProps<{
  locale: {
    loadingTitle: string;
    loadingTitleFailed: string;
    loadingInfo: string;
  };
}>();

const isLoading = shallowRef(false);
const errorText = shallowRef('');
const repoLink = pageData.repoLink!;
const data = shallowRef<DayData[]>([]);

onMounted(() => {
  fetchGraphData();
});

async function fetchGraphData() {
  isLoading.value = true;
  try {
    let response: Response;
    do {
      response = await GET(`${repoLink}/activity/code-frequency/data`);
      if (response.status === 202) {
        await sleep(1000); // wait for 1 second before retrying
      }
    } while (response.status === 202);
    if (response.ok) {
      const dayDataObject: DayDataObject = await response.json();
      const weekValues = Object.values(dayDataObject);
      const start = weekValues[0].week;
      const end = firstStartDateAfterDate(new Date());
      const startDays = startDaysBetween(start, end);
      data.value = fillEmptyStartDaysWithZeroes(startDays, dayDataObject);
      errorText.value = '';
    } else {
      errorText.value = response.statusText;
    }
  } catch (err) {
    errorText.value = errorMessage(err);
  } finally {
    isLoading.value = false;
  }
}

const graphData = computed(() => toGraphData(data.value));

function toGraphData(data: DayData[]): ChartData<'line'> {
  return {
    datasets: [
      {
        data: data.map((i) => ({x: i.week, y: i.additions})),
        pointRadius: 0,
        pointHitRadius: 0,
        fill: true,
        label: 'Additions',
        backgroundColor: chartJsColors['additions'],
        borderWidth: 0,
        tension: 0.3,
      },
      {
        data: data.map((i) => ({x: i.week, y: -i.deletions})),
        pointRadius: 0,
        pointHitRadius: 0,
        fill: true,
        label: 'Deletions',
        backgroundColor: chartJsColors['deletions'],
        borderWidth: 0,
        tension: 0.3,
      },
    ],
  };
}

const options: ChartOptions<'line'> = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      display: true,
    },
  },
  scales: {
    x: {
      type: 'time',
      grid: {
        display: false,
      },
      time: {
        minUnit: 'month',
      },
      ticks: {
        maxRotation: 0,
        maxTicksLimit: 12,
      },
    },
    y: {
      ticks: {
        maxTicksLimit: 6,
      },
    },
  },
};
</script>

<template>
  <div>
    <div class="ui header">
      {{ isLoading ? locale.loadingTitle : errorText ? locale.loadingTitleFailed: `Code frequency over the history of ${repoLink.slice(1)}` }}
    </div>
    <div class="tw-flex ui segment main-graph">
      <div v-if="isLoading || errorText !== ''" class="tw-m-auto">
        <div v-if="isLoading">
          <SvgIcon name="gitea-running" class="tw-mr-2 rotate-clockwise"/>
          {{ locale.loadingInfo }}
        </div>
        <div v-else class="tw-text-red">
          <SvgIcon name="octicon-x-circle-fill"/>
          {{ errorText }}
        </div>
      </div>
      <ChartCanvas
        v-if="data.length !== 0"
        type="line" :data="graphData" :options="options"
      />
    </div>
  </div>
</template>

<style scoped>
.main-graph {
  height: 440px;
}
</style>
