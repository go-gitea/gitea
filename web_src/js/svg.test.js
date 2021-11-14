import {svg, svgNode} from './svg.js';

test('svg', () => {
  expect(svg('octicon-repo')).toStartWith('<svg');
});

test('svgNode', () => {
  expect(svgNode('octicon-repo')).toBeInstanceOf(Element);
});
