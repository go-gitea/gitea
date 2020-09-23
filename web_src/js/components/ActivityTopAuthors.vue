<template>
    <div>
        <div class="activity-bar-graph" ref="style" style="width:0px;height:0px"></div>
        <div class="activity-bar-graph-alt" ref="altStyle" style="width:0px;height:0px"></div>
        <vue-bar-graph
            :points="graphData"
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
            <template v-slot:label="opt">
                <g v-for="(author, idx) in authors" :key="author.position">
                    <a
                        v-if="opt.bar.index === idx && author.home_link !== ''"
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
            <template v-slot:title="opt">
                <tspan v-for="(author, idx) in authors" :key="author.position"><tspan v-if="opt.bar.index === idx">{{ author.name }}</tspan></tspan>
            </template>
        </vue-bar-graph>
    </div>
</template>

<script>
import VueBarGraph from 'vue-bar-graph';

export default {
    components: {
        VueBarGraph,
    },
    props: {
        data: { type: Array, default: () => [] },
    },
    mounted() {
        const st = window.getComputedStyle(this.$refs.style);
        const stalt = window.getComputedStyle(this.$refs.altStyle);

        this.colors.barColor = st.backgroundColor;
        this.colors.textColor = st.color;
        this.colors.textAltColor = stalt.color;
    },
    data() {
        return {
            colors: {
                barColor: 'green',
                textColor: 'black',
                textAltColor: 'white',
            },
        };
    },
    computed: {
        graphData() {
            return this.data.map((item) => {
                return {
                    value: item.commits,
                    label: item.name,
                };
            });
        },
        authors() {
            return this.data.map((item, idx) => {
                return {
                    position: idx+1,
                    ...item,
                }
            });
        },
        graphWidth() {
            return this.data.length * 40;
        },
    },
    methods: {
        hasHomeLink(i) {
            return this.graphData[i].homeLink !== '' && this.graphData[i].homeLink !== null;
        },
    }
}
</script>