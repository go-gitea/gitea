<template>
  <div class="build-view-container">
    <div class="build-view-left">
      <div class="build-info-summary">{{ buildInfo.title }}</div>

      <div class="job-group-section" v-for="(jobGroup, i) in allJobGroups" :key="i">
        <div class="job-group-summary">
          {{ jobGroup.summary }}
        </div>
        <div class="job-brief-list">
          <a class="job-brief-item" v-for="job in jobGroup.jobs" :key="job.id">
            <SvgIcon name="octicon-check-circle-fill"/> {{ job.name }}
          </a>
        </div>
      </div>
    </div>

    <div class="build-view-right">
      <div class="job-info-header">
        <div class="job-info-header-title">
          {{ currentJobInfo.title }}
        </div>
        <div class="job-info-header-detail">
          {{ currentJobInfo.detail }}
        </div>
      </div>
      <div class="job-stage-container">
        <div class="job-stage-section" v-for="(jobStage, i) in currentJobStages" :key="i">
          <div class="job-stage-summary" @click.stop="toggleStageLogs(i)">
            <SvgIcon name="octicon-chevron-down" v-show="currentJobStagesStates[i].expanded"/>
            <SvgIcon name="octicon-chevron-right" v-show="!currentJobStagesStates[i].expanded"/>

            <SvgIcon name="octicon-check-circle-fill"/>

            {{ jobStage.summary }}
          </div>

          <!-- the log elements could be a lot, do not use v-if to destroy/reconstruct the DOM -->
          <div class="job-stage-logs" ref="elJobStageLogs" v-show="currentJobStagesStates[i].expanded">
            <!--
            <div class="job-log-group">
              <div class="job-log-group-summary"></div>
              <div class="job-log-list">
                <div class="job-log-line"></div>
              </div>
            </div>
            -->

            <!--
            <div class="job-log-line"></div>
            -->
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import {SvgIcon} from '../svg.js';
import Vue from 'vue';

const sfc = {
  name: 'RepoBuildView',
  components: {
    SvgIcon,
  },

  data() {
    return {
      // internal state
      loading: false,
      currentJobStagesStates: [],

      // provided by backend
      buildInfo: {},
      allJobGroups: [],
      currentJobInfo: {},
      currentJobStages: [],
    };
  },

  mounted() {
    const elBodyDiv = document.querySelector('body > div.full.height');
    elBodyDiv.style.height = '100%';
    this.loadJobData();
  },

  methods: {
    stageLogsGetActiveContainer(idx) {
      const el = this.$refs.elJobStageLogs[idx];
      return el._stageLogsActiveContainer ?? el;
    },
    stageLogsGroupBegin(idx) {
      const el = this.$refs.elJobStageLogs[idx];

      const elJobLogGroup = document.createElement('div');
      elJobLogGroup.classList.add('job-log-group');

      const elJobLogGroupSummary = document.createElement('div');
      elJobLogGroupSummary.classList.add('job-log-group-summary');

      const elJobLogList = document.createElement('div');
      elJobLogList.classList.add('job-log-list');

      elJobLogGroup.appendChild(elJobLogGroupSummary);
      elJobLogGroup.appendChild(elJobLogList);
      el._stageLogsActiveContainer = elJobLogList;
    },
    stageLogsGroupEnd(idx) {
      const el = this.$refs.elJobStageLogs[idx];
      el._stageLogsActiveContainer = null;
    },

    toggleStageLogs(idx) {
      this.currentJobStagesStates[idx].expanded = !this.currentJobStagesStates[idx].expanded;
    },

    createLogLine(msg, time) {
      const el = document.createElement('div');
      el.classList.add('job-log-line');
      el.innerText = msg;
      el._jobLogTime = time;
      return el;
    },

    appendLogs(stageIndex, logLines) {
      for (const line of logLines) {
        // group: ##[group]GroupTItle , ##[endgroup]
        const el = this.stageLogsGetActiveContainer(stageIndex);
        el.append(this.createLogLine(line.m, line.t));
      }
    },

    fetchMockData(reqData) {
      const stateData = {
        buildInfo: {title: 'The Demo Build'},
        allJobGroups: [
          {summary: 'Job Group Foo', jobs: [{id: 1, name: 'Job A'}, {id: 2, name: 'Job B'}]},
          {summary: 'Job Group Bar', jobs: [{id: 3, name: 'Job X'}, {id: 4, name: 'Job Y'}]},
        ],
        currentJobInfo: {title: 'the job title', detail: ' succeeded 3 hours ago in 11s'},
        currentJobStages: [
          {summary: 'Job Stage 1'},
          {summary: 'Job Stage 2'},
        ],
      };
      const logsData = {streamingLogs: []};

      for (const reqCursor of reqData.stageLogCursors) {
        if (!reqCursor.expanded) continue;
        // if (reqCursor.cursor > 100) continue;
        const stageIndex = reqCursor.stageIndex;
        let cursor = reqCursor.cursor;
        const lines = [];
        for (let i = 0; i < 110; i++) {
          lines.push({m: `hello world ${Date.now()}, cursor: ${cursor}`, t: Date.now()});
          cursor++;
        }
        logsData.streamingLogs.push({stageIndex, cursor, lines});
      }
      return {stateData, logsData};
    },

    async fetchJobData(reqData) {
      const resp = await fetch(`?job_id=${this.jobId}`, {method: 'POST', body: JSON.stringify(reqData)});
      return await resp.json();
    },

    async loadJobData() {
      try {
        if (this.loading) return;
        this.loading = true;

        const stageLogCursors = this.currentJobStagesStates.map((it, idx) => {return {stageIndex: idx, cursor: it.cursor, expanded: it.expanded}});
        const reqData = {stageLogCursors};

        // const data = await this.fetchJobData();
        const data = this.fetchMockData(reqData);

        console.log('loadJobData', data);

        for (const [key, value] of Object.entries(data.stateData)) {
          this[key] = value;
        }
        for (let i = 0; i < this.currentJobStages.length; i++) {
          if (!this.currentJobStagesStates[i]) {
            this.$set(this.currentJobStagesStates, i, {cursor: null, expanded: false});
          }
        }
        for (const [_, logs] of data.logsData.streamingLogs.entries()) {
          this.currentJobStagesStates[logs.stageIndex].cursor = logs.cursor;
          this.appendLogs(logs.stageIndex, logs.lines);
        }
      } finally {
        this.loading = false;
        setTimeout(() => this.loadJobData(), 1000);
      }
    }
  },
};

export default sfc;

export function initRepositoryBuildView() {
  const el = document.getElementById('repo-build-view');
  if (!el) return;

  const View = Vue.extend({
    render: (createElement) => createElement(sfc),
  });
  new View().$mount(el);
}

</script>

<style scoped lang="less">

.build-view-container {
  display: flex;
  height: 100%;
}

// ================
// build view left

.build-view-left {
  width: 20%;
  overflow-y: scroll;
  margin-left: 10px;
}

.build-info-summary {
  font-size: 150%;
  margin: 5px 0;
}

.job-group-section {
  .job-group-summary {
  }

  .job-brief-list {
    a.job-brief-item {
      display: block;
      margin: 5px 0;
      padding: 5px;
      background: #f8f8f8;
      border-radius: 5px;
    }
  }
}



// ================
// build view right

.build-view-right {
  flex: 1;
  background-color: #262626;
  color: #d6d6d6;
  max-height: 100%;

  display: flex;
  flex-direction: column;
}

.job-info-header {
  .job-info-header-title {
    color: #fdfdfd;
    font-size: 150%;
    padding: 10px;
  }
  .job-info-header-detail {
    padding: 0 10px 10px;
    border-bottom: 1px solid #666;
  }
}

.job-stage-container {
  max-height: 100%;
  overflow: auto;

  .job-stage-summary {
    cursor: pointer;
    padding: 5px 0;
  }
  .job-stage-summary:hover {
    background-color: #333;
  }
}
</style>

<style lang="less">
// some elements are not managed by vue, so we need to use global style
.job-stage-section {
  margin: 10px;
  .job-stage-logs {
    .job-log-line {
      margin-left: 20px;
    }

    .job-log-group {
    }

    .job-log-group-summary {
    }

    .job-log-list {
    }
  }
}
</style>
