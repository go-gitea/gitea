<script lang="ts" setup>
import {computed, onMounted, shallowRef} from 'vue';
import {SvgIcon} from '../svg.ts';
import dayjs from 'dayjs';
import {GET} from '../modules/fetch.ts';
import {Line as ChartLine} from 'vue-chartjs';
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
import zoomPlugin from 'chartjs-plugin-zoom';
import {chartJsColors} from '../utils/color.ts';
import 'chartjs-adapter-dayjs-4/dist/chartjs-adapter-dayjs-4.esm';
import {
  startDaysBetween,
  firstStartDateAfterDate,
  fillEmptyStartDaysWithZeroes,
} from '../utils/time.ts';
import {errorMessage} from '../modules/errors.ts';
import {sleep} from '../utils.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {pathEscapeSegments} from '../utils/url.ts';

type ContributionType = 'commits' | 'additions' | 'deletions';
type ChartType = 'main' | 'contributor';

const oneWeek = 7 * 24 * 60 * 60 * 1000;

const customEventListener: Plugin = {
  id: 'customEventListener',
  afterEvent: (chart, args, opts) => {
    // event will be replayed from chart.update when reset zoom,
    // so we need to check whether args.replay is true to avoid call loops
    if (args.event.type === 'dblclick' && opts.chartType === 'main' && !args.replay) {
      chart.resetZoom();
      opts.onDoubleClick({chart}, true);
    }
  },
};

type LineOptions = ChartOptions<'line'> & {
 plugins?: {
   customEventListener?: {
     chartType: ChartType;
     onDoubleClick: (args: {chart: Chart}, reset: boolean) => void;
   };
 };
}

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

// rounds up to the next multiple of the leading power of ten, so the axis does not rescale on zoom and pan
function roundUpMax(maxValue: number) {
  const [coefficient, exp] = maxValue.toExponential().split('e').map(Number);
  return Math.ceil(coefficient) * 10 ** exp;
}

type ContributorsData = {
  total: {
    weeks: Record<string, any>,
  },
  [other: string]: Record<string, Record<string, any>>,
}

const props = defineProps<{
  locale: {
    filterLabel: string;
    contributionType: Record<ContributionType, string>;
    loadingTitle: string;
    loadingTitleFailed: string;
    loadingInfo: string;
    chartZoomHint: string;
  };
  repoLink: string;
  repoDefaultBranchName: string;
}>();

const isLoading = shallowRef(false);
const errorText = shallowRef('');
const totalStats = shallowRef<Record<string, any>>({});
const sortedContributors = shallowRef<Array<Record<string, any>>>([]);
const type = shallowRef<ContributionType>('commits');
let contributorsStats: Record<string, any> = {}; // these three are not read during render
let xAxisStart: number | null = null;
let xAxisEnd: number | null = null;
const xAxisMin = shallowRef<number | null>(null);
const xAxisMax = shallowRef<number | null>(null);

onMounted(() => {
  fetchGraphData();

  fomanticQuery('#repo-contributors').dropdown({
    onChange: (val: ContributionType) => {
      xAxisMin.value = xAxisStart;
      xAxisMax.value = xAxisEnd;
      type.value = val;
      sortContributors();
    },
  });
});

function sortContributors() {
  const criteria = `total_${type.value}`;
  sortedContributors.value = filterContributorWeeksByDateRange()
    .filter((contributor) => contributor[criteria] !== 0)
    .sort((a, b) => b[criteria] - a[criteria])
    .slice(0, 100);
}

const searchBase = computed(() => {
  const min = dayjs(xAxisMin.value).format('YYYY-MM-DD');
  const max = dayjs(xAxisMax.value).format('YYYY-MM-DD');
  return {prefix: `after:${min}, before:${max}, author:`, branch: pathEscapeSegments(props.repoDefaultBranchName)};
});

function getContributorSearchQuery(contributorEmail: string) {
  const params = new URLSearchParams({'q': `${searchBase.value.prefix}${contributorEmail}`});
  return `${props.repoLink}/commits/branch/${searchBase.value.branch}/search?${params.toString()}`;
}

async function fetchGraphData() {
  isLoading.value = true;
  try {
    let response: Response;
    do {
      response = await GET(`${props.repoLink}/activity/contributors/data`);
      if (response.status === 202) {
        await sleep(1000); // wait for 1 second before retrying
      }
    } while (response.status === 202);
    if (response.ok) {
      const data = await response.json() as ContributorsData;
      const {total, ...other} = data;
      // below line might be deleted if we are sure go produces map always sorted by keys
      total.weeks = Object.fromEntries(Object.entries(total.weeks).sort());

      const weekValues = Object.values(total.weeks);
      xAxisStart = weekValues[0].week;
      xAxisEnd = firstStartDateAfterDate(new Date());
      const startDays = startDaysBetween(xAxisStart, xAxisEnd);
      total.weeks = fillEmptyStartDaysWithZeroes(startDays, total.weeks);
      xAxisMin.value = xAxisStart;
      xAxisMax.value = xAxisEnd;
      contributorsStats = Object.fromEntries(Object.entries(other).map(([email, user]) => {
        return [email, {...user, weeks: fillEmptyStartDaysWithZeroes(startDays, user.weeks)}];
      }));
      sortContributors();
      totalStats.value = total;
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

function filterContributorWeeksByDateRange() {
  const filteredData: Array<Record<string, any>> = [];
  const minTime = xAxisMin.value! - oneWeek;
  const maxTime = xAxisMax.value! + oneWeek;
  const contributionType = type.value;
  for (const [key, user] of Object.entries(contributorsStats)) {
    user.total_commits = 0;
    user.total_additions = 0;
    user.total_deletions = 0;
    user.max_contribution_type = 0;
    const filteredWeeks = user.weeks.filter((week: Record<string, number>) => {
      if (week.week >= minTime && week.week <= maxTime) {
        user.total_commits += week.commits;
        user.total_additions += week.additions;
        user.total_deletions += week.deletions;
        if (week[contributionType] > user.max_contribution_type) {
          user.max_contribution_type = week[contributionType];
        }
        return true;
      }
      return false;
    });
    // this line is required. See https://github.com/sahinakkaya/gitea/pull/3#discussion_r1396495722
    // for details.
    user.max_contribution_type += 1;

    filteredData.push({...user, weeks: filteredWeeks, email: key});
  }

  return filteredData;
}

const maxMainGraph = computed(() => {
  return roundUpMax(Math.max(...totalStats.value.weeks.map((o: Record<string, any>) => o[type.value])));
});

// one shared maximum, otherwise the contributor graphs cannot be compared
const maxContributorGraph = computed(() => {
  return roundUpMax(Math.max(...sortedContributors.value.map((c: Record<string, any>) => c.max_contribution_type)));
});

function toGraphData(data: Array<Record<string, any>>): ChartData<'line'> {
  const contributionType = type.value;
  return {
    datasets: [
      {
        data: data.map((i) => ({x: i.week, y: i[contributionType]})),
        pointRadius: 0,
        pointHitRadius: 0,
        fill: 'start',
        backgroundColor: chartJsColors[type.value],
        borderWidth: 0,
        tension: 0.3,
      },
    ],
  };
}

function updateOtherCharts({chart}: {chart: Chart}, reset: boolean = false) {
  const minVal = Number(chart.options.scales?.x?.min);
  const maxVal = Number(chart.options.scales?.x?.max);
  if (reset) {
    xAxisMin.value = xAxisStart;
    xAxisMax.value = xAxisEnd;
    sortContributors();
  } else if (minVal) {
    xAxisMin.value = minVal;
    xAxisMax.value = maxVal;
    sortContributors();
  }
}

function getOptions(chartType: ChartType): LineOptions {
  return {
    responsive: true,
    maintainAspectRatio: false,
    animation: false,
    events: ['mousemove', 'mouseout', 'click', 'touchstart', 'touchmove', 'dblclick'],
    plugins: {
      title: {
        display: chartType === 'main',
        text: props.locale.chartZoomHint,
        position: 'top',
        align: 'center',
      },
      customEventListener: {
        chartType,
        onDoubleClick: updateOtherCharts,
      },
      zoom: {
        pan: {
          enabled: true,
          modifierKey: 'shift',
          mode: 'x',
          threshold: 20,
          onPanComplete: updateOtherCharts,
        },
        limits: {
          x: {
            // Check https://www.chartjs.org/chartjs-plugin-zoom/latest/guide/options.html#scale-limits
            // to know what each option means
            min: 'original',
            max: 'original',

            minRange: 2 * oneWeek, // do not zoom in tighter than two weeks

          },
        },
        zoom: {
          drag: {
            enabled: chartType === 'main',
          },
          pinch: {
            enabled: chartType === 'main',
          },
          mode: 'x',
          onZoomComplete: updateOtherCharts,
        },
      },
    },
    scales: {
      x: {
        min: xAxisMin.value ?? undefined,
        max: xAxisMax.value ?? undefined,
        type: 'time',
        grid: {
          display: false,
        },
        time: {
          minUnit: 'month',
        },
        ticks: {
          maxRotation: 0,
          maxTicksLimit: chartType === 'main' ? 12 : 6,
        },
      },
      y: {
        min: 0,
        max: chartType === 'main' ? maxMainGraph.value : maxContributorGraph.value,
        ticks: {
          maxTicksLimit: chartType === 'main' ? 6 : 4,
        },
      },
    },
  };
}
</script>
<template>
  <div>
    <div class="ui header flex-left-right">
      <div>
        <relative-time
          v-if="xAxisMin && xAxisMin > 0"
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
          v-if="xAxisMax && xAxisMax > 0"
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
        <div class="ui floating dropdown jump" id="repo-contributors">
          <div class="ui basic compact button">
            <span class="not-mobile">{{ locale.filterLabel }}</span> <strong>{{ locale.contributionType[type] }}</strong>
            <svg-icon name="octicon-triangle-down" :size="14"/>
          </div>
          <div class="left menu">
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
            <img loading="lazy" class="ui avatar tw-align-middle" height="40" width="40" :src="contributor.avatar_link" alt="">
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
              <strong v-if="contributor.total_additions" class="tw-text-green">{{ contributor.total_additions.toLocaleString() }}++ </strong>
              <strong v-if="contributor.total_deletions" class="tw-text-red">
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
