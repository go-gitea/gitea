import {svg, svgNode} from './svg.js';

test('svg', () => {
  expect(svg('octicon-repo')).toStartWith('<svg');
});

test('svgNode', () => {
  expect(svgNode('octicon-repo')).toBeInstanceOf(Element);
  expect(svgNode('octicon-repo', 16).getAttribute('width')).toEqual('16');
  expect(svgNode('octicon-repo', 32).getAttribute('width')).toEqual('32');
});
