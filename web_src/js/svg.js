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


export const svgs = {
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
  'octicon-milestone': octiconMilestone,
  'octicon-mirror': octiconMirror,
  'octicon-project': octiconProject,
  'octicon-repo': octiconRepo,
  'octicon-repo-forked': octiconRepoForked,
  'octicon-repo-template': octiconRepoTemplate,
  'octicon-sidebar-collapse': octiconSidebarCollapse,
  'octicon-sidebar-expand': octiconSidebarExpand,
  'octicon-triangle-down': octiconTriangleDown,
  'octicon-x': octiconX,
};


const parser = new DOMParser();
const serializer = new XMLSerializer();

// retrieve a HTML string for given SVG icon name, size and additional classes
export function svg(name, size = 16, className = '') {
  if (!(name in svgs)) return '';
  if (size === 16 && !className) return svgs[name];

  const document = parser.parseFromString(svgs[name], 'image/svg+xml');
  const svgNode = document.firstChild;
  if (size !== 16) svgNode.setAttribute('width', String(size));
  if (size !== 16) svgNode.setAttribute('height', String(size));
  if (className) svgNode.classList.add(...className.split(/\s+/));
  return serializer.serializeToString(svgNode);
}

export const SvgIcon = {
  name: 'SvgIcon',
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
};
