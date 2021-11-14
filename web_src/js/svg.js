import octiconChevronDown from '../../public/img/svg/octicon-chevron-down.svg';
import octiconChevronRight from '../../public/img/svg/octicon-chevron-right.svg';
import octiconCopy from '../../public/img/svg/octicon-copy.svg';
import octiconGitMerge from '../../public/img/svg/octicon-git-merge.svg';
import octiconGitPullRequest from '../../public/img/svg/octicon-git-pull-request.svg';
import octiconIssueClosed from '../../public/img/svg/octicon-issue-closed.svg';
import octiconIssueOpened from '../../public/img/svg/octicon-issue-opened.svg';
import octiconKebabHorizontal from '../../public/img/svg/octicon-kebab-horizontal.svg';
import octiconLink from '../../public/img/svg/octicon-link.svg';
import octiconLock from '../../public/img/svg/octicon-lock.svg';
import octiconMilestone from '../../public/img/svg/octicon-milestone.svg';
import octiconMirror from '../../public/img/svg/octicon-mirror.svg';
import octiconProject from '../../public/img/svg/octicon-project.svg';
import octiconRepo from '../../public/img/svg/octicon-repo.svg';
import octiconRepoForked from '../../public/img/svg/octicon-repo-forked.svg';
import octiconRepoTemplate from '../../public/img/svg/octicon-repo-template.svg';
import octiconTriangleDown from '../../public/img/svg/octicon-triangle-down.svg';

import Vue from 'vue';

export const svgs = {
  'octicon-chevron-down': octiconChevronDown,
  'octicon-chevron-right': octiconChevronRight,
  'octicon-copy': octiconCopy,
  'octicon-git-merge': octiconGitMerge,
  'octicon-git-pull-request': octiconGitPullRequest,
  'octicon-issue-closed': octiconIssueClosed,
  'octicon-issue-opened': octiconIssueOpened,
  'octicon-kebab-horizontal': octiconKebabHorizontal,
  'octicon-link': octiconLink,
  'octicon-lock': octiconLock,
  'octicon-milestone': octiconMilestone,
  'octicon-mirror': octiconMirror,
  'octicon-project': octiconProject,
  'octicon-repo': octiconRepo,
  'octicon-repo-forked': octiconRepoForked,
  'octicon-repo-template': octiconRepoTemplate,
  'octicon-triangle-down': octiconTriangleDown,
};

const parser = new DOMParser();
const serializer = new XMLSerializer();
const parsedSvgs = new Map();

function getParsedSvg(name) {
  if (parsedSvgs.has(name)) return parsedSvgs.get(name).cloneNode(true);
  const root = parser.parseFromString(svgs[name], 'text/html');
  const svgNode = root.querySelector('svg');
  parsedSvgs.set(name, svgNode);
  return svgNode;
}

function applyAttributes(node, size, className) {
  if (size !== 16) node.setAttribute('width', String(size));
  if (size !== 16) node.setAttribute('height', String(size));
  if (className) node.classList.add(...className.split(/\s+/));
  return node;
}

// returns a SVG node for given SVG icon name, size and additional classes
export function svgNode(name, size = 16, className = '') {
  return applyAttributes(getParsedSvg(name), size, className);
}

// returns a HTML string for given SVG icon name, size and additional classes
export function svg(name, size, className) {
  if (!(name in svgs)) return '';
  if (size === 16 && !className) return svgs[name];
  return serializer.serializeToString(svgNode(name, size, className));
}

export const SvgIcon = Vue.component('SvgIcon', {
  props: {
    name: {type: String, required: true},
    size: {type: Number, default: 16},
    className: {type: String, default: ''},
  },

  computed: {
    svg() {
      return svg(this.name, this.size, this.className);
    },
  },

  template: `<span v-html="svg" />`
});
