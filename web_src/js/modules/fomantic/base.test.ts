import {createElementFromHTML} from '../../utils/dom.ts';
import {initAriaLabels} from './base.ts';

test('aria-labels-link-textarea', () => {
  const form = createElementFromHTML(`<div class="ui form">
    <div class="field"><label>Title</label><input name="title"></div>
    <div class="field"><label>Content</label><textarea name="content"></textarea></div>
  </div>`);
  initAriaLabels(form);

  for (const field of form.querySelectorAll('.field')) {
    const control = field.querySelector<HTMLElement>('input, textarea')!;
    expect(control.id).toBeTruthy();
    expect(field.querySelector('label')!.getAttribute('for')).toEqual(control.id);
  }
});
