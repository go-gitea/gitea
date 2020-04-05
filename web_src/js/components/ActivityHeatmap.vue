<template>
    <div>
        <div v-show="isLoading">
            <slot name="loading"></slot>
        </div>
        <h4 class="total-contributions" v-if="!isLoading">
            {{ totalContributions }} total contributions in the last 12 months
        </h4>
        <calendar-heatmap v-show="!isLoading" :locale="locale" :no-data-text="locale.no_contributions" :tooltip-unit="locale.contributions" :end-date="endDate" :values="values" :range-color="colorRange"/>
    </div>
</template>

<script>
import {CalendarHeatmap} from 'vue-calendar-heatmap';
const {AppSubUrl, heatmapUser} = window.config;

export default {
    name: "ActivityHeatmap",
    components: {
        CalendarHeatmap
    },
    data() {
        return {
            isLoading: true,
            colorRange: [],
            endDate: null,
            values: [],
            totalContributions: 0,
            suburl: AppSubUrl,
            user: heatmapUser,
            locale: {
                contributions: 'contributions',
                no_contributions: 'No contributions',
            },
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
                    chartData[i] = {date: new Date(chartRawData[i].timestamp * 1000), count: chartRawData[i].contributions};
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
}
</script>

<style scoped>

</style>
