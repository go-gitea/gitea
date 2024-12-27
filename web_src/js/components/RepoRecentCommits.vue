<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import {
  Chart,
  Tooltip,
  BarElement,
  LinearScale,
  TimeScale,
  type ChartOptions,
  type ChartData,
} from 'chart.js';
import {GET} from '../modules/fetch.ts';
import {Bar} from 'vue-chartjs';
import {
  startDaysBetween,
  firstStartDateAfterDate,
  fillEmptyStartDaysWithZeroes,
  type DayData,
  type DayDataObject,
} from '../utils/time.ts';
import {chartJsColors} from '../utils/color.ts';
import {sleep} from '../utils.ts';
import 'chartjs-adapter-dayjs-4/dist/chartjs-adapter-dayjs-4.esm';
import {onMounted, ref} from 'vue';

const {pageData} = window.config;

Chart.defaults.color = chartJsColors.text;
Chart.defaults.borderColor = chartJsColors.border;

Chart.register(
  TimeScale,
  LinearScale,
  BarElement,
  Tooltip,
);

defineProps<{
  locale: {
    loadingTitle: string;
    loadingTitleFailed: string;
    loadingInfo: string;
  };
}>();

const isLoading = ref(false);
const errorText = ref('');
const repoLink = ref(pageData.repoLink || []);
const data = ref<DayData[]>([]);

onMounted(() => {
  fetchGraphData();
});

async function fetchGraphData() {
  isLoading.value = true;
  try {
    let response: Response;
    do {
      response = await GET(`${repoLink.value}/activity/recent-commits/data`);
      if (response.status === 202) {
        await sleep(1000); // wait for 1 second before retrying
      }
    } while (response.status === 202);
    if (response.ok) {
      const dayDataObj: DayDataObject = await response.json();
      const start = Object.values(dayDataObj)[0].week;
      const end = firstStartDateAfterDate(new Date());
      const startDays = startDaysBetween(start, end);
      data.value = fillEmptyStartDaysWithZeroes(startDays, dayDataObj).slice(-52);
      errorText.value = '';
    } else {
      errorText.value = response.statusText;
    }
  } catch (err) {
    errorText.value = err.message;
  } finally {
    isLoading.value = false;
  }
}

function toGraphData(data: DayData[]): ChartData<'bar'> {
  return {
    datasets: [
      {
        // @ts-expect-error -- bar chart expects one-dimensional data, but apparently x/y still works
        data: data.map((i) => ({x: i.week, y: i.commits})),
        label: 'Commits',
        backgroundColor: chartJsColors['commits'],
        borderWidth: 0,
        tension: 0.3,
      },
    ],
  };
}

const options: ChartOptions<'bar'> = {
  responsive: true,
  maintainAspectRatio: false,
  scales: {
    x: {
      type: 'time',
      grid: {
        display: false,
      },
      time: {
        minUnit: 'week',
      },
      ticks: {
        maxRotation: 0,
        maxTicksLimit: 52,
      },
    },
    y: {
      ticks: {
        maxTicksLimit: 6,
      },
    },
  },
} satisfies ChartOptions;
</script>

<template>
  <div>
    <div class="ui header tw-flex tw-items-center tw-justify-between">
      {{ isLoading ? locale.loadingTitle : errorText ? locale.loadingTitleFailed: "Number of commits in the past year" }}
    </div>
    <div class="tw-flex ui segment main-graph">
      <div v-if="isLoading || errorText !== ''" class="gt-tc tw-m-auto">
        <div v-if="isLoading">
          <SvgIcon name="octicon-sync" class="tw-mr-2 job-status-rotate"/>
          {{ locale.loadingInfo }}
        </div>
        <div v-else class="text red">
          <SvgIcon name="octicon-x-circle-fill"/>
          {{ errorText }}
        </div>
      </div>
      <Bar
        v-memo="data" v-if="data.length !== 0"
        :data="toGraphData(data)" :options="options"
      />
    </div>
  </div>
</template>
<style scoped>
.main-graph {
  height: 250px;
}
</style>
