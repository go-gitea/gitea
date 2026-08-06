import {createApp, h, isReactive, nextTick, reactive, shallowRef} from 'vue';
import ChartCanvas from './ChartCanvas.vue';
import {_adapters} from 'chart.js';

const {charts} = vi.hoisted(() => ({charts: [] as Array<Record<string, any>>}));

// chart.js cannot initialize without a canvas context, which happy-dom does not provide
vi.mock('chart.js', async (importOriginal) => {
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
  return {...await importOriginal(), Chart};
});

test('ChartCanvas', async () => {
  const data = shallowRef(reactive({datasets: []}));
  const options = {};
  const app = createApp({render: () => h(ChartCanvas, {type: 'line', data: data.value, options})});
  app.mount(document.createElement('div'));
  expect(charts).toHaveLength(1);
  expect(isReactive(charts[0].data)).toBe(false); // chart.js mutates it
  const adapter = new (_adapters._date as any)({}); // the component registered it during setup
  const time = Date.UTC(2026, 4, 15);
  expect(adapter.format(time, adapter.formats().quarter)).toEqual('Q2 - 2026');
  expect(adapter.format(adapter.startOf(time, 'quarter'), 'MMM YYYY')).toEqual('Apr 2026');
  data.value = reactive({datasets: []});
  await nextTick();
  expect(charts[0].update).toHaveBeenCalledOnce();
  app.unmount();
  expect(charts[0].destroy).toHaveBeenCalledOnce();
});
