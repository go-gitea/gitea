import {createApp, h, nextTick, ref} from 'vue';
import RepoRecentCommits from './RepoRecentCommits.vue';

const {chart} = vi.hoisted(() => ({chart: {renders: 0}}));

vi.mock('./ChartCanvas.vue', () => ({
  default: () => {
    chart.renders++;
    return h('canvas');
  },
}));

vi.mock('../modules/fetch.ts', () => ({
  GET: async () => {
    const week = Date.UTC(2026, 0, 4); // the api returns sundays
    return Response.json({[week]: {week, commits: 3, additions: 2, deletions: 1}});
  },
}));

test('chart data survives an unrelated re-render', async () => {
  window.config.pageData.repoLink = '/user/repo';
  const cssClass = ref('first');
  const locale = {loadingTitle: '', loadingTitleFailed: '', loadingInfo: ''};
  createApp({
    render: () => h(RepoRecentCommits, {locale, class: cssClass.value}),
  }).mount(document.createElement('div'));
  await vi.waitUntil(() => chart.renders);
  cssClass.value = 'second';
  await nextTick();
  expect(chart.renders).toEqual(1); // a new data object here would rebuild the chart
});
