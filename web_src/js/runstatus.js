import {svg} from './svg.js';

// retrieve a HTML string for given run status, size and additional classes
function runstatus(status, size = 16, className = '') {
  switch (status) {
    case 'success':
      return [
        svg('octicon-check-circle-fill', size, className),
        'green'
      ];
    case 'skipped':
      return [
        svg('octicon-skip', size, className),
        'ui text grey'
      ];
    case 'waiting':
      return [
        svg('octicon-clock', size, className),
        'ui text yellow'
      ];
    case 'blocked':
      return [
        svg('octicon-blocked', size, className),
        'ui text yellow'
      ];
    case 'running':
      return [
        svg('octicon-meter', size, className),
        'ui text yellow'
      ];
    default:
      return [
        svg('octicon-x-circle-fill', size, className),
        'red'
      ];
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
  },

  template: `<span v-html="runstatus[0]" :class="runstatus[1]"/>`
};
