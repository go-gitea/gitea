import {mergeChildIfOnlyOneDir, pathListToTree, type File} from './filetree.ts';

const emptyList: File[] = [];
const singleFile = [{Name: 'file1'}] as File[];
const singleDir = [{Name: 'dir1/file1'}] as File[];
const nestedDir = [{Name: 'dir1/dir2/file1'}] as File[];
const multiplePathsDisjoint = [{Name: 'dir1/dir2/file1'}, {Name: 'dir3/file2'}] as File[];
const multiplePathsShared = [{Name: 'dir1/dir2/dir3/file1'}, {Name: 'dir1/file2'}] as File[];

test('pathListToTree', () => {
  expect(pathListToTree(emptyList)).toEqual([]);
  expect(pathListToTree(singleFile)).toEqual([
    {isFile: true, name: 'file1', path: 'file1', file: {Name: 'file1'}},
  ]);
  expect(pathListToTree(singleDir)).toEqual([
    {isFile: false, name: 'dir1', path: 'dir1', children: [
      {isFile: true, name: 'file1', path: 'dir1/file1', file: {Name: 'dir1/file1'}},
    ]},
  ]);
  expect(pathListToTree(nestedDir)).toEqual([
    {isFile: false, name: 'dir1', path: 'dir1', children: [
      {isFile: false, name: 'dir2', path: 'dir1/dir2', children: [
        {isFile: true, name: 'file1', path: 'dir1/dir2/file1', file: {Name: 'dir1/dir2/file1'}},
      ]},
    ]},
  ]);
  expect(pathListToTree(multiplePathsDisjoint)).toEqual([
    {isFile: false, name: 'dir1', path: 'dir1', children: [
      {isFile: false, name: 'dir2', path: 'dir1/dir2', children: [
        {isFile: true, name: 'file1', path: 'dir1/dir2/file1', file: {Name: 'dir1/dir2/file1'}},
      ]},
    ]},
    {isFile: false, name: 'dir3', path: 'dir3', children: [
      {isFile: true, name: 'file2', path: 'dir3/file2', file: {Name: 'dir3/file2'}},
    ]},
  ]);
  expect(pathListToTree(multiplePathsShared)).toEqual([
    {isFile: false, name: 'dir1', path: 'dir1', children: [
      {isFile: false, name: 'dir2', path: 'dir1/dir2', children: [
        {isFile: false, name: 'dir3', path: 'dir1/dir2/dir3', children: [
          {isFile: true, name: 'file1', path: 'dir1/dir2/dir3/file1', file: {Name: 'dir1/dir2/dir3/file1'}},
        ]},
      ]},
      {isFile: true, name: 'file2', path: 'dir1/file2', file: {Name: 'dir1/file2'}},
    ]},
  ]);
});

const mergeChildWrapper = (testCase: File[]) => {
  const tree = pathListToTree(testCase);
  mergeChildIfOnlyOneDir(tree);
  return tree;
};

test('mergeChildIfOnlyOneDir', () => {
  expect(mergeChildWrapper(emptyList)).toEqual([]);
  expect(mergeChildWrapper(singleFile)).toEqual([
    {isFile: true, name: 'file1', path: 'file1', file: {Name: 'file1'}},
  ]);
  expect(mergeChildWrapper(singleDir)).toEqual([
    {isFile: false, name: 'dir1', path: 'dir1', children: [
      {isFile: true, name: 'file1', path: 'dir1/file1', file: {Name: 'dir1/file1'}},
    ]},
  ]);
  expect(mergeChildWrapper(nestedDir)).toEqual([
    {isFile: false, name: 'dir1/dir2', path: 'dir1/dir2', children: [
      {isFile: true, name: 'file1', path: 'dir1/dir2/file1', file: {Name: 'dir1/dir2/file1'}},
    ]},
  ]);
  expect(mergeChildWrapper(multiplePathsDisjoint)).toEqual([
    {isFile: false, name: 'dir1/dir2', path: 'dir1/dir2', children: [
      {isFile: true, name: 'file1', path: 'dir1/dir2/file1', file: {Name: 'dir1/dir2/file1'}},
    ]},
    {isFile: false, name: 'dir3', path: 'dir3', children: [
      {isFile: true, name: 'file2', path: 'dir3/file2', file: {Name: 'dir3/file2'}},
    ]},
  ]);
  expect(mergeChildWrapper(multiplePathsShared)).toEqual([
    {isFile: false, name: 'dir1', path: 'dir1', children: [
      {isFile: false, name: 'dir2/dir3', path: 'dir1/dir2/dir3', children: [
        {isFile: true, name: 'file1', path: 'dir1/dir2/dir3/file1', file: {Name: 'dir1/dir2/dir3/file1'}},
      ]},
      {isFile: true, name: 'file2', path: 'dir1/file2', file: {Name: 'dir1/file2'}},
    ]},
  ]);
});
