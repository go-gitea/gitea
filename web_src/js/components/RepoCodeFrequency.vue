<script>
import {SvgIcon} from '../svg.js';
import {
  Chart,
  Legend,
  LinearScale,
  TimeScale,
  PointElement,
  LineElement,
  Filler,
} from 'chart.js';
import {GET} from '../modules/fetch.js';
import {Line as ChartLine} from 'vue-chartjs';
import {
  startDaysBetween,
  firstStartDateAfterDate,
  fillEmptyStartDaysWithZeroes,
} from '../utils/time.js';
import {chartJsColors} from '../utils/color.js';
import {sleep} from '../utils.js';
import 'chartjs-adapter-dayjs-4/dist/chartjs-adapter-dayjs-4.esm';

const {pageData} = window.config;

Chart.defaults.color = chartJsColors.text;
Chart.defaults.borderColor = chartJsColors.border;

Chart.register(
  TimeScale,
  LinearScale,
  Legend,
  PointElement,
  LineElement,
  Filler,
);

export default {
  components: {ChartLine, SvgIcon},
  props: {
    locale: {
      type: Object,
      required: true,
    },
  },
  data: () => ({
    isLoading: false,
    errorText: '',
    repoLink: pageData.repoLink || [],
    data: [],
  }),
  mounted() {
    this.fetchGraphData();
  },
  methods: {
    async fetchGraphData() {
      this.isLoading = true;
      try {
        let response;
        do {
          response = await GET(`${this.repoLink}/activity/code-frequency/data`);
          if (response.status === 202) {
            await sleep(1000); // wait for 1 second before retrying
          }
        } while (response.status === 202);
        if (response.ok) {
          this.data = await response.json();
          const weekValues = Object.values(this.data);
          const start = weekValues[0].week;
          const end = firstStartDateAfterDate(new Date());
          const startDays = startDaysBetween(start, end);
          this.data = fillEmptyStartDaysWithZeroes(startDays, this.data);
          this.errorText = '';
        } else {
          this.errorText = response.statusText;
        }
      } catch (err) {
        this.errorText = err.message;
      } finally {
        this.isLoading = false;
      }
    },

    toGraphData(data) {
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
    },

    getOptions() {
      return {
        responsive: true,
        maintainAspectRatio: false,
        animation: true,
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
    },
  },
};
</script>
<template>
  <div>
    <div class="ui header tw-flex tw-items-center tw-justify-between">
      {{ isLoading ? locale.loadingTitle : errorText ? locale.loadingTitleFailed: `Code frequency over the history of ${repoLink.slice(1)}` }}
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
      <ChartLine
        v-memo="data" v-if="data.length !== 0"
        :data="toGraphData(data)" :options="getOptions()"
      />
    </div>
  </div>
</template>
<style scoped>
.main-graph {
  height: 440px;
}
</style>
