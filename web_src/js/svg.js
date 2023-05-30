import {h} from 'vue';
import giteaDoubleChevronLeft from '../../public/img/svg/gitea-double-chevron-left.svg';
import giteaDoubleChevronRight from '../../public/img/svg/gitea-double-chevron-right.svg';
import giteaEmptyCheckbox from '../../public/img/svg/gitea-empty-checkbox.svg';
import giteaExclamation from '../../public/img/svg/gitea-exclamation.svg';
import octiconArchive from '../../public/img/svg/octicon-archive.svg';
import octiconArrowSwitch from '../../public/img/svg/octicon-arrow-switch.svg';
import octiconBlocked from '../../public/img/svg/octicon-blocked.svg';
import octiconBold from '../../public/img/svg/octicon-bold.svg';
import octiconCheck from '../../public/img/svg/octicon-check.svg';
import octiconCheckbox from '../../public/img/svg/octicon-checkbox.svg';
import octiconCheckCircleFill from '../../public/img/svg/octicon-check-circle-fill.svg';
import octiconChevronDown from '../../public/img/svg/octicon-chevron-down.svg';
import octiconChevronLeft from '../../public/img/svg/octicon-chevron-left.svg';
import octiconChevronRight from '../../public/img/svg/octicon-chevron-right.svg';
import octiconClock from '../../public/img/svg/octicon-clock.svg';
import octiconCode from '../../public/img/svg/octicon-code.svg';
import octiconColumns from '../../public/img/svg/octicon-columns.svg';
import octiconCopy from '../../public/img/svg/octicon-copy.svg';
import octiconDiffAdded from '../../public/img/svg/octicon-diff-added.svg';
import octiconDiffModified from '../../public/img/svg/octicon-diff-modified.svg';
import octiconDiffRemoved from '../../public/img/svg/octicon-diff-removed.svg';
import octiconDiffRenamed from '../../public/img/svg/octicon-diff-renamed.svg';
import octiconDotFill from '../../public/img/svg/octicon-dot-fill.svg';
import octiconEye from '../../public/img/svg/octicon-eye.svg';
import octiconFile from '../../public/img/svg/octicon-file.svg';
import octiconFileDirectoryFill from '../../public/img/svg/octicon-file-directory-fill.svg';
import octiconFilter from '../../public/img/svg/octicon-filter.svg';
import octiconGear from '../../public/img/svg/octicon-gear.svg';
import octiconGitBranch from '../../public/img/svg/octicon-git-branch.svg';
import octiconGitMerge from '../../public/img/svg/octicon-git-merge.svg';
import octiconGitPullRequest from '../../public/img/svg/octicon-git-pull-request.svg';
import octiconHeading from '../../public/img/svg/octicon-heading.svg';
import octiconHorizontalRule from '../../public/img/svg/octicon-horizontal-rule.svg';
import octiconImage from '../../public/img/svg/octicon-image.svg';
import octiconIssueClosed from '../../public/img/svg/octicon-issue-closed.svg';
import octiconIssueOpened from '../../public/img/svg/octicon-issue-opened.svg';
import octiconItalic from '../../public/img/svg/octicon-italic.svg';
import octiconKebabHorizontal from '../../public/img/svg/octicon-kebab-horizontal.svg';
import octiconLink from '../../public/img/svg/octicon-link.svg';
import octiconListOrdered from '../../public/img/svg/octicon-list-ordered.svg';
import octiconListUnordered from '../../public/img/svg/octicon-list-unordered.svg';
import octiconLock from '../../public/img/svg/octicon-lock.svg';
import octiconMeter from '../../public/img/svg/octicon-meter.svg';
import octiconMilestone from '../../public/img/svg/octicon-milestone.svg';
import octiconMirror from '../../public/img/svg/octicon-mirror.svg';
import octiconOrganization from '../../public/img/svg/octicon-organization.svg';
import octiconPlay from '../../public/img/svg/octicon-play.svg';
import octiconPlus from '../../public/img/svg/octicon-plus.svg';
import octiconProject from '../../public/img/svg/octicon-project.svg';
import octiconQuote from '../../public/img/svg/octicon-quote.svg';
import octiconRepo from '../../public/img/svg/octicon-repo.svg';
import octiconRepoForked from '../../public/img/svg/octicon-repo-forked.svg';
import octiconRepoTemplate from '../../public/img/svg/octicon-repo-template.svg';
import octiconRss from '../../public/img/svg/octicon-rss.svg';
import octiconScreenFull from '../../public/img/svg/octicon-screen-full.svg';
import octiconSearch from '../../public/img/svg/octicon-search.svg';
import octiconSidebarCollapse from '../../public/img/svg/octicon-sidebar-collapse.svg';
import octiconSidebarExpand from '../../public/img/svg/octicon-sidebar-expand.svg';
import octiconSkip from '../../public/img/svg/octicon-skip.svg';
import octiconStar from '../../public/img/svg/octicon-star.svg';
import octiconStrikethrough from '../../public/img/svg/octicon-strikethrough.svg';
import octiconSync from '../../public/img/svg/octicon-sync.svg';
import octiconTable from '../../public/img/svg/octicon-table.svg';
import octiconTag from '../../public/img/svg/octicon-tag.svg';
import octiconTriangleDown from '../../public/img/svg/octicon-triangle-down.svg';
import octiconX from '../../public/img/svg/octicon-x.svg';
import octiconXCircleFill from '../../public/img/svg/octicon-x-circle-fill.svg';

const svgs = {
  'gitea-double-chevron-left': giteaDoubleChevronLeft,
  'gitea-double-chevron-right': giteaDoubleChevronRight,
  'gitea-empty-checkbox': giteaEmptyCheckbox,
  'gitea-exclamation': giteaExclamation,
  'octicon-archive': octiconArchive,
  'octicon-arrow-switch': octiconArrowSwitch,
  'octicon-blocked': octiconBlocked,
  'octicon-bold': octiconBold,
  'octicon-check': octiconCheck,
  'octicon-check-circle-fill': octiconCheckCircleFill,
  'octicon-checkbox': octiconCheckbox,
  'octicon-chevron-down': octiconChevronDown,
  'octicon-chevron-left': octiconChevronLeft,
  'octicon-chevron-right': octiconChevronRight,
  'octicon-clock': octiconClock,
  'octicon-code': octiconCode,
  'octicon-columns': octiconColumns,
  'octicon-copy': octiconCopy,
  'octicon-diff-added': octiconDiffAdded,
  'octicon-diff-modified': octiconDiffModified,
  'octicon-diff-removed': octiconDiffRemoved,
  'octicon-diff-renamed': octiconDiffRenamed,
  'octicon-dot-fill': octiconDotFill,
  'octicon-eye': octiconEye,
  'octicon-file': octiconFile,
  'octicon-file-directory-fill': octiconFileDirectoryFill,
  'octicon-filter': octiconFilter,
  'octicon-gear': octiconGear,
  'octicon-git-branch': octiconGitBranch,
  'octicon-git-merge': octiconGitMerge,
  'octicon-git-pull-request': octiconGitPullRequest,
  'octicon-heading': octiconHeading,
  'octicon-horizontal-rule': octiconHorizontalRule,
  'octicon-image': octiconImage,
  'octicon-issue-closed': octiconIssueClosed,
  'octicon-issue-opened': octiconIssueOpened,
  'octicon-italic': octiconItalic,
  'octicon-kebab-horizontal': octiconKebabHorizontal,
  'octicon-link': octiconLink,
  'octicon-list-ordered': octiconListOrdered,
  'octicon-list-unordered': octiconListUnordered,
  'octicon-lock': octiconLock,
  'octicon-meter': octiconMeter,
  'octicon-milestone': octiconMilestone,
  'octicon-mirror': octiconMirror,
  'octicon-organization': octiconOrganization,
  'octicon-play': octiconPlay,
  'octicon-plus': octiconPlus,
  'octicon-project': octiconProject,
  'octicon-quote': octiconQuote,
  'octicon-repo': octiconRepo,
  'octicon-repo-forked': octiconRepoForked,
  'octicon-repo-template': octiconRepoTemplate,
  'octicon-rss': octiconRss,
  'octicon-screen-full': octiconScreenFull,
  'octicon-search': octiconSearch,
  'octicon-sidebar-collapse': octiconSidebarCollapse,
  'octicon-sidebar-expand': octiconSidebarExpand,
  'octicon-skip': octiconSkip,
  'octicon-star': octiconStar,
  'octicon-strikethrough': octiconStrikethrough,
  'octicon-sync': octiconSync,
  'octicon-table': octiconTable,
  'octicon-tag': octiconTag,
  'octicon-triangle-down': octiconTriangleDown,
  'octicon-x': octiconX,
  'octicon-x-circle-fill': octiconXCircleFill,
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
  if (size !== 16) {
    svgNode.setAttribute('width', String(size));
    svgNode.setAttribute('height', String(size));
  }
  if (className) svgNode.classList.add(...className.split(/\s+/).filter(Boolean));
  return serializer.serializeToString(svgNode);
}

export function svgParseOuterInner(name) {
  const svgStr = svgs[name];
  if (!svgStr) throw new Error(`Unknown SVG icon: ${name}`);

  // parse the SVG string to 2 parts
  // * svgInnerHtml: the inner part of the SVG, will be used as the content of the <svg> VNode
  // * svgOuter: the outer part of the SVG, including attributes
  // the builtin SVG contents are clean, so it's safe to use `indexOf` to split the content:
  // eg: <svg outer-attributes>${svgInnerHtml}</svg>
  const p1 = svgStr.indexOf('>'), p2 = svgStr.lastIndexOf('<');
  if (p1 === -1 || p2 === -1) throw new Error(`Invalid SVG icon: ${name}`);
  const svgInnerHtml = svgStr.slice(p1 + 1, p2);
  const svgOuterHtml = svgStr.slice(0, p1 + 1) + svgStr.slice(p2);
  const svgDoc = parser.parseFromString(svgOuterHtml, 'image/svg+xml');
  const svgOuter = svgDoc.firstChild;
  return {svgOuter, svgInnerHtml};
}

export const SvgIcon = {
  name: 'SvgIcon',
  props: {
    name: {type: String, required: true},
    size: {type: Number, default: 16},
    className: {type: String, default: ''},
  },
  render() {
    const {svgOuter, svgInnerHtml} = svgParseOuterInner(this.name);
    // https://vuejs.org/guide/extras/render-function.html#creating-vnodes
    // the `^` is used for attr, set SVG attributes like 'width', `aria-hidden`, `viewBox`, etc
    const attrs = {};
    for (const attr of svgOuter.attributes) {
      if (attr.name === 'class') continue;
      attrs[`^${attr.name}`] = attr.value;
    }
    attrs[`^width`] = this.size;
    attrs[`^height`] = this.size;

    // make the <SvgIcon class="foo" class-name="bar"> classes work together
    const classes = [];
    for (const cls of svgOuter.classList) {
      classes.push(cls);
    }
    // TODO: drop the `className/class-name` prop in the future, only use "class" prop
    if (this.className) {
      classes.push(...this.className.split(/\s+/).filter(Boolean));
    }

    // create VNode
    return h('svg', {
      ...attrs,
      class: classes,
      innerHTML: svgInnerHtml,
    });
  },
};
