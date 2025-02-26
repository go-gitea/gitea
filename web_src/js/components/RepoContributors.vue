<script lang="ts">
import {defineComponent, type PropType} from 'vue';
import {SvgIcon} from '../svg.ts';
import dayjs from 'dayjs';
import {
  Chart,
  Title,
  BarElement,
  LinearScale,
  TimeScale,
  PointElement,
  LineElement,
  Filler,
  type ChartOptions,
  type ChartData,
  type Plugin,
} from 'chart.js';
import {GET} from '../modules/fetch.ts';
import zoomPlugin from 'chartjs-plugin-zoom';
import {Line as ChartLine} from 'vue-chartjs';
import {
  startDaysBetween,
  firstStartDateAfterDate,
  fillEmptyStartDaysWithZeroes,
} from '../utils/time.ts';
import {chartJsColors} from '../utils/color.ts';
import {sleep} from '../utils.ts';
import 'chartjs-adapter-dayjs-4/dist/chartjs-adapter-dayjs-4.esm';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import type {Entries} from 'type-fest';
import {pathEscapeSegments} from '../utils/url.ts';

const customEventListener: Plugin = {
  id: 'customEventListener',
  afterEvent: (chart, args, opts) => {
    // event will be replayed from chart.update when reset zoom,
    // so we need to check whether args.replay is true to avoid call loops
    if (args.event.type === 'dblclick' && opts.chartType === 'main' && !args.replay) {
      chart.resetZoom();
      opts.instance.updateOtherCharts(args.event, true);
    }
  },
};

Chart.defaults.color = chartJsColors.text;
Chart.defaults.borderColor = chartJsColors.border;

Chart.register(
  TimeScale,
  LinearScale,
  BarElement,
  Title,
  PointElement,
  LineElement,
  Filler,
  zoomPlugin,
  customEventListener,
);

export default defineComponent({
  components: {ChartLine, SvgIcon},
  props: {
    locale: {
      type: Object as PropType<Record<string, any>>,
      required: true,
    },
    repoLink: {
      type: String,
      required: true,
    },
    repoDefaultBranchName: {
      type: String,
      required: true,
    },
  },
  data: () => ({
    isLoading: false,
    errorText: '',
    totalStats: {} as Record<string, any>,
    sortedContributors: {} as Record<string, any>,
    type: 'commits',
    contributorsStats: {} as Record<string, any>,
    xAxisStart: null as number | null,
    xAxisEnd: null as number | null,
    xAxisMin: null as number | null,
    xAxisMax: null as number | null,
  }),
  mounted() {
    this.fetchGraphData();

    fomanticQuery('#repo-contributors').dropdown({
      onChange: (val: string) => {
        this.xAxisMin = this.xAxisStart;
        this.xAxisMax = this.xAxisEnd;
        this.type = val;
        this.sortContributors();
      },
    });
  },
  methods: {
    sortContributors() {
      const contributors: Record<string, any> = this.filterContributorWeeksByDateRange();
      const criteria = `total_${this.type}`;
      this.sortedContributors = Object.values(contributors)
        .filter((contributor) => contributor[criteria] !== 0)
        .sort((a, b) => a[criteria] > b[criteria] ? -1 : a[criteria] === b[criteria] ? 0 : 1)
        .slice(0, 100);
    },

    getContributorSearchQuery(contributorEmail: string) {
      const min = dayjs(this.xAxisMin).format('YYYY-MM-DD');
      const max = dayjs(this.xAxisMax).format('YYYY-MM-DD');
      const params = new URLSearchParams({
        'q': `after:${min}, before:${max}, author:${contributorEmail}`,
      });
      return `${this.repoLink}/commits/branch/${pathEscapeSegments(this.repoDefaultBranchName)}/search?${params.toString()}`;
    },

    async fetchGraphData() {
      this.isLoading = true;
      try {
        let response: Response;
        do {
          response = await GET(`${this.repoLink}/activity/contributors/data`);
          if (response.status === 202) {
            await sleep(1000); // wait for 1 second before retrying
          }
        } while (response.status === 202);
        if (response.ok) {
          const data = await response.json();
          const {total, ...rest} = data;
          // below line might be deleted if we are sure go produces map always sorted by keys
          total.weeks = Object.fromEntries(Object.entries(total.weeks).sort());

          const weekValues = Object.values(total.weeks) as any;
          this.xAxisStart = weekValues[0].week;
          this.xAxisEnd = firstStartDateAfterDate(new Date());
          const startDays = startDaysBetween(this.xAxisStart, this.xAxisEnd);
          total.weeks = fillEmptyStartDaysWithZeroes(startDays, total.weeks);
          this.xAxisMin = this.xAxisStart;
          this.xAxisMax = this.xAxisEnd;
          this.contributorsStats = {};
          for (const [email, user] of Object.entries(rest) as Entries<Record<string, Record<string, any>>>) {
            user.weeks = fillEmptyStartDaysWithZeroes(startDays, user.weeks);
            this.contributorsStats[email] = user;
          }
          this.sortContributors();
          this.totalStats = total;
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

    filterContributorWeeksByDateRange() {
      const filteredData: Record<string, any> = {};
      const data = this.contributorsStats;
      for (const key of Object.keys(data)) {
        const user = data[key];
        user.total_commits = 0;
        user.total_additions = 0;
        user.total_deletions = 0;
        user.max_contribution_type = 0;
        const filteredWeeks = user.weeks.filter((week: Record<string, number>) => {
          const oneWeek = 7 * 24 * 60 * 60 * 1000;
          if (week.week >= this.xAxisMin - oneWeek && week.week <= this.xAxisMax + oneWeek) {
            user.total_commits += week.commits;
            user.total_additions += week.additions;
            user.total_deletions += week.deletions;
            if (week[this.type] > user.max_contribution_type) {
              user.max_contribution_type = week[this.type];
            }
            return true;
          }
          return false;
        });
        // this line is required. See https://github.com/sahinakkaya/gitea/pull/3#discussion_r1396495722
        // for details.
        user.max_contribution_type += 1;

        filteredData[key] = {...user, weeks: filteredWeeks, email: key};
      }

      return filteredData;
    },

    maxMainGraph() {
      // This method calculates maximum value for Y value of the main graph. If the number
      // of maximum contributions for selected contribution type is 15.955 it is probably
      // better to round it up to 20.000.This method is responsible for doing that.
      // Normally, chartjs handles this automatically, but it will resize the graph when you
      // zoom, pan etc. I think resizing the graph makes it harder to compare things visually.
      const maxValue = Math.max(
        ...this.totalStats.weeks.map((o: Record<string, any>) => o[this.type]),
      );
      const [coefficient, exp] = maxValue.toExponential().split('e').map(Number);
      if (coefficient % 1 === 0) return maxValue;
      return (1 - (coefficient % 1)) * 10 ** exp + maxValue;
    },

    maxContributorGraph() {
      // Similar to maxMainGraph method this method calculates maximum value for Y value
      // for contributors' graph. If I let chartjs do this for me, it will choose different
      // maxY value for each contributors' graph which again makes it harder to compare.
      const maxValue = Math.max(
        ...this.sortedContributors.map((c: Record<string, any>) => c.max_contribution_type),
      );
      const [coefficient, exp] = maxValue.toExponential().split('e').map(Number);
      if (coefficient % 1 === 0) return maxValue;
      return (1 - (coefficient % 1)) * 10 ** exp + maxValue;
    },

    toGraphData(data: Array<Record<string, any>>): ChartData<'line'> {
      return {
        datasets: [
          {
            data: data.map((i) => ({x: i.week, y: i[this.type]})),
            pointRadius: 0,
            pointHitRadius: 0,
            fill: 'start',
            backgroundColor: chartJsColors[this.type],
            borderWidth: 0,
            tension: 0.3,
          },
        ],
      };
    },

    updateOtherCharts({chart}: {chart: Chart}, reset: boolean = false) {
      const minVal = Number(chart.options.scales.x.min);
      const maxVal = Number(chart.options.scales.x.max);
      if (reset) {
        this.xAxisMin = this.xAxisStart;
        this.xAxisMax = this.xAxisEnd;
        this.sortContributors();
      } else if (minVal) {
        this.xAxisMin = minVal;
        this.xAxisMax = maxVal;
        this.sortContributors();
      }
    },

    getOptions(type: string): ChartOptions<'line'> {
      return {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        events: ['mousemove', 'mouseout', 'click', 'touchstart', 'touchmove', 'dblclick'],
        plugins: {
          title: {
            display: type === 'main',
            text: 'drag: zoom, shift+drag: pan, double click: reset zoom',
            position: 'top',
            align: 'center',
          },
          // @ts-expect-error: bug in chart.js types
          customEventListener: {
            chartType: type,
            instance: this,
          },
          zoom: {
            pan: {
              enabled: true,
              modifierKey: 'shift',
              mode: 'x',
              threshold: 20,
              onPanComplete: this.updateOtherCharts,
            },
            limits: {
              x: {
                // Check https://www.chartjs.org/chartjs-plugin-zoom/latest/guide/options.html#scale-limits
                // to know what each option means
                min: 'original',
                max: 'original',

                // number of milliseconds in 2 weeks. Minimum x range will be 2 weeks when you zoom on the graph
                minRange: 2 * 7 * 24 * 60 * 60 * 1000,
              },
            },
            zoom: {
              drag: {
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
            min: this.xAxisMin,
            max: this.xAxisMax,
            type: 'time',
            grid: {
              display: false,
            },
            time: {
              minUnit: 'month',
            },
            ticks: {
              maxRotation: 0,
              maxTicksLimit: type === 'main' ? 12 : 6,
            },
          },
          y: {
            min: 0,
            max: type === 'main' ? this.maxMainGraph() : this.maxContributorGraph(),
            ticks: {
              maxTicksLimit: type === 'main' ? 6 : 4,
            },
          },
        },
      };
    },
  },
});
</script>
<template>
  <div>
    <div class="ui header tw-flex tw-items-center tw-justify-between">
      <div>
        <relative-time
          v-if="xAxisMin > 0"
          format="datetime"
          year="numeric"
          month="short"
          day="numeric"
          weekday=""
          :datetime="new Date(xAxisMin)"
        >
          {{ new Date(xAxisMin) }}
        </relative-time>
        {{ isLoading ? locale.loadingTitle : errorText ? locale.loadingTitleFailed: "-" }}
        <relative-time
          v-if="xAxisMax > 0"
          format="datetime"
          year="numeric"
          month="short"
          day="numeric"
          weekday=""
          :datetime="new Date(xAxisMax)"
        >
          {{ new Date(xAxisMax) }}
        </relative-time>
      </div>
      <div>
        <!-- Contribution type -->
        <div class="ui dropdown jump" id="repo-contributors">
          <div class="ui basic compact button">
            <span class="not-mobile">{{ locale.filterLabel }}</span> <strong>{{ locale.contributionType[type] }}</strong>
            <svg-icon name="octicon-triangle-down" :size="14"/>
          </div>
          <div class="menu">
            <div :class="['item', {'selected': type === 'commits'}]" data-value="commits">
              {{ locale.contributionType.commits }}
            </div>
            <div :class="['item', {'selected': type === 'additions'}]" data-value="additions">
              {{ locale.contributionType.additions }}
            </div>
            <div :class="['item', {'selected': type === 'deletions'}]" data-value="deletions">
              {{ locale.contributionType.deletions }}
            </div>
          </div>
        </div>
      </div>
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
        v-memo="[totalStats.weeks, type]" v-if="Object.keys(totalStats).length !== 0"
        :data="toGraphData(totalStats.weeks)" :options="getOptions('main')"
      />
    </div>
    <div class="contributor-grid">
      <div
        v-for="(contributor, index) in sortedContributors"
        :key="index"
        v-memo="[sortedContributors, type]"
      >
        <div class="ui top attached header tw-flex tw-flex-1">
          <b class="ui right">#{{ index + 1 }}</b>
          <a :href="contributor.home_link">
            <img class="ui avatar tw-align-middle" height="40" width="40" :src="contributor.avatar_link" alt="">
          </a>
          <div class="tw-ml-2">
            <a v-if="contributor.home_link !== ''" :href="contributor.home_link"><h4>{{ contributor.name }}</h4></a>
            <h4 v-else class="contributor-name">
              {{ contributor.name }}
            </h4>
            <p class="tw-text-12 tw-flex tw-gap-1">
              <strong v-if="contributor.total_commits">
                <a class="silenced" :href="getContributorSearchQuery(contributor.email)">
                  {{ contributor.total_commits.toLocaleString() }} {{ locale.contributionType.commits }}
                </a>
              </strong>
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
<style scoped>
.main-graph {
  height: 260px;
  padding-top: 2px;
}

.contributor-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1rem;
}

.contributor-grid > * {
  min-width: 0;
}

@media (max-width: 991.98px) {
  .contributor-grid {
    grid-template-columns: repeat(1, 1fr);
  }
}

.contributor-name {
  margin-bottom: 0;
}
</style>
