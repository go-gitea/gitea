import Vue from 'vue';

const { AppSubUrl, heatmapUser } = window.config;

export default async function initHeatmap() {
  const el = document.getElementById('user-heatmap');
  if (!el) return;

  const { CalendarHeatmap } = await import(/* webpackChunkName: "userheatmap" */'vue-calendar-heatmap');
  Vue.component('calendarHeatmap', CalendarHeatmap);

  const vueDelimeters = ['${', '}'];

  Vue.component('activity-heatmap', {
    delimiters: vueDelimeters,

    props: {
      user: {
        type: String,
        required: true
      },
      suburl: {
        type: String,
        required: true
      },
      locale: {
        type: Object,
        required: true
      }
    },

    data() {
      return {
        isLoading: true,
        colorRange: [],
        endDate: null,
        values: [],
        totalContributions: 0,
      };
    },

    mounted() {
      this.colorRange = [
        this.getColor(0),
        this.getColor(1),
        this.getColor(2),
        this.getColor(3),
        this.getColor(4),
        this.getColor(5)
      ];
      this.endDate = new Date();
      this.loadHeatmap(this.user);
    },

    methods: {
      loadHeatmap(userName) {
        const self = this;
        $.get(`${this.suburl}/api/v1/users/${userName}/heatmap`, (chartRawData) => {
          const chartData = [];
          for (let i = 0; i < chartRawData.length; i++) {
            self.totalContributions += chartRawData[i].contributions;
            chartData[i] = { date: new Date(chartRawData[i].timestamp * 1000), count: chartRawData[i].contributions };
          }
          self.values = chartData;
          self.isLoading = false;
        });
      },

      getColor(idx) {
        const el = document.createElement('div');
        el.className = `heatmap-color-${idx}`;
        document.body.appendChild(el);

        const color = getComputedStyle(el).backgroundColor;

        document.body.removeChild(el);

        return color;
      }
    },

    template: '<div><div v-show="isLoading"><slot name="loading"></slot></div><h4 class="total-contributions" v-if="!isLoading"><span v-html="totalContributions"></span> total contributions in the last 12 months</h4><calendar-heatmap v-show="!isLoading" :locale="locale" :no-data-text="locale.no_contributions" :tooltip-unit="locale.contributions" :end-date="endDate" :values="values" :range-color="colorRange"/></div>'
  });

  new Vue({
    delimiters: vueDelimeters,
    el,

    data: {
      suburl: AppSubUrl,
      heatmapUser,
      locale: {
        contributions: 'contributions',
        no_contributions: 'No contributions',
      },
    },
  });
}
