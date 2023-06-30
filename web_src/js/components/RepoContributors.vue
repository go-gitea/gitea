<template>
  <div>
    <h2 class="ui header">
      <relative-time
        v-if="dateFrom !== null"
        format="datetime"
        year="numeric"
        month="short"
        day="numeric"
        weekday=""
        :datetime="dateFrom"
      >
        {{ dateFrom }}
      </relative-time>
      -
      <relative-time
        v-if="dateUntil !== null"
        format="datetime"
        year="numeric"
        month="short"
        day="numeric"
        weekday=""
        :datetime="dateUntil"
      >
        {{ dateUntil }}
      </relative-time>

      <div class="ui right">
        <!-- Contribution type -->
        <div class="ui dropdown simple">
          <div class="ui basic compact button">
            <span class="text">
              Contribution type: <strong>{{ type }}</strong>
              <svg-icon name="octicon-triangle-down" :size="14"/>
            </span>
          </div>
          <div class="menu">
            <a :class="type === 'commits' ? 'active item' : 'item'" :href="`${repoLink}/activity/contributors/commits`">Commits</a>
            <a :class="type === 'additions' ? 'active item' : 'item'" :href="`${repoLink}/activity/contributors/additions`">Additions</a>
            <a :class="type === 'deletions' ? 'active item' : 'item'" :href="`${repoLink}/activity/contributors/deletions`">Deletions</a>
          </div>
        </div>
      </div>
    </h2>
    <div class="ui divider"/>
    <div style="height: 380px">
      <CLine
        v-memo="[totalStats.weeks]"
        v-if="Object.keys(totalStats).length !== 0"
        :data="toGraphData(totalStats.weeks)"
        :options="getOptions('main')"
      />
    </div>
    <div class="ui divider"/>

    <div class="ui attached two column grid">
      <div
        v-for="(contributor, index) in sortedContributors"
        :key="index"
        class="column stats-table"
        v-memo="[sortedContributors]"
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
              <strong>{{ contributor.total_commits.toLocaleString() }} commits
              </strong>
              <strong class="text green">{{ additions(contributor.weeks).toLocaleString() }}++
              </strong>
              <strong class="text red">
                {{ deletions(contributor.weeks).toLocaleString() }}--</strong>
            </p>
          </div>
        </div>
        <div class="ui attached segment">
          <div>
            <CLine
              :data="toGraphData(contributor.weeks)"
              :options="getOptions('contributor')"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import {SvgIcon} from '../svg.js';
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
import {Line as CLine} from 'vue-chartjs';
import 'chartjs-adapter-dayjs-4/dist/chartjs-adapter-dayjs-4.esm';

const {pageData} = window.config;

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

export default {
  components: {CLine, SvgIcon},
  data: () => {
    return {
      isLoading: false,
      totalStats: {},
      repoLink: pageData.repoLink || [],
      type: pageData.contributionType,
      contributorsStats: [],
      dateFrom: null,
      dateUntil: null,
    };
  },
  computed: {
    sortedContributors() {
      return Object.values(this.contributorsStats).sort((a, b) =>
        a.total_commits > b.total_commits ? -1 : a.total_commits === b.total_commits ? 0 : 1
      ).slice(0, 100);
    },
  },
  mounted() {
    this.fetchGraphData();
  },
  methods: {
    async fetchGraphData() {
      this.isLoading = true;
      fetch(`${this.repoLink}/activity/contributors/data`)
        .then((response) => response.json())
        .then((data) => {
          const {total, ...rest} = data;
          this.contributorsStats = rest;
          this.totalStats = total;
          this.dateFrom = new Date(total.weeks[0].week).toISOString();
          this.dateUntil = new Date().toISOString();
          this.isLoading = false;
        });
    },
    maxMainGraph() {
      const maxValue = Math.max(
        ...this.totalStats.weeks.map((o) => o[this.type])
      );
      const [cooefficient, exp] = maxValue
        .toExponential()
        .split('e')
        .map(Number);
      if (cooefficient % 1 === 0) {
        return maxValue;
      }
      return (1 - (cooefficient % 1)) * 10 ** exp + maxValue;
    },
    additions(data) {
      return Object.values(data).reduce((acc, item) => acc + item.additions, 0);
    },
    deletions(data) {
      return Object.values(data).reduce((acc, item) => acc + item.deletions, 0);
    },

    toGraphData(data) {
      const style = getComputedStyle(document.body);
      const colorName = this.type === 'commits' ? '--color-primary-alpha-60' : (this.type === 'additions' ? '--color-green-badge-hover-bg' : '--color-red-badge-hover-bg');
      const color = style.getPropertyValue(colorName);
      return {
        datasets: [
          {
            data: data.map((i) => {
              return {x: i.week, y: i[this.type]};
            }),
            pointRadius: 0,
            pointHitRadius: 0,
            fill: 'start',
            backgroundColor: color,
            borderWidth: 0,
            tension: 0.3,
          },
        ],
      };
    },

    updateOtherCharts(event) {
      const minVal = event.chart.options.scales.x.min;
      const maxVal = event.chart.options.scales.x.max;
      if (minVal) {
        this.dateFrom = new Date(minVal).toISOString();
        this.dateUntil = new Date(maxVal).toISOString();
      }

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
        maintainAspectRatio: false,
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

              onPan: this.updateOtherCharts,
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
              onZoomComplete: this.updateOtherCharts,
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
          },
        },
      };
    },
  },
};
</script>
