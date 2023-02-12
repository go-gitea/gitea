<template>
  <div class="action-view-container">
    <div class="action-view-header">
      <div class="action-info-summary">
        {{ run.title }}
        <button class="run_cancel" @click="cancelRun()" v-if="run.canCancel">
          <i class="stop circle outline icon"/>
        </button>
      </div>
    </div>
    <div class="action-view-body">
      <div class="action-view-left">
        <div class="job-group-section">
          <div class="job-brief-list">
            <div class="job-brief-item" v-for="(job, index) in run.jobs" :key="job.id">
              <a class="job-brief-link" :href="run.link+'/jobs/'+index">
                <SvgIcon name="octicon-check-circle-fill" class="green" v-if="job.status === 'success'"/>
                <SvgIcon name="octicon-skip" class="ui text grey" v-else-if="job.status === 'skipped'"/>
                <SvgIcon name="octicon-clock" class="ui text yellow" v-else-if="job.status === 'waiting'"/>
                <SvgIcon name="octicon-blocked" class="ui text yellow" v-else-if="job.status === 'blocked'"/>
                <SvgIcon name="octicon-meter" class="ui text yellow" class-name="job-status-rotate" v-else-if="job.status === 'running'"/>
                <SvgIcon name="octicon-x-circle-fill" class="red" v-else/>
                <span class="ui text">{{ job.name }}</span>
              </a>
              <button class="job-brief-rerun" @click="rerunJob(index)" v-if="job.canRerun">
                <SvgIcon name="octicon-sync" class="ui text black"/>
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="action-view-right">
        <div class="job-info-header">
          <div class="job-info-header-title">
            {{ currentJob.title }}
          </div>
          <div class="job-info-header-detail">
            {{ currentJob.detail }}
          </div>
        </div>
        <div class="job-step-container">
          <div class="job-step-section" v-for="(jobStep, i) in currentJob.steps" :key="i">
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
            <div class="job-step-logs" ref="logs" v-show="currentJobStepsStates[i].expanded"/>
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
    runIndex: String,
    jobIndex: String,
    actionsURL: String,
  },

  data() {
    return {
      ansiToHTML: new AnsiToHTML({escapeXML: true}),

      // internal state
      loading: false,
      intervalID: null,
      currentJobStepsStates: [],

      // provided by backend
      run: {
        link: '',
        title: '',
        canCancel: false,
        done: false,
        jobs: [
          // {
          //   id: 0,
          //   name: '',
          //   status: '',
          //   canRerun: false,
          // },
        ],
      },
      currentJob: {
        title: '',
        detail: '',
        steps: [
          // {
          //   summary: '',
          //   duration: '',
          //   status: '',
          // }
        ],
      },
    };
  },

  mounted() {
    // load job data and then auto-reload periodically
    this.loadJob();
    this.intervalID = setInterval(this.loadJob, 1000);
  },

  methods: {
    // get the active container element, either the `job-step-logs` or the `job-log-list` in the `job-log-group`
    getLogsContainer(idx) {
      const el = this.$refs.logs[idx];
      return el._stepLogsActiveContainer ?? el;
    },
    // begin a log group
    beginLogGroup(idx) {
      const el = this.$refs.logs[idx];

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
    endLogGroup(idx) {
      const el = this.$refs.logs[idx];
      el._stepLogsActiveContainer = null;
    },

    // show/hide the step logs for a step
    toggleStepLogs(idx) {
      this.currentJobStepsStates[idx].expanded = !this.currentJobStepsStates[idx].expanded;
      if (this.currentJobStepsStates[idx].expanded) {
        this.loadJob(); // try to load the data immediately instead of waiting for next timer interval
      }
    },
    // rerun a job
    async rerunJob(idx) {
      const jobLink = `${this.run.link}/jobs/${idx}`;
      await this.fetchPost(`${jobLink}/rerun`);
      window.location.href = jobLink;
    },
    // cancel a run
    cancelRun() {
      this.fetchPost(`${this.run.link}/cancel`);
    },

    createLogLine(line) {
      const div = document.createElement('div');
      div.classList.add('job-log-line');
      div._jobLogTime = line.timestamp;

      const lineNumber = document.createElement('div');
      lineNumber.className = 'line-num';
      lineNumber.innerText = line.index;
      div.appendChild(lineNumber);

      // TODO: Support displaying time optionally

      const logMessage = document.createElement('div');
      logMessage.className = 'log-msg';
      logMessage.innerHTML = this.ansiToHTML.toHtml(line.message);
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

    async fetchJob() {
      const logCursors = this.currentJobStepsStates.map((it, idx) => {
        // cursor is used to indicate the last position of the logs
        // it's only used by backend, frontend just reads it and passes it back, it and can be any type.
        // for example: make cursor=null means the first time to fetch logs, cursor=eof means no more logs, etc
        return {step: idx, cursor: it.cursor, expanded: it.expanded};
      });
      const resp = await this.fetchPost(
        `${this.actionsURL}/runs/${this.runIndex}/jobs/${this.jobIndex}`,
        JSON.stringify({logCursors}),
      );
      return await resp.json();
    },

    async loadJob() {
      if (this.loading) return;
      try {
        this.loading = true;

        const response = await this.fetchJob();

        // save the state to Vue data, then the UI will be updated
        this.run = response.state.run;
        this.currentJob = response.state.currentJob;

        // sync the currentJobStepsStates to store the job step states
        for (let i = 0; i < this.currentJob.steps.length; i++) {
          if (!this.currentJobStepsStates[i]) {
            this.currentJobStepsStates[i] = {cursor: null, expanded: false};
          }
        }
        // append logs to the UI
        for (const logs of response.logs.stepsLog) {
          // save the cursor, it will be passed to backend next time
          this.currentJobStepsStates[logs.step].cursor = logs.cursor;
          this.appendLogs(logs.step, logs.lines);
        }

        if (this.run.done && this.intervalID) {
          clearInterval(this.intervalID);
          this.intervalID = null;
        }
      } finally {
        this.loading = false;
      }
    },

    fetchPost(url, body) {
      return fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Csrf-Token': csrfToken,
        },
        body,
      });
    },
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
  margin: 0 20px 20px 20px;
  .run_cancel {
    border: none;
    color: var(--color-red);
    background-color: transparent;
    outline: none;
    cursor: pointer;
    transition:transform 0.2s;
  };
  .run_cancel:hover{
    transform:scale(130%);
  };
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
    .job-brief-item {
      margin: 5px 0;
      padding: 10px;
      background: var(--color-info-bg);
      border-radius: 5px;
      text-decoration: none;
      display: flex;
      justify-items: center;
      flex-wrap: nowrap;
      .job-brief-rerun {
        float: right;
        border: none;
        background-color: transparent;
        outline: none;
        cursor: pointer;
        transition:transform 0.2s;
      };
      .job-brief-rerun:hover{
        transform:scale(130%);
      };
      .job-brief-link {
        flex-grow: 1;
        display: flex;
        span {
          margin-right: 8px;
          display: flex;
          align-items: center;
        }
      }
    }
    .job-brief-item:hover {
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

