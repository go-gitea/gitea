import {createApp, h, isReactive, nextTick, reactive, shallowRef} from 'vue';
import ChartCanvas from './ChartCanvas.vue';

const {charts, dateAdapter} = vi.hoisted(() => ({charts: [] as Array<Record<string, any>>, dateAdapter: {} as Record<string, any>}));

vi.mock('chart.js', () => {
  class Chart {
    static defaults: Record<string, any> = {};
    static register() {}
    data: unknown;
    update = vi.fn();
    destroy = vi.fn();
    constructor(_canvas: unknown, config: Record<string, any>) {
      this.data = config.data;
      charts.push(this);
    }
  }
  return {Chart, BarController: {}, LineController: {}, LinearScale: {}, TimeScale: {}, _adapters: {_date: {override: (a: any) => Object.assign(dateAdapter, a)}}};
});

test('ChartCanvas', async () => {
  const data = shallowRef(reactive({datasets: []}));
  const options = {};
  const app = createApp({render: () => h(ChartCanvas, {type: 'line', data: data.value, options})});
  app.mount(document.createElement('div'));
  expect(charts).toHaveLength(1);
  expect(isReactive(charts[0].data)).toBe(false); // chart.js mutates it
  const time = Date.UTC(2026, 4, 15); // the date adapter registers during setup
  expect(dateAdapter.format(time, dateAdapter.formats().quarter)).toEqual('Q2 - 2026');
  expect(dateAdapter.format(dateAdapter.startOf(time, 'quarter'), 'MMM YYYY')).toEqual('Apr 2026');
  data.value = reactive({datasets: []});
  await nextTick();
  expect(charts[0].update).toHaveBeenCalledOnce();
  app.unmount();
  expect(charts[0].destroy).toHaveBeenCalledOnce();
});
