import {createApp} from 'vue';

// convertName convert the html tag a-b to aB
export function convertName(o) {
  return o.replace(/-(\w)/g, (_, c) => {
    return c ? c.toUpperCase() : '';
  });
}

// initComponent will mount the component with tag id named id and vue sfc
// it will also assign all attributes of the tag with the prefix data-locale- and data-
// to the component as props
export function initComponent(id, sfc) {
  const el = document.getElementById(id);
  if (!el) return;

  const data = {};

  for (const attr of el.getAttributeNames()) {
    if (attr.startsWith('data-locale-')) {
      data.locale = data.locale || {};
      data.locale[convertName(attr.slice(12))] = el.getAttribute(attr);
    } else if (attr.startsWith('data-')) {
      data[convertName(attr.slice(5))] = el.getAttribute(attr);
    }
  }

  if (!sfc.props.locale) {
    sfc.props.locale = {
      type: Object,
      default: () => {},
    };
  }

  const view = createApp(sfc, data);
  view.mount(el);
}
