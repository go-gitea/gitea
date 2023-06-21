<template>
  <div>
    <Line :data="mainGraphData" :options="options" />

    <div class="ui attached segment two column grid">
      <div
        v-for="(contributor, index) in individualGraphData"
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
            />
          </a>
          <div class="gt-ml-3">
            <a :href="contributor.home_link"
              ><h4>{{ contributor.name }}</h4></a
            >
            <p class="gt-font-12">
              <strong>{{ contributor.total.toLocaleString() }} commits </strong>
              <strong class="text green"
                >{{ additions(contributor.weeks).toLocaleString() }}++
              </strong>
              <strong class="text red">
                {{ deletions(contributor.weeks).toLocaleString() }}--</strong
              >
            </p>
          </div>
        </div>
        <div class="ui attached segment">
          <Line :data="graph(contributor.weeks)" :options="options" />
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { createApp } from "vue";
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
  Filler,
} from "chart.js";
import zoomPlugin from "chartjs-plugin-zoom";
import { Bar, Line } from "vue-chartjs";
import "chartjs-adapter-dayjs-4/dist/chartjs-adapter-dayjs-4.esm";

ChartJS.register(
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
  components: { Line },
  data: () => ({
    data: {},

    masterChartData: window.config.pageData.repoContributorsCommitStats || [],
    type: window.config.pageData.contributionType,
    individualChartsData:
      window.config.pageData.repoContributorsCommitStats || [],
  }),
  computed: {
    options() {
      return {
        responsive: true,
        animation: false,
        onClick: (e) => {
          e.chart.resetZoom()
        },
        plugins: {
          legend: {
            display: false,
          },
          zoom: {
            pan: {
              enabled: true,
              mode: "x",
              threshold: 20,

              onPan: function (event) {
                var minVal = event.chart.options.scales.x.min;
                var maxVal = event.chart.options.scales.x.max;

                Object.values(ChartJS.instances).forEach(function (instance) {
                  if (instance !== event.chart){
                    instance.options.scales.x.min = minVal;
                    instance.options.scales.x.max = maxVal;
                    instance.update();
                  }
                });
              },
            },
            limits: {
              x: {
                min: "original",
                max: "original",
                minRange: 1000000000,
              },
            },
            zoom: {
              wheel: {
                enabled: true,
              },
              pinch: {
                enabled: true,
              },
              mode: "x",

              onZoomComplete: function (event) {
                var minVal = event.chart.options.scales.x.min;
                var maxVal = event.chart.options.scales.x.max;

                Object.values(ChartJS.instances).forEach(function (instance) {
                  if (instance !== event.chart){
                    instance.options.scales.x.min = minVal;
                    instance.options.scales.x.max = maxVal;
                    instance.update();
                  }
                });
              },

            },
          },
        },
        scales: {
          x: {
            type: "time",
            time: {
              // unit: 'year'
              minUnit: "day",
            },
          },
          y: {
            min: 0,
            max: this.maxMainGraph(),
          },
        },
      };
    },
    mainGraphData() {
      return {
        datasets: [
          {
            data: this.masterChartData[""].weeks.map((i) => {
              return { x: i.week, y: i[this.type] };
            }),
            pointRadius: 0,
            pointHitRadius: 0,
            fill: "start",
            borderColor: "rgb(75, 192, 192)",
            borderWidth: 0,
            backgroundColor: "rgba(137, 191, 154, 0.6)",
            tension: 0.3,
          },
        ],
      };
    },
    individualGraphData() {
      let { "": _, ...rest } = this.individualChartsData;
      console.log(rest);
      const data = Object.values(rest)
        .sort((a, b) => (a.total > b.total ? -1 : a.total == b.total ? 0 : 1))
        .slice(0, 100);
      console.log(data);
      return data;
    },
  },
  methods: {
    maxMainGraph() {
      const maxValue = Math.max(
        ...this.masterChartData[""].weeks.map((o) => o[this.type])
      );
      const [cooefficient, exp] = maxValue.toExponential().split("e");
      if (Number(cooefficient) % 1 == 0) {
        return maxValue;
      }
      return (1 - (Number(cooefficient) % 1)) * 10 ** Number(exp) + maxValue;
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
    graph(data) {
      return {
        datasets: [
          {
            data: data.map((i) => {
              return { x: i.week, y: i[this.type] };
            }),
            pointRadius: 0,
            pointHitRadius: 0,
            fill: "start",
            borderColor: "rgb(75, 192, 192)",
            backgroundColor: "rgba(96, 153, 38, 0.7)",
            borderWidth: 0,
            tension: 0.3,
          },
        ],
      };
    },
  },
};

export function initRepoContributorsChart() {
  const el = document.getElementById("repo-contributors-chart");
  if (el) {
    createApp(sfc).mount(el);
  }
}

export default sfc; // activate the IDE's Vue plugin
</script>
