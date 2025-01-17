import {h} from 'vue';
import {parseDom, serializeXml} from './utils.ts';
import giteaDoubleChevronLeft from '../../public/assets/img/svg/gitea-double-chevron-left.svg';
import giteaDoubleChevronRight from '../../public/assets/img/svg/gitea-double-chevron-right.svg';
import giteaEmptyCheckbox from '../../public/assets/img/svg/gitea-empty-checkbox.svg';
import giteaExclamation from '../../public/assets/img/svg/gitea-exclamation.svg';
import octiconArchive from '../../public/assets/img/svg/octicon-archive.svg';
import octiconArrowSwitch from '../../public/assets/img/svg/octicon-arrow-switch.svg';
import octiconBlocked from '../../public/assets/img/svg/octicon-blocked.svg';
import octiconBold from '../../public/assets/img/svg/octicon-bold.svg';
import octiconCheck from '../../public/assets/img/svg/octicon-check.svg';
import octiconCheckbox from '../../public/assets/img/svg/octicon-checkbox.svg';
import octiconCheckCircleFill from '../../public/assets/img/svg/octicon-check-circle-fill.svg';
import octiconChevronDown from '../../public/assets/img/svg/octicon-chevron-down.svg';
import octiconChevronLeft from '../../public/assets/img/svg/octicon-chevron-left.svg';
import octiconChevronRight from '../../public/assets/img/svg/octicon-chevron-right.svg';
import octiconClock from '../../public/assets/img/svg/octicon-clock.svg';
import octiconCode from '../../public/assets/img/svg/octicon-code.svg';
import octiconColumns from '../../public/assets/img/svg/octicon-columns.svg';
import octiconCopy from '../../public/assets/img/svg/octicon-copy.svg';
import octiconDiffAdded from '../../public/assets/img/svg/octicon-diff-added.svg';
import octiconDiffModified from '../../public/assets/img/svg/octicon-diff-modified.svg';
import octiconDiffRemoved from '../../public/assets/img/svg/octicon-diff-removed.svg';
import octiconDiffRenamed from '../../public/assets/img/svg/octicon-diff-renamed.svg';
import octiconDotFill from '../../public/assets/img/svg/octicon-dot-fill.svg';
import octiconDownload from '../../public/assets/img/svg/octicon-download.svg';
import octiconEye from '../../public/assets/img/svg/octicon-eye.svg';
import octiconFile from '../../public/assets/img/svg/octicon-file.svg';
import octiconFileDirectoryFill from '../../public/assets/img/svg/octicon-file-directory-fill.svg';
import octiconFileDirectoryOpenFill from '../../public/assets/img/svg/octicon-file-directory-open-fill.svg';
import octiconFilter from '../../public/assets/img/svg/octicon-filter.svg';
import octiconGear from '../../public/assets/img/svg/octicon-gear.svg';
import octiconGitBranch from '../../public/assets/img/svg/octicon-git-branch.svg';
import octiconGitCommit from '../../public/assets/img/svg/octicon-git-commit.svg';
import octiconGitMerge from '../../public/assets/img/svg/octicon-git-merge.svg';
import octiconGitPullRequest from '../../public/assets/img/svg/octicon-git-pull-request.svg';
import octiconGitPullRequestClosed from '../../public/assets/img/svg/octicon-git-pull-request-closed.svg';
import octiconGitPullRequestDraft from '../../public/assets/img/svg/octicon-git-pull-request-draft.svg';
import octiconGrabber from '../../public/assets/img/svg/octicon-grabber.svg';
import octiconHeading from '../../public/assets/img/svg/octicon-heading.svg';
import octiconHorizontalRule from '../../public/assets/img/svg/octicon-horizontal-rule.svg';
import octiconImage from '../../public/assets/img/svg/octicon-image.svg';
import octiconIssueClosed from '../../public/assets/img/svg/octicon-issue-closed.svg';
import octiconIssueOpened from '../../public/assets/img/svg/octicon-issue-opened.svg';
import octiconItalic from '../../public/assets/img/svg/octicon-italic.svg';
import octiconKebabHorizontal from '../../public/assets/img/svg/octicon-kebab-horizontal.svg';
import octiconLink from '../../public/assets/img/svg/octicon-link.svg';
import octiconListOrdered from '../../public/assets/img/svg/octicon-list-ordered.svg';
import octiconListUnordered from '../../public/assets/img/svg/octicon-list-unordered.svg';
import octiconLock from '../../public/assets/img/svg/octicon-lock.svg';
import octiconMeter from '../../public/assets/img/svg/octicon-meter.svg';
import octiconMilestone from '../../public/assets/img/svg/octicon-milestone.svg';
import octiconMirror from '../../public/assets/img/svg/octicon-mirror.svg';
import octiconOrganization from '../../public/assets/img/svg/octicon-organization.svg';
import octiconPlay from '../../public/assets/img/svg/octicon-play.svg';
import octiconPlus from '../../public/assets/img/svg/octicon-plus.svg';
import octiconProject from '../../public/assets/img/svg/octicon-project.svg';
import octiconQuote from '../../public/assets/img/svg/octicon-quote.svg';
import octiconRepo from '../../public/assets/img/svg/octicon-repo.svg';
import octiconRepoForked from '../../public/assets/img/svg/octicon-repo-forked.svg';
import octiconRepoTemplate from '../../public/assets/img/svg/octicon-repo-template.svg';
import octiconRss from '../../public/assets/img/svg/octicon-rss.svg';
import octiconScreenFull from '../../public/assets/img/svg/octicon-screen-full.svg';
import octiconSearch from '../../public/assets/img/svg/octicon-search.svg';
import octiconSidebarCollapse from '../../public/assets/img/svg/octicon-sidebar-collapse.svg';
import octiconSidebarExpand from '../../public/assets/img/svg/octicon-sidebar-expand.svg';
import octiconSkip from '../../public/assets/img/svg/octicon-skip.svg';
import octiconStar from '../../public/assets/img/svg/octicon-star.svg';
import octiconStop from '../../public/assets/img/svg/octicon-stop.svg';
import octiconStrikethrough from '../../public/assets/img/svg/octicon-strikethrough.svg';
import octiconSync from '../../public/assets/img/svg/octicon-sync.svg';
import octiconTable from '../../public/assets/img/svg/octicon-table.svg';
import octiconTag from '../../public/assets/img/svg/octicon-tag.svg';
import octiconTrash from '../../public/assets/img/svg/octicon-trash.svg';
import octiconTriangleDown from '../../public/assets/img/svg/octicon-triangle-down.svg';
import octiconX from '../../public/assets/img/svg/octicon-x.svg';
import octiconXCircleFill from '../../public/assets/img/svg/octicon-x-circle-fill.svg';

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
  'octicon-download': octiconDownload,
  'octicon-eye': octiconEye,
  'octicon-file': octiconFile,
  'octicon-file-directory-fill': octiconFileDirectoryFill,
  'octicon-file-directory-open-fill': octiconFileDirectoryOpenFill,
  'octicon-filter': octiconFilter,
  'octicon-gear': octiconGear,
  'octicon-git-branch': octiconGitBranch,
  'octicon-git-commit': octiconGitCommit,
  'octicon-git-merge': octiconGitMerge,
  'octicon-git-pull-request': octiconGitPullRequest,
  'octicon-git-pull-request-closed': octiconGitPullRequestClosed,
  'octicon-git-pull-request-draft': octiconGitPullRequestDraft,
  'octicon-grabber': octiconGrabber,
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
  'octicon-stop': octiconStop,
  'octicon-strikethrough': octiconStrikethrough,
  'octicon-sync': octiconSync,
  'octicon-table': octiconTable,
  'octicon-tag': octiconTag,
  'octicon-trash': octiconTrash,
  'octicon-triangle-down': octiconTriangleDown,
  'octicon-x': octiconX,
  'octicon-x-circle-fill': octiconXCircleFill,
};

export type SvgName = keyof typeof svgs;

// TODO: use a more general approach to access SVG icons.
//  At the moment, developers must check, pick and fill the names manually,
//  most of the SVG icons in assets couldn't be used directly.

// retrieve an HTML string for given SVG icon name, size and additional classes
export function svg(name: SvgName, size = 16, classNames?: string|string[]): string {
  const className = Array.isArray(classNames) ? classNames.join(' ') : classNames;
  if (!(name in svgs)) throw new Error(`Unknown SVG icon: ${name}`);
  if (size === 16 && !className) return svgs[name];

  const document = parseDom(svgs[name], 'image/svg+xml');
  const svgNode = document.firstChild as SVGElement;
  if (size !== 16) {
    svgNode.setAttribute('width', String(size));
    svgNode.setAttribute('height', String(size));
  }
  if (className) svgNode.classList.add(...className.split(/\s+/).filter(Boolean));
  return serializeXml(svgNode);
}

export function svgParseOuterInner(name: SvgName) {
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
  const svgDoc = parseDom(svgOuterHtml, 'image/svg+xml');
  const svgOuter = svgDoc.firstChild as SVGElement;
  return {svgOuter, svgInnerHtml};
}

export const SvgIcon = {
  name: 'SvgIcon',
  props: {
    name: {type: String, required: true},
    size: {type: Number, default: 16},
    className: {type: String, default: ''},
    symbolId: {type: String},
  },
  render() {
    let {svgOuter, svgInnerHtml} = svgParseOuterInner(this.name);
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
    if (this.symbolId) {
      classes.push('tw-hidden', 'svg-symbol-container');
      svgInnerHtml = `<symbol id="${this.symbolId}" viewBox="${attrs['^viewBox']}">${svgInnerHtml}</symbol>`;
    }
    // create VNode
    return h('svg', {
      ...attrs,
      class: classes,
      innerHTML: svgInnerHtml,
    });
  },
};
