import {svg, svgNode} from './svg.js';

test('svg', () => {
  expect(svg('octicon-repo')).toStartWith('<svg');
});

test('svgNode', () => {
  expect(svgNode('octicon-repo')).toBeInstanceOf(Element);

  const node1 = svgNode('octicon-repo', 16);
  expect(node1.getAttribute('width')).toEqual('16');
  const node2 = svgNode('octicon-repo', 32);
  expect(node1.getAttribute('width')).toEqual('16');
  expect(node2.getAttribute('width')).toEqual('32');
  expect(node1).not.toEqual(node2);
  expect(node1.childNodes.length).toBeGreaterThan(0);
  expect(node2.childNodes.length).toBeGreaterThan(0);
});
