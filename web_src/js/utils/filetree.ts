import {dirname, basename} from '../utils.ts';

export type FileStatus = 'added' | 'modified' | 'deleted' | 'renamed' | 'copied' | 'typechange';

export type File = {
  Name: string;
  NameHash: string;
  Status: FileStatus;
  IsViewed: boolean;
  IsSubmodule: boolean;
}

type DirItem = {
    isFile: false;
    name: string;
    path: string;

    children: Item[];
}

type FileItem = {
    isFile: true;
    name: string;
    path: string;
    file: File;
}

export type Item = DirItem | FileItem;

export function pathListToTree(fileEntries: File[]): Item[] {
  const pathToItem = new Map<string, DirItem>();

    // init root node
  const root: DirItem = {name: '', path: '', isFile: false, children: []};
  pathToItem.set('', root);

  for (const fileEntry of fileEntries) {
    const [parentPath, fileName] = [dirname(fileEntry.Name), basename(fileEntry.Name)];

    let parentItem = pathToItem.get(parentPath);
    if (!parentItem) {
      parentItem = constructParents(pathToItem, parentPath);
    }

    const fileItem: FileItem = {name: fileName, path: fileEntry.Name, isFile: true, file: fileEntry};

    parentItem.children.push(fileItem);
  }

  return root.children;
}

function constructParents(pathToItem: Map<string, DirItem>, dirPath: string): DirItem {
  const [dirParentPath, dirName] = [dirname(dirPath), basename(dirPath)];

  let parentItem = pathToItem.get(dirParentPath);
  if (!parentItem) {
    // if the parent node does not exist, create it
    parentItem = constructParents(pathToItem, dirParentPath);
  }

  const dirItem: DirItem = {name: dirName, path: dirPath, isFile: false, children: []};
  parentItem.children.push(dirItem);
  pathToItem.set(dirPath, dirItem);

  return dirItem;
}

export function mergeChildIfOnlyOneDir(nodes: Item[]): void {
  for (const node of nodes) {
    if (node.isFile) {
      continue;
    }
    const dir = node as DirItem;

    mergeChildIfOnlyOneDir(dir.children);

    if (dir.children.length === 1 && dir.children[0].isFile === false) {
      const child = dir.children[0];
      dir.name = `${dir.name}/${child.name}`;
      dir.path = child.path;
      dir.children = child.children;
    }
  }
}
