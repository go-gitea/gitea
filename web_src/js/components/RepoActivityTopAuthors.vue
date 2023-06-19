<template>
  <div>
    <Line :data="graphData" :options="data.options" />
    <div class="activity-bar-graph" ref="style" style="width: 0; height: 0;"/>
    <div class="activity-bar-graph-alt" ref="altStyle" style="width: 0; height: 0;"/>
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

<script>
import VueBarGraph from 'vue-bar-graph';
import {createApp} from 'vue';
import {
  Chart as ChartJS,
  Title,
  Tooltip,
  Legend,
  BarElement,
  CategoryScale,
  LinearScale,
  TimeScale,
  PointElement,
  LineElement,
  Filler
} from 'chart.js'
import { Bar, Line } from 'vue-chartjs'
import 'chartjs-adapter-dayjs-4/dist/chartjs-adapter-dayjs-4.esm';

ChartJS.register(TimeScale, CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend, PointElement, LineElement, Filler)

const sfc = {
  components: {VueBarGraph, Line},
  data: () => ({
    colors: {
      barColor: 'green',
      textColor: 'black',
      textAltColor: 'white',
    },
    data: {
      options: {
        responsive: true,
        scales: {
            x: {
                // min: '2021-11-06',
                type: 'time',
                time: {
                    // unit: 'year'
                    minUnit: 'day'
                }
            },
            y: {
               min: 0
            }
        }
      }
    },

    // possible keys:
    // * avatar_link: (...)
    // * commits: (...)
    // * home_link: (...)
    // * login: (...)
    // * name: (...)
    activityTopAuthors: window.config.pageData.repoActivityTopAuthors || [],
    contributorsCommitsStats: window.config.pageData.repoContributorsCommitStats|| [],
  }),
  computed: {
    graphData() {
      return {
        datasets: [{
          label: 'Number of additions',
          data: this.contributorsCommitsStats.map((item) => {
            return {x: item.date, y: item.stats.additions}
          }),
          pointRadius: 0,
          pointHitRadius: 0,
          fill: 'start',
          borderColor: 'rgb(75, 192, 192)',
          backgroundColor: 'rgba(65, 182, 182, 0.3)',
          tension: 0.2
        }]
      }
    },
    graphPoints() {
      return this.activityTopAuthors.map((item) => {
        return {
          value: item.commits,
          label: item.name,
        };
      });
    },
    graphAuthors() {
      return this.activityTopAuthors.map((item, idx) => {
        return {
          position: idx + 1,
          ...item,
        };
      });
    },
    graphWidth() {
      return this.activityTopAuthors.length * 40;
    },
  },
  mounted() {
    const refStyle = window.getComputedStyle(this.$refs.style);
    const refAltStyle = window.getComputedStyle(this.$refs.altStyle);

    this.colors.barColor = refStyle.backgroundColor;
    this.colors.textColor = refStyle.color;
    this.colors.textAltColor = refAltStyle.color;
  }
};

export function initRepoActivityTopAuthorsChart() {
  const el = document.getElementById('repo-activity-top-authors-chart');
  if (el) {
    createApp(sfc).mount(el);
  }
  const el2 = document.getElementById('repo-contributors-master-chart');
  if (el2) {
    createApp(sfc).mount(el2);
  }
}

export default sfc; // activate the IDE's Vue plugin
</script>
