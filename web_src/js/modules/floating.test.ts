import {createFloatingElement, getFloatingElement} from './floating.ts';
import {sleep} from '../utils.ts';

function makeTarget(): HTMLElement {
  const el = document.createElement('button');
  document.body.append(el);
  return el;
}

describe.sequential('floating', () => {
  beforeEach(() => { document.body.innerHTML = '' });

  test('register', () => {
    const target = makeTarget();
    const instance = createFloatingElement(target, {trigger: 'manual'});
    expect(getFloatingElement(target)).toBe(instance);
    instance.destroy();
    expect(getFloatingElement(target)).toBeNull();
  });

  test('show/hide', () => {
    const instance = createFloatingElement(makeTarget(), {content: 'x', trigger: 'manual'});
    instance.show();
    expect(instance.state.isShown).toBe(true);
    expect(instance.element.parentElement).toBe(document.body);
    instance.hide();
    expect(instance.state.isShown).toBe(false);
    expect(instance.element.parentElement).toBeNull();
    instance.destroy();
  });

  test('setContent', () => {
    const instance = createFloatingElement(makeTarget(), {content: 'a', trigger: 'manual'});
    instance.setContent('b');
    expect(instance.element.querySelector('.floating-content')!.textContent).toBe('b');
    instance.destroy();
  });

  test('hover delay', async () => {
    const target = makeTarget();
    const instance = createFloatingElement(target, {delay: [40, 0]});
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
    const instance = createFloatingElement(target, {trigger: 'click'});
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(true);
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(false);
    instance.destroy();
  });

  test('focus click', () => {
    const target = makeTarget();
    const instance = createFloatingElement(target, {trigger: 'focus click'});
    target.dispatchEvent(new FocusEvent('focus'));
    target.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(instance.state.isShown).toBe(true);
    instance.destroy();
  });

  test('interactive hover', async () => {
    const target = makeTarget();
    const instance = createFloatingElement(target, {interactive: true});
    target.dispatchEvent(new MouseEvent('mouseenter'));
    instance.element.dispatchEvent(new MouseEvent('mouseenter'));
    target.dispatchEvent(new MouseEvent('mouseleave'));
    expect(instance.state.isShown).toBe(true);
    instance.element.dispatchEvent(new MouseEvent('mouseleave'));
    await vi.waitFor(() => expect(instance.state.isShown).toBe(false));
    instance.destroy();
  });

  test('tooltip dedupe', () => {
    const a = createFloatingElement(makeTarget(), {trigger: 'manual', role: 'tooltip'});
    const b = createFloatingElement(makeTarget(), {trigger: 'manual', role: 'tooltip'});
    a.show();
    b.show();
    expect(a.state.isShown).toBe(false);
    expect(b.state.isShown).toBe(true);
    a.destroy();
    b.destroy();
  });
});
