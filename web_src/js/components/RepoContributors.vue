<template>
  <div>
    <Line :data="graphData" :options="data.options" />
  </div>
</template>

<script>
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
  components: {Line},
  data: () => ({
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

    contributorsCommitsStats: window.config.pageData.repoContributorsCommitStats|| [],
  }),
  computed: {
    graphData() {
      return {
        datasets: [{
          label: 'Number of additions',
          data: Object.entries(this.contributorsCommitsStats[""].weeks).map(([date, stats]) => {
            return {x: date, y: stats.commits}
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
  },
};

export function initRepoContributorsChart() {
  const el = document.getElementById('repo-contributors-master-chart');
  if (el) {
    createApp(sfc).mount(el);
  }
}

export default sfc; // activate the IDE's Vue plugin
</script>
