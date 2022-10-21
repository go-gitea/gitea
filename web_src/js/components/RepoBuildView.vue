<template>
  <div class="build-view-container">
    <div class="build-view-left">
      <div class="build-info-summary">
        {{ buildInfo.title }}
      </div>

      <div class="job-group-section" v-for="(jobGroup, i) in allJobGroups" :key="i">
        <div class="job-group-summary">
          {{ jobGroup.summary }}
        </div>
        <div class="job-brief-list">
          <a class="job-brief-item" v-for="(job, index) in jobGroup.jobs" :key="job.id" v-bind:href="buildInfo.htmlurl+'/jobs/'+index">
            <SvgIcon name="octicon-check-circle-fill" class="green" v-if="job.status === 'success'"/>
            <SvgIcon name="octicon-skip" class="ui text grey" v-else-if="job.status === 'skipped'"/>
            <SvgIcon name="octicon-clock" class="ui text yellow" v-else-if="job.status === 'waiting'"/>
            <SvgIcon name="octicon-meter" class="ui text yellow" class-name="job-status-rotate" v-else-if="job.status === 'running'"/>
            <SvgIcon name="octicon-x-circle-fill" class="red" v-else/>
            {{ job.name }}
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
      <div class="job-step-container">
        <div class="job-step-section" v-for="(jobStep, i) in currentJobSteps" :key="i">
          <div class="job-step-summary" @click.stop="toggleStepLogs(i)">
            <SvgIcon name="octicon-chevron-down" class="mr-3" v-show="currentJobStepsStates[i].expanded"/>
            <SvgIcon name="octicon-chevron-right" class="mr-3" v-show="!currentJobStepsStates[i].expanded"/>

            <SvgIcon name="octicon-check-circle-fill" class="green mr-3" v-if="jobStep.status === 'success'"/>
            <SvgIcon name="octicon-skip" class="ui text grey mr-3" v-else-if="jobStep.status === 'skipped'"/>
            <SvgIcon name="octicon-clock" class="ui text yellow mr-3" v-else-if="jobStep.status === 'waiting'"/>
            <SvgIcon name="octicon-meter" class="ui text yellow mr-3" class-name="job-status-rotate" v-else-if="jobStep.status === 'running'"/>
            <SvgIcon name="octicon-x-circle-fill" class="red mr-3 " v-else/>

            <span class="step-summary-msg">{{ jobStep.summary }}</span>
            <span class="step-summary-dur">{{ formatDuration(jobStep.duration) }}</span>
          </div>

          <!-- the log elements could be a lot, do not use v-if to destroy/reconstruct the DOM -->
          <div class="job-step-logs" ref="elJobStepLogs" v-show="currentJobStepsStates[i].expanded">
            <!--
            possible layouts:
            <div class="job-log-group">
              <div class="job-log-group-summary"></div>
              <div class="job-log-list">
                <div class="job-log-line"></div>
              </div>
            </div>
            -- or --
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
import Vue, {createApp} from 'vue';
import AnsiToHTML from `ansi-to-html`;

const {csrfToken} = window.config;

const sfc = {
  name: 'RepoBuildView',
  components: {
    SvgIcon,
  },
  props: {
    runIndex: Number,
    jobIndex: Number,
  },

  data() {
    return {
      // internal state
      loading: false,
      currentJobStepsStates: [],

      // provided by backend
      buildInfo: {},
      allJobGroups: [],
      currentJobInfo: {},
      currentJobSteps: [],
    };
  },

  created() {
    this.ansiToHTML = new AnsiToHTML({escapeXML: true});
  },

  mounted() {
    // load job data and then auto-reload periodically
    this.loadJobData();
    setInterval(() => this.loadJobData(), 1000);
  },

  methods: {
    // get the active container element, either the `job-step-logs` or the `job-log-list` in the `job-log-group`
    stepLogsGetActiveContainer(idx) {
      const el = this.$refs.elJobStepLogs[idx];
      return el._stepLogsActiveContainer ?? el;
    },
    // begin a log group
    stepLogsGroupBegin(idx) {
      const el = this.$refs.elJobStepLogs[idx];

      const elJobLogGroup = document.createElement('div');
      elJobLogGroup.classList.add('job-log-group');

      const elJobLogGroupSummary = document.createElement('div');
      elJobLogGroupSummary.classList.add('job-log-group-summary');

      const elJobLogList = document.createElement('div');
      elJobLogList.classList.add('job-log-list');

      elJobLogGroup.appendChild(elJobLogGroupSummary);
      elJobLogGroup.appendChild(elJobLogList);
      el._stepLogsActiveContainer = elJobLogList;
    },
    // end a log group
    stepLogsGroupEnd(idx) {
      const el = this.$refs.elJobStepLogs[idx];
      el._stepLogsActiveContainer = null;
    },

    // show/hide the step logs for a step
    toggleStepLogs(idx) {
      this.currentJobStepsStates[idx].expanded = !this.currentJobStepsStates[idx].expanded;
      if (this.currentJobStepsStates[idx].expanded) {
        this.loadJobData(); // try to load the data immediately instead of waiting for next timer interval
      }
    },

    formatDuration(d) {
      d = Math.round(d);
      const unitValues = [60, 60, 24];
      const unitNames = ['s', 'm', 'h', 'd'];
      const parts = [];
      for (let i = 0; i < unitValues.length; i++) {
        parts[i] = d % unitValues[i];
        d = Math.floor(d / unitValues[i]);
      }
      parts.push(d);
      let res = '', resCount = 0;
      for (let i = parts.length - 1; i >= 0 && resCount < 2; i--) {
        if (parts[i] > 0) {
          res += parts[i] + unitNames[i] + ' ';
          resCount++;
        }
      }
      if (!res) return '0s';
      return res.substring(0, res.length - 1);
    },

    createLogLine(line) {
      const el = document.createElement('div');
      el.classList.add('job-log-line');
      el._jobLogTime = line.t;

      const elLineNum = document.createElement('line-num');
      elLineNum.innerText = line.ln;
      el.appendChild(elLineNum);

      const elLogTime = document.createElement('log-time');
      elLogTime.innerText = new Date(line.t*1000).toISOString();
      el.appendChild(elLogTime);

      const elLogMsg = document.createElement('log-msg');
      elLogMsg.innerHTML = this.ansiToHTML.toHtml(line.m);
      el.appendChild(elLogMsg);

      return el;
    },

    appendLogs(stepIndex, logLines) {
      for (const line of logLines) {
        // TODO: group support: ##[group]GroupTitle , ##[endgroup]
        const el = this.stepLogsGetActiveContainer(stepIndex);
        el.append(this.createLogLine(line));
      }
    },

    // the respData has the following fields:
    // * stateData: it will be stored into Vue data and used to update the UI state
    // * logsData: the logs in it will be appended to the UI manually, no touch to Vue data
    fetchMockData(reqData) {
      const stateData = {
        buildInfo: {title: 'The Demo Build'},
        allJobGroups: [
          {
            summary: 'Job Group Foo',
            jobs: [
              {id: 1, name: 'Job A', status: 'success'},
              {id: 2, name: 'Job B', status: 'error'},
            ],
          },
          {
            summary: 'Job Group Bar',
            jobs: [
              {id: 3, name: 'Job X', status: 'skipped'},
              {id: 4, name: 'Job Y', status: 'waiting'},
              {id: 5, name: 'Job Z', status: 'running'},
            ],
          },
        ],
        currentJobInfo: {title: 'the job title', detail: 'succeeded 3 hours ago in 11s'},
        currentJobSteps: [
          {summary: 'Job Step 1', duration: 0.5, status: 'success'},
          {summary: 'Job Step 2', duration: 2, status: 'error'},
          {summary: 'Job Step 3', duration: 64, status: 'skipped'},
          {summary: 'Job Step 4', duration: 3600 + 60, status: 'waiting'},
          {summary: 'Job Step 5', duration: 86400 + 60 + 1, status: 'running'},
        ],
      };
      const logsData = {
        streamingLogs: [
          // {stepIndex: 0, lines: [{t: timestamp, ln: lineNumber, m: message}, ...]},
        ]
      };

      // prepare mock data for logs
      for (const reqCursor of reqData.stepLogCursors) {
        if (!reqCursor.expanded) continue; // backend can decide whether send logs for a step
        if (reqCursor.cursor > 100) continue;
        let cursor = reqCursor.cursor; // use cursor to send remaining logs
        const lines = [];
        for (let i = 0; i < 110; i++) {
          lines.push({
            ln: cursor, // demo only, use cursor for line number
            m: ' '.repeat(i % 4) + `\x1B[1;3;31mDemo Log\x1B[0m, tag test <br>, hello world ${Date.now()}, cursor: ${cursor}`,
            t: Date.now()/1000, // use second as unit
          });
          cursor++;
        }
        logsData.streamingLogs.push({stepIndex: reqCursor.stepIndex, cursor, lines});
      }

      return {stateData, logsData};
    },

    async fetchJobData(reqData) {
      const resp = await fetch(``, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Csrf-Token': csrfToken,
        },
        body: JSON.stringify(reqData),
      });
      return await resp.json();
    },

    async loadJobData() {
      if (this.loading) return;
      try {
        this.loading = true;

        const stepLogCursors = this.currentJobStepsStates.map((it, idx) => {
          // cursor is used to indicate the last position of the logs
          // it's only used by backend, frontend just reads it and passes it back, it and can be any type.
          // for example: make cursor=null means the first time to fetch logs, cursor=eof means no more logs, etc
          return {stepIndex: idx, cursor: it.cursor, expanded: it.expanded};
        });
        const reqData = {stepLogCursors};

        const respData = await this.fetchJobData(reqData);
        // const respData = this.fetchMockData(reqData);

        // console.log('loadJobData by request', reqData, ', get response ', respData);

        // save the stateData to Vue data, then the UI will be updated
        for (const [key, value] of Object.entries(respData.stateData)) {
          this[key] = value;
        }

        // sync the currentJobStepsStates to store the job step states
        for (let i = 0; i < this.currentJobSteps.length; i++) {
          if (!this.currentJobStepsStates[i]) {
            this.currentJobStepsStates[i] = {cursor: null, expanded: false};
          }
        }
        // append logs to the UI
        for (const logs of respData.logsData.streamingLogs) {
          // save the cursor, it will be passed to backend next time
          this.currentJobStepsStates[logs.stepIndex].cursor = logs.cursor;
          this.appendLogs(logs.stepIndex, logs.lines);
        }
      } finally {
        this.loading = false;
      }
    }
  },
};

export default sfc;

export function initRepositoryBuildView() {
  const el = document.getElementById('repo-build-view');
  if (!el) return;

  const view = createApp(sfc, {
    jobIndex: el.getAttribute("data-job-index"),
    runIndex: el.getAttribute("data-run-index"),
  });
  view.mount(el);
}

</script>

<style scoped lang="less">

.build-view-container {
  display: flex;
  height: calc(100vh - 286px); // fine tune this value to make the main view has full height
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
    margin: 5px 0;
    padding: 10px;
  }

  .job-brief-list {
    a.job-brief-item {
      display: block;
      margin: 5px 0;
      padding: 10px;
      background: #f8f8f8;
      border-radius: 5px;
      text-decoration: none;
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

.job-step-container {
  max-height: 100%;
  overflow: auto;

  .job-step-summary {
    cursor: pointer;
    padding: 5px 10px;
    display: flex;

    .step-summary-msg {
      flex: 1;
    }
    .step-summary-dur {
      margin-left: 16px;
    }
  }
  .job-step-summary:hover {
    background-color: #333;
  }
}
</style>

<style lang="less">
// some elements are not managed by vue, so we need to use global style

// TODO: the parent element's full height doesn't work well now
body > div.full.height {
  padding-bottom: 0;
}

.job-status-rotate {
  animation: job-status-rotate-keyframes 1s linear infinite;
}
@keyframes job-status-rotate-keyframes {
  100% {
    transform:rotate(360deg);
  }
}

.job-step-section {
  margin: 10px;
  .job-step-logs {
    font-family: monospace, monospace;
    .job-log-line {
      display: flex;
      line-num {
        width: 48px;
        color: #555;
        text-align: right;
      }
      log-time {
        color: #777;
        margin-left: 10px;
      }
      log-msg {
        flex: 1;
        white-space: pre;
        margin-left: 10px;
      }
    }

    // TODO: group support
    .job-log-group {
    }

    .job-log-group-summary {
    }

    .job-log-list {
    }
  }
}
</style>
