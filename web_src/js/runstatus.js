import {svg} from './svg.js';

// retrieve a HTML string for given run status, size and additional classes
function runstatus(status, size = 16, className = '') {
  switch (status) {
    case 'success':
      return svg('octicon-check-circle-fill', size, className);
    case 'skipped':
      return svg('octicon-skip', size, className);
    case 'waiting':
      return svg('octicon-clock', size, className);
    case 'blocked':
      return svg('octicon-blocked', size, className);
    case 'running':
      return svg('octicon-meter', size, className);
    default:
      return svg('octicon-x-circle-fill', size, className);
  }
}

function spanclass(status) {
  switch (status) {
    case 'success':
      return 'green';
    case 'skipped':
      return 'ui text grey';
    case 'waiting':
      return 'ui text yellow';
    case 'blocked':
      return 'ui text yellow';
    case 'running':
      return 'ui text yellow';
    default:
      return 'red';
  }
}

export const RunStatus = {
  name: 'RunStatus',
  props: {
    status: {type: String, required: true},
    size: {type: Number, default: 16},
    className: {type: String, default: ''},
  },

  computed: {
    runstatus() {
      return runstatus(this.status, this.size, this.className);
    },
    spanclass() {
      return spanclass(this.status);
    }
  },

  template: `<span v-html="runstatus" :class="spanclass"/>`
};
