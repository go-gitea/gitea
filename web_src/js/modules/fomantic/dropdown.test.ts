import {createElementFromHTML} from '../../utils/dom.ts';
import {hideScopedEmptyDividers} from './dropdown.ts';

test('hideScopedEmptyDividers-simple', () => {
  const container = createElementFromHTML(`<div>
<div class="divider"></div>
<div class="item">a</div>
<div class="divider"></div>
<div class="divider"></div>
<div class="divider"></div>
<div class="item">b</div>
<div class="divider"></div>
</div>`);
  hideScopedEmptyDividers(container);
  expect(container.innerHTML).toEqual(`
<div class="divider hidden transition"></div>
<div class="item">a</div>
<div class="divider hidden transition"></div>
<div class="divider hidden transition"></div>
<div class="divider"></div>
<div class="item">b</div>
<div class="divider hidden transition"></div>
`);
});

test('hideScopedEmptyDividers-hidden1', () => {
  const container = createElementFromHTML(`<div>
<div class="item">a</div>
<div class="divider" data-scope="b"></div>
<div class="item tw-hidden" data-scope="b">b</div>
</div>`);
  hideScopedEmptyDividers(container);
  expect(container.innerHTML).toEqual(`
<div class="item">a</div>
<div class="divider hidden transition" data-scope="b"></div>
<div class="item tw-hidden" data-scope="b">b</div>
`);
});

test('hideScopedEmptyDividers-hidden2', () => {
  const container = createElementFromHTML(`<div>
<div class="item" data-scope="">a</div>
<div class="divider" data-scope="b"></div>
<div class="item tw-hidden" data-scope="b">b</div>
<div class="divider" data-scope=""></div>
<div class="item" data-scope="">c</div>
</div>`);
  hideScopedEmptyDividers(container);
  expect(container.innerHTML).toEqual(`
<div class="item" data-scope="">a</div>
<div class="divider hidden transition" data-scope="b"></div>
<div class="item tw-hidden" data-scope="b">b</div>
<div class="divider hidden transition" data-scope=""></div>
<div class="item" data-scope="">c</div>
`);
});
