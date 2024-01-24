import {createApp} from 'vue';

function convertName(o) {
  return o.replace(/-(\w)/g, (_, c) => {
    return c ? c.toUpperCase() : '';
  });
}

export function initComponent(id, sfc) {
  const el = document.getElementById(id);
  if (!el) return;

  const data = {};

  el.getAttributeNames().forEach((attr) => {
    if (attr.startsWith('data-locale-')) {
      data.locale = data.locale || {};
      data.locale[convertName(attr.slice(12))] = el.getAttribute(attr);
    } else if (attr.startsWith('data-')) {
      data[convertName(attr.slice(5))] = el.getAttribute(attr);
    }
  });

  const view = createApp(sfc, data);
  view.mount(el);
}
