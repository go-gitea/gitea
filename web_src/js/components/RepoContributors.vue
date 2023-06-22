<template>
  <div>
    <Line :data="toGraphData(totalStats.weeks, 'main')" :options="getOptions('main')"/>

    <div class="ui attached segment two column grid">
      <div
        v-for="(contributor, index) in sortedContributors"
        :key="index"
        class="column stats-table"
      >
        <div class="ui top attached header gt-df gt-f1">
          <b class="ui right">#{{ index + 1 }}</b>
          <a :href="contributor.home_link">
            <img
              height="40"
              width="40"
              :href="contributor.avatar_link"
              :src="contributor.avatar_link"
            >
          </a>
          <div class="gt-ml-3">
            <a :href="contributor.home_link"><h4>{{ contributor.name }}</h4></a>
            <p class="gt-font-12">
              <strong>{{ contributor.total_commits.toLocaleString() }} commits </strong>
              <strong class="text green">{{ additions(contributor.weeks).toLocaleString() }}++
              </strong>
              <strong class="text red">
                {{ deletions(contributor.weeks).toLocaleString() }}--</strong>
            </p>
          </div>
        </div>
        <div class="ui attached segment">
          <Line :data="toGraphData(contributor.weeks, 'contributor')" :options="getOptions('contributor')"/>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import {createApp} from 'vue';
import {
  Chart,
  Title,
  Tooltip,
  Legend,
  BarElement,
  CategoryScale,
  LinearScale,
  TimeScale,
  PointElement,
  LineElement,
  Filler,
} from 'chart.js';
import zoomPlugin from 'chartjs-plugin-zoom';
import {Line} from 'vue-chartjs';
import 'chartjs-adapter-dayjs-4/dist/chartjs-adapter-dayjs-4.esm';

Chart.register(
  TimeScale,
  CategoryScale,
  LinearScale,
  BarElement,
  Title,
  Tooltip,
  Legend,
  PointElement,
  LineElement,
  Filler,
  zoomPlugin
);

const sfc = {
  components: {Line},
  data: () => ({
    totalStats: window.config.pageData.repoTotalStats || [],
    type: window.config.pageData.contributionType,
    contributorsStats:
      window.config.pageData.repoContributorsStats || [],
  }),
  computed: {
    sortedContributors() {
      return Object.values(this.contributorsStats)
        .sort((a, b) => (a.total_commits > b.total_commits ? -1 : a.total_commits === b.total_commits ? 0 : 1))
        .slice(0, 100);
    },
  },
  methods: {
    maxMainGraph() {
      const maxValue = Math.max(
        ...this.totalStats.weeks.map((o) => o[this.type])
      );
      const [cooefficient, exp] = maxValue.toExponential().split('e').map(Number);
      if (cooefficient % 1 === 0) {
        return maxValue;
      }
      return (1 - (cooefficient % 1)) * 10 ** exp + maxValue;
    },
    additions(data) {
      return Object.values(data).reduce((acc, item) => {
        return acc + item.additions;
      }, 0);
    },
    deletions(data) {
      return Object.values(data).reduce((acc, item) => {
        return acc + item.deletions;
      }, 0);
    },

    toGraphData(data, type) {
      return {
        datasets: [
          {
            data: data.map((i) => {
              return {x: i.week, y: i[this.type]};
            }),
            pointRadius: 0,
            pointHitRadius: 0,
            fill: 'start',
            backgroundColor: type === 'main' ? 'rgba(137, 191, 154, 0.6)' : 'rgba(96, 153, 38, 0.7)',
            borderWidth: 0,
            tension: 0.3,
          },
        ],
      };
    },

    updateOtherCharts(event) {
      const minVal = event.chart.options.scales.x.min;
      const maxVal = event.chart.options.scales.x.max;

      for (const instance of Object.values(Chart.instances)) {
        if (instance !== event.chart) {
          instance.options.scales.x.min = minVal;
          instance.options.scales.x.max = maxVal;
          instance.update();
        }
      }
    },

    getOptions(type) {
      return {
        responsive: true,
        animation: false,
        onClick: (e) => {
          if (type === 'main') {
            e.chart.resetZoom();
          }
        },
        plugins: {
          legend: {
            display: false,
          },
          zoom: {
            pan: {
              enabled: true,
              mode: 'x',
              threshold: 20,

              onPan: this.updateOtherCharts
            },
            limits: {
              x: {
                min: 'original',
                max: 'original',
                minRange: 1000000000,
              },
            },
            zoom: {
              wheel: {
                enabled: type === 'main',
              },
              pinch: {
                enabled: type === 'main',
              },
              mode: 'x',

              onZoomComplete: this.updateOtherCharts
            },
          },
        },
        scales: {
          x: {
            type: 'time',
            grid: {
              display: false,
            },
            time: {
              minUnit: 'day',
            },
          },
          y: {
            min: 0,
            max: this.maxMainGraph(),
          },
        },
      };
    },
  },
};

export function initRepoContributorsChart() {
  const el = document.getElementById('repo-contributors-chart');
  if (el) {
    createApp(sfc).mount(el);
  }
}

export default sfc; // activate the IDE's Vue plugin
</script>
