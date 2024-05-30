import {createApp} from 'vue';

// create a new vue root and container and mount a component into it
export function createVueRoot(component, props) {
  const container = document.createElement('div');
  const view = createApp(component, props);
  try {
    view.mount(container);
    return container;
  } catch (err) {
    console.error(err);
    return null;
  }
}
