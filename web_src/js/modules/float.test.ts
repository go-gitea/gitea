import {createFloat, getFloat} from './float.ts';
import {sleep} from '../utils.ts';

function makeTarget(): HTMLElement {
  const el = document.createElement('button');
  document.body.append(el);
  return el;
}

describe.sequential('float', () => {
  beforeEach(() => { document.body.innerHTML = '' });

  test('register', () => {
    const target = makeTarget();
    const instance = createFloat(target, {trigger: 'manual'});
    expect(getFloat(target)).toBe(instance);
    instance.destroy();
    expect(getFloat(target)).toBeNull();
  });

  test('show/hide', () => {
    const instance = createFloat(makeTarget(), {content: 'x', trigger: 'manual'});
    instance.show();
    expect(instance.state.isShown).toBe(true);
    expect(instance.float.parentElement).toBe(document.body);
    instance.hide();
    expect(instance.state.isShown).toBe(false);
    expect(instance.float.parentElement).toBeNull();
    instance.destroy();
  });

  test('setContent', () => {
    const instance = createFloat(makeTarget(), {content: 'a', trigger: 'manual'});
    instance.setContent('b');
    expect(instance.float.querySelector('.float-content')!.textContent).toBe('b');
    instance.destroy();
  });

  test('hover delay', async () => {
    const target = makeTarget();
    const instance = createFloat(target, {delay: [40, 0]});
    target.dispatchEvent(new MouseEvent('mouseenter'));
    expect(instance.state.isShown).toBe(false);
    await sleep(80);
    expect(instance.state.isShown).toBe(true);
    target.dispatchEvent(new MouseEvent('mouseleave'));
    expect(instance.state.isShown).toBe(false);
    instance.destroy();
  });

  test('click toggles', () => {
    const target = makeTarget();
    const instance = createFloat(target, {trigger: 'click'});
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(true);
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(false);
    instance.destroy();
  });

  test('focus click', () => {
    const target = makeTarget();
    const instance = createFloat(target, {trigger: 'focus click'});
    target.dispatchEvent(new FocusEvent('focus'));
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(true);
    instance.destroy();
  });

  test('interactive hover', () => {
    const target = makeTarget();
    const instance = createFloat(target, {interactive: true});
    target.dispatchEvent(new MouseEvent('mouseenter'));
    instance.float.dispatchEvent(new MouseEvent('mouseenter'));
    target.dispatchEvent(new MouseEvent('mouseleave'));
    expect(instance.state.isShown).toBe(true);
    instance.float.dispatchEvent(new MouseEvent('mouseleave'));
    expect(instance.state.isShown).toBe(false);
    instance.destroy();
  });

  test('tooltip dedupe', () => {
    const a = createFloat(makeTarget(), {trigger: 'manual', role: 'tooltip'});
    const b = createFloat(makeTarget(), {trigger: 'manual', role: 'tooltip'});
    a.show();
    b.show();
    expect(a.state.isShown).toBe(false);
    expect(b.state.isShown).toBe(true);
    a.destroy();
    b.destroy();
  });
});
