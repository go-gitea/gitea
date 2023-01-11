<template>
  <div class="action-view-container">
    <div class="action-view-header">
      <div class="action-info-summary">
        {{ runInfo.title }}
      </div>
    </div>
    <div class="action-view-body">
      <div class="action-view-left">
        <div class="job-group-section" v-for="(jobGroup, i) in allJobGroups" :key="i">
          <div class="job-brief-list">
            <a class="job-brief-item" v-for="(job, index) in jobGroup.jobs" :key="job.id" :href="runInfo.htmlurl+'/jobs/'+index">
              <SvgIcon name="octicon-check-circle-fill" class="green" v-if="job.status === 'success'"/>
              <SvgIcon name="octicon-skip" class="ui text grey" v-else-if="job.status === 'skipped'"/>
              <SvgIcon name="octicon-clock" class="ui text yellow" v-else-if="job.status === 'waiting'"/>
              <SvgIcon name="octicon-blocked" class="ui text yellow" v-else-if="job.status === 'blocked'"/>
              <SvgIcon name="octicon-meter" class="ui text yellow" class-name="job-status-rotate" v-else-if="job.status === 'running'"/>
              <SvgIcon name="octicon-x-circle-fill" class="red" v-else/>
              {{ job.name }}
              <button class="job-brief-rerun" @click="rerunJob(index)" v-if="job.can_rerun">
                <SvgIcon name="octicon-sync" class="ui text black"/>
              </button>
            </a>
          </div>
          <button class="ui fluid tiny basic red button" @click="cancelRun()" v-if="runInfo.can_cancel">
            Cancel
          </button>
        </div>
      </div>

      <div class="action-view-right">
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
              <SvgIcon name="octicon-blocked" class="ui text yellow mr-3" v-else-if="jobStep.status === 'blocked'"/>
              <SvgIcon name="octicon-meter" class="ui text yellow mr-3" class-name="job-status-rotate" v-else-if="jobStep.status === 'running'"/>
              <SvgIcon name="octicon-x-circle-fill" class="red mr-3 " v-else/>

              <span class="step-summary-msg">{{ jobStep.summary }}</span>
              <span class="step-summary-dur">{{ jobStep.duration }}</span>
            </div>

            <!-- the log elements could be a lot, do not use v-if to destroy/reconstruct the DOM -->
            <div class="job-step-logs" ref="elJobStepLogs" v-show="currentJobStepsStates[i].expanded"/>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import {SvgIcon} from '../svg.js';
import {createApp} from 'vue';
import AnsiToHTML from 'ansi-to-html';

const {csrfToken} = window.config;

const sfc = {
  name: 'RepoActionView',
  components: {
    SvgIcon,
  },
  props: {
    runIndex: Number,
    jobIndex: Number,
    actionsURL: String,
  },

  data() {
    return {
      ansiToHTML: new AnsiToHTML({escapeXML: true}),

      // internal state
      loading: false,
      currentJobStepsStates: [],

      // provided by backend
      runInfo: {},
      allJobGroups: [],
      currentJobInfo: {},
      currentJobSteps: [],
    };
  },

  mounted() {
    // load job data and then auto-reload periodically
    this.loadJobData();
    setInterval(() => this.loadJobData(), 1000);
  },

  methods: {
    // get the active container element, either the `job-step-logs` or the `job-log-list` in the `job-log-group`
    getLogsContainer(idx) {
      const el = this.$refs.elJobStepLogs[idx];
      return el.logsContainer ?? el;
    },
    // begin a log group
    beginLogGroup(idx) {
      const el = this.$refs.elJobStepLogs[idx];

      const elJobLogGroup = document.createElement('div');
      elJobLogGroup.classList.add('job-log-group');

      const elJobLogGroupSummary = document.createElement('div');
      elJobLogGroupSummary.classList.add('job-log-group-summary');

      const elJobLogList = document.createElement('div');
      elJobLogList.classList.add('job-log-list');

      elJobLogGroup.appendChild(elJobLogGroupSummary);
      elJobLogGroup.appendChild(elJobLogList);
      el.logsContainer = elJobLogList;
    },
    // end a log group
    endLogGroup(idx) {
      const el = this.$refs.elJobStepLogs[idx];
      el.logsContainer = null;
    },

    // show/hide the step logs for a step
    toggleStepLogs(idx) {
      this.currentJobStepsStates[idx].expanded = !this.currentJobStepsStates[idx].expanded;
      if (this.currentJobStepsStates[idx].expanded) {
        this.loadJobData(); // try to load the data immediately instead of waiting for next timer interval
      }
    },
    // rerun a job
    rerunJob(idx) {
      fetch(`${this.runInfo.htmlurl}/jobs/${idx}/rerun`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Csrf-Token': csrfToken,
        },
        body: {},
      });
    },
    // cancel a run
    cancelRun() {
      fetch(`${this.runInfo.htmlurl}/cancel`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Csrf-Token': csrfToken,
        },
        body: {},
      });
    },

    createLogLine(line) {
      const div = document.createElement('div');
      div.classList.add('job-log-line');
      div._jobLogTime = line.t;

      const lineNumber = document.createElement('div');
      lineNumber.className = 'line-num';
      lineNumber.innerText = line.ln;
      div.appendChild(lineNumber);

      // TODO: Support displaying time optionally

      const logMessage = document.createElement('div');
      logMessage.className = 'log-msg';
      logMessage.innerHTML = this.ansiToHTML.toHtml(line.m);
      div.appendChild(logMessage);

      return div;
    },

    appendLogs(stepIndex, logLines) {
      for (const line of logLines) {
        // TODO: group support: ##[group]GroupTitle , ##[endgroup]
        const el = this.getLogsContainer(stepIndex);
        el.append(this.createLogLine(line));
      }
    },

    // the respData has the following fields:
    // * stateData: it will be stored into Vue data and used to update the UI state
    // * logsData: the logs in it will be appended to the UI manually, no touch to Vue data
    async fetchJobData(reqData) {
      const resp = await fetch(`${this.actionsURL}/runs/${this.runIndex}/jobs/${this.jobIndex}`, {
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

export function initRepositoryActionView() {
  const el = document.getElementById('repo-action-view');
  if (!el) return;

  const view = createApp(sfc, {
    runIndex: el.getAttribute('data-run-index'),
    jobIndex: el.getAttribute('data-job-index'),
    actionsURL: el.getAttribute('data-actions-url'),
  });
  view.mount(el);
}

</script>

<style scoped lang="less">

// some elements are not managed by vue, so we need to use _actions.less in addition.

.action-view-body {
  display: flex;
  height: calc(100vh - 266px); // fine tune this value to make the main view has full height
}

// ================
// action view header

.action-view-header {
  margin: 0 0 20px 20px;
}

.action-info-summary {
  font-size: 150%;
  height: 20px;
  padding: 0 10px;
}

// ================
// action view left

.action-view-left {
  width: 30%;
  max-width: 400px;
  overflow-y: scroll;
  margin-left: 10px;
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
      background: var(--color-info-bg);
      border-radius: 5px;
      text-decoration: none;
      button.job-brief-rerun {
        float: right;
        border: none;
        background-color: transparent;
        outline: none
      };
    }
    a.job-brief-item:hover {
      background-color: var(--color-secondary);
    }
  }
}

// ================
// action view right

.action-view-right {
  flex: 1;
  background-color: var(--color-console-bg);
  color: var(--color-console-fg);
  max-height: 100%;
  margin-right: 10px;

  display: flex;
  flex-direction: column;
}

.job-info-header {
  .job-info-header-title {
    font-size: 150%;
    padding: 10px;
  }
  .job-info-header-detail {
    padding: 0 10px 10px;
    border-bottom: 1px solid var(--color-grey);
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
    background-color: var(--color-black-light);
  }
}
</style>

