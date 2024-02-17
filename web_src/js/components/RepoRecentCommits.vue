<script>
import {SvgIcon} from '../svg.js';
import {
  Chart,
  Tooltip,
  BarElement,
  LinearScale,
  TimeScale,
} from 'chart.js';
import {GET} from '../modules/fetch.js';
import {Bar} from 'vue-chartjs';
import {
  startDaysBetween,
  firstStartDateAfterDate,
  fillEmptyStartDaysWithZeroes,
} from '../utils/time.js';
import 'chartjs-adapter-dayjs-4/dist/chartjs-adapter-dayjs-4.esm';

const {pageData} = window.config;

const colors = {
  text: '--color-text',
  border: '--color-secondary-alpha-60',
  commits: '--color-primary-alpha-60',
  additions: '--color-green',
  deletions: '--color-red',
};

const styles = window.getComputedStyle(document.documentElement);
const getColor = (name) => styles.getPropertyValue(name).trim();

for (const [key, value] of Object.entries(colors)) {
  colors[key] = getColor(value);
}

Chart.defaults.color = colors.text;
Chart.defaults.borderColor = colors.border;

Chart.register(
  TimeScale,
  LinearScale,
  BarElement,
  Tooltip,
);

export default {
  components: {Bar, SvgIcon},
  props: {
    locale: {
      type: Object,
      required: true
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
          response = await GET(`${this.repoLink}/activity/recent-commits/data`);
          if (response.status === 202) {
            await new Promise((resolve) => setTimeout(resolve, 1000)); // wait for 1 second before retrying
          }
        } while (response.status === 202);
        if (response.ok) {
          const data = await response.json();
          const start = Object.values(data)[0].week;
          const end = firstStartDateAfterDate(new Date());
          const startDays = startDaysBetween(new Date(start), new Date(end));
          this.data = fillEmptyStartDaysWithZeroes(startDays, data).slice(-52);
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
            data: data.map((i) => ({x: i.week, y: i.commits})),
            label: 'Commits',
            backgroundColor: colors['commits'],
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
              maxTicksLimit: 52
            },
          },
          y: {
            ticks: {
              maxTicksLimit: 6
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
    <div class="ui header gt-df gt-ac gt-sb">
      {{ isLoading ? locale.loadingTitle : errorText ? locale.loadingTitleFailed: "Number of commits in the past year" }}
    </div>
    <div class="gt-df ui segment main-graph">
      <div v-if="isLoading || errorText !== ''" class="gt-tc gt-m-auto">
        <div v-if="isLoading">
          <SvgIcon name="octicon-sync" class="gt-mr-3 job-status-rotate"/>
          {{ locale.loadingInfo }}
        </div>
        <div v-else class="text red">
          <SvgIcon name="octicon-x-circle-fill"/>
          {{ errorText }}
        </div>
      </div>
      <Bar
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
