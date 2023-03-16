import {h} from 'vue';
import octiconChevronDown from '../../public/img/svg/octicon-chevron-down.svg';
import octiconChevronRight from '../../public/img/svg/octicon-chevron-right.svg';
import octiconClock from '../../public/img/svg/octicon-clock.svg';
import octiconCopy from '../../public/img/svg/octicon-copy.svg';
import octiconDiffAdded from '../../public/img/svg/octicon-diff-added.svg';
import octiconDiffModified from '../../public/img/svg/octicon-diff-modified.svg';
import octiconDiffRemoved from '../../public/img/svg/octicon-diff-removed.svg';
import octiconDiffRenamed from '../../public/img/svg/octicon-diff-renamed.svg';
import octiconFile from '../../public/img/svg/octicon-file.svg';
import octiconFileDirectoryFill from '../../public/img/svg/octicon-file-directory-fill.svg';
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
import octiconSidebarCollapse from '../../public/img/svg/octicon-sidebar-collapse.svg';
import octiconSidebarExpand from '../../public/img/svg/octicon-sidebar-expand.svg';
import octiconTriangleDown from '../../public/img/svg/octicon-triangle-down.svg';
import octiconX from '../../public/img/svg/octicon-x.svg';
import octiconCheckCircleFill from '../../public/img/svg/octicon-check-circle-fill.svg';
import octiconXCircleFill from '../../public/img/svg/octicon-x-circle-fill.svg';
import octiconSkip from '../../public/img/svg/octicon-skip.svg';
import octiconMeter from '../../public/img/svg/octicon-meter.svg';
import octiconBlocked from '../../public/img/svg/octicon-blocked.svg';
import octiconSync from '../../public/img/svg/octicon-sync.svg';
import octiconFilter from '../../public/img/svg/octicon-filter.svg';
import octiconPlus from '../../public/img/svg/octicon-plus.svg';
import octiconSearch from '../../public/img/svg/octicon-search.svg';
import octiconArchive from '../../public/img/svg/octicon-archive.svg';
import octiconStar from '../../public/img/svg/octicon-star.svg';
import giteaDoubleChevronLeft from '../../public/img/svg/gitea-double-chevron-left.svg';
import giteaDoubleChevronRight from '../../public/img/svg/gitea-double-chevron-right.svg';
import octiconChevronLeft from '../../public/img/svg/octicon-chevron-left.svg';
import octiconOrganization from '../../public/img/svg/octicon-organization.svg';
import octiconTag from '../../public/img/svg/octicon-tag.svg';
import octiconGitBranch from '../../public/img/svg/octicon-git-branch.svg';

const svgs = {
  'octicon-blocked': octiconBlocked,
  'octicon-check-circle-fill': octiconCheckCircleFill,
  'octicon-chevron-down': octiconChevronDown,
  'octicon-chevron-right': octiconChevronRight,
  'octicon-clock': octiconClock,
  'octicon-copy': octiconCopy,
  'octicon-diff-added': octiconDiffAdded,
  'octicon-diff-modified': octiconDiffModified,
  'octicon-diff-removed': octiconDiffRemoved,
  'octicon-diff-renamed': octiconDiffRenamed,
  'octicon-file': octiconFile,
  'octicon-file-directory-fill': octiconFileDirectoryFill,
  'octicon-git-merge': octiconGitMerge,
  'octicon-git-pull-request': octiconGitPullRequest,
  'octicon-issue-closed': octiconIssueClosed,
  'octicon-issue-opened': octiconIssueOpened,
  'octicon-kebab-horizontal': octiconKebabHorizontal,
  'octicon-link': octiconLink,
  'octicon-lock': octiconLock,
  'octicon-meter': octiconMeter,
  'octicon-milestone': octiconMilestone,
  'octicon-mirror': octiconMirror,
  'octicon-project': octiconProject,
  'octicon-repo': octiconRepo,
  'octicon-repo-forked': octiconRepoForked,
  'octicon-repo-template': octiconRepoTemplate,
  'octicon-sidebar-collapse': octiconSidebarCollapse,
  'octicon-sidebar-expand': octiconSidebarExpand,
  'octicon-skip': octiconSkip,
  'octicon-sync': octiconSync,
  'octicon-triangle-down': octiconTriangleDown,
  'octicon-x': octiconX,
  'octicon-x-circle-fill': octiconXCircleFill,
  'octicon-filter': octiconFilter,
  'octicon-plus': octiconPlus,
  'octicon-search': octiconSearch,
  'octicon-archive': octiconArchive,
  'octicon-star': octiconStar,
  'gitea-double-chevron-left': giteaDoubleChevronLeft,
  'gitea-double-chevron-right': giteaDoubleChevronRight,
  'octicon-chevron-left': octiconChevronLeft,
  'octicon-organization': octiconOrganization,
  'octicon-tag': octiconTag,
  'octicon-git-branch': octiconGitBranch,
};

// TODO: use a more general approach to access SVG icons.
//  At the moment, developers must check, pick and fill the names manually,
//  most of the SVG icons in assets couldn't be used directly.

const parser = new DOMParser();
const serializer = new XMLSerializer();

// retrieve an HTML string for given SVG icon name, size and additional classes
export function svg(name, size = 16, className = '') {
  if (!(name in svgs)) throw new Error(`Unknown SVG icon: ${name}`);
  if (size === 16 && !className) return svgs[name];

  const document = parser.parseFromString(svgs[name], 'image/svg+xml');
  const svgNode = document.firstChild;
  if (size !== 16) svgNode.setAttribute('width', String(size));
  if (size !== 16) svgNode.setAttribute('height', String(size));
  // filter array to remove empty string
  if (className) svgNode.classList.add(...className.split(/\s+/).filter(Boolean));
  return serializer.serializeToString(svgNode);
}

export const SvgIcon = {
  name: 'SvgIcon',
  props: {
    name: {type: String, required: true},
    size: {type: Number, default: 16},
    className: {type: String, default: ''},
  },
  render() {
    return h('span', {innerHTML: svg(this.name, this.size, this.className)});
  },
};
