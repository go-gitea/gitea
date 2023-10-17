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
      {{ isLoading ? locale.loadingTitle : errorText ? locale.loadingTitleFailed: "-" }}
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
        <div class="ui dropdown jump" id="dropdown">
          <div class="ui basic compact button">
            <span class="text">
              {{ locale.filterLabel }} <strong>{{ locale.contributionType[type] }}</strong>
              <svg-icon name="octicon-triangle-down" :size="14"/>
            </span>
          </div>
          <div class="menu">
            <div :class="['item', {'active': type === 'commits'}]">
              {{ locale.contributionType.commits }}
            </div>
            <div :class="['item', {'active': type === 'additions'}]">
              {{ locale.contributionType.additions }}
            </div>
            <div :class="['item', {'active': type === 'deletions'}]">
              {{ locale.contributionType.deletions }}
            </div>
          </div>
        </div>
      </div>
    </h2>
    <div class="divider"/>
    <div class="gt-df main-graph">
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
      <ChartLine
        v-memo="[totalStats.weeks, type]" v-if="Object.keys(totalStats).length !== 0"
        :data="toGraphData(totalStats.weeks)" :options="getOptions('main')"
      />
    </div>
    <div class="divider"/>

    <div class="ui attached two column grid">
      <div
        v-for="(contributor, index) in sortedContributors" :key="index" class="column stats-table"
        v-memo="[sortedContributors, type]"
      >
        <div class="ui top attached header gt-df gt-f1">
          <b class="ui right">#{{ index + 1 }}</b>
          <a :href="contributor.home_link">
            <img height="40" width="40" :src="contributor.avatar_link">
          </a>
          <div class="gt-ml-3">
            <a :href="contributor.home_link"><h4>{{ contributor.name }}</h4></a>
            <p class="gt-font-12">
              <strong v-if="contributor.total_commits">{{ contributor.total_commits.toLocaleString() }} {{ locale.contributionType.commits }}</strong>
              <strong v-if="contributor.total_additions" class="text green">{{ contributor.total_additions.toLocaleString() }}++ </strong>
              <strong v-if="contributor.total_deletions" class="text red">
                {{ contributor.total_deletions.toLocaleString() }}--</strong>
            </p>
          </div>
        </div>
        <div class="ui attached segment">
          <div>
            <ChartLine
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
import {Line as ChartLine} from 'vue-chartjs';
import 'chartjs-adapter-dayjs-4/dist/chartjs-adapter-dayjs-4.esm';
import $ from 'jquery';

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
  components: {ChartLine, SvgIcon},
  props: {
    locale: {
      type: Object,
      required: true
    },
  },
  data: () => {
    return {
      isLoading: false,
      errorText: '',
      totalStats: {},
      sortedContributors: {},
      repoLink: pageData.repoLink || [],
      type: pageData.contributionType,
      contributorsStats: [],
      dateFrom: null,
      dateUntil: null,
      startDate: null,
      endDate: null,
    };
  },
  mounted() {
    this.fetchGraphData();

    $('.ui.dropdown').dropdown({
      onChange: (val) => {
        this.dateFrom = this.startDate;
        this.dateUntil = this.endDate;
        this.type = val;
        this.sortContributors();
      }
    });
  },
  methods: {
    sortContributors() {
      const contributors = this.filterContributorWeeksByDateRange();
      const sortingCriteria = `total_${this.type}`;
      this.sortedContributors = Object.values(contributors).filter((contributor) => contributor[sortingCriteria] !== 0).sort((a, b) =>
        a[sortingCriteria] > b[sortingCriteria] ? -1 : a[sortingCriteria] === b[sortingCriteria] ? 0 : 1
      ).slice(0, 100);
    },
    async fetchGraphData() {
      this.isLoading = true;
      try {
        const response = await fetch(`${this.repoLink}/activity/contributors/data`);
        const data = await response.json();
        const {total, ...rest} = data;
        this.contributorsStats = rest;
        this.dateFrom = new Date(total.weeks[0].week);
        this.dateUntil = new Date();
        this.startDate = this.dateFrom;
        this.endDate = this.dateUntil;
        this.sortContributors();
        this.totalStats = total;
        this.errorText = '';
      } catch (e) {
        this.errorText = e.message;
      } finally {
        this.isLoading = false;
      }
    },

    filterContributorWeeksByDateRange() {
      const filteredData = {};

      const data = this.contributorsStats;
      for (const key of Object.keys(data)) {
        const user = data[key];
        user['total_commits'] = 0;
        user['total_additions'] = 0;
        user['total_deletions'] = 0;
        user['max_contribution_type'] = 0;
        const filteredWeeks = user.weeks.filter((week) => {
          const weekDate = new Date(week.week);
          if (weekDate >= this.dateFrom && weekDate <= this.dateUntil) {
            user['total_commits'] += week.commits;
            user['total_additions'] += week.additions;
            user['total_deletions'] += week.deletions;
            if (week[this.type] > user['max_contribution_type']) {
              user['max_contribution_type'] = week[this.type];
            }
            return true;
          }
          return false;
        });

        filteredData[key] = {...user, weeks: filteredWeeks};
      }

      return filteredData;
    },

    maxMainGraph() {
      // This method calculates maximum value for Y value of the main graph.
      // If the number of maximum contributions for selected contribution type is
      // 15.955 it is probably better to round it up to 20.000
      // This method is responsible for doing that
      // Normally, chartjs handles this automatically, but it will resize the
      // graph when you zoom, pan etc. I think resizing the graph makes it harder
      // to compare things visually.

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
    maxContributorGraph() {
      // Similar to maxMainGraph method this method calculates maximum value for Y value
      // for contributors' graph. If I let chartjs do this for me, it will choose different
      // maxY value for each contributors' graph which again makes it harder to compare.

      const maxValue = Math.max(
        ...this.sortedContributors.map((c) => c['max_contribution_type'])
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

    toGraphData(data) {
      const style = getComputedStyle(document.body);
      const colorName = this.type === 'commits' ? '--color-primary-alpha-60' : (this.type === 'additions' ? '--color-green-badge-hover-bg' : '--color-red-badge-hover-bg');
      const color = style.getPropertyValue(colorName).trim();
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

    updateOtherCharts(event, reset) {
      const minVal = event.chart.options.scales.x.min;
      const maxVal = event.chart.options.scales.x.max;
      if (reset) {
        this.dateFrom = this.startDate;
        this.dateUntil = this.endDate;
        this.sortContributors();
      } else if (minVal) {
        this.dateFrom = new Date(minVal);
        this.dateUntil = new Date(maxVal);
        this.sortContributors();
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
            this.updateOtherCharts(e, true);
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
              onPanComplete: this.updateOtherCharts,
            },
            limits: {
              x: {
                /* Check https://www.chartjs.org/chartjs-plugin-zoom/latest/guide/options.html#scale-limits
                   to know what each option means */
                min: 'original',
                max: 'original',

                // number of milliseconds in 2 weeks. Minimum x range will be 2 weeks when you zoom on the graph
                minRange: 2 * 7 * 24 * 60 * 60 * 1000,
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
            max: type === 'main' ? this.maxMainGraph() : this.maxContributorGraph(),
          },
        },
      };
    },
  },
};
</script>
<style scoped>
.main-graph {
  height: 380px;
}
</style>
