import {parseDom, serializeXml} from './utils.ts';
import {htmlRaw} from './utils/html.ts';
import giteaDoubleChevronLeft from '../../public/assets/img/svg/gitea-double-chevron-left.svg?raw';
import giteaDoubleChevronRight from '../../public/assets/img/svg/gitea-double-chevron-right.svg?raw';
import giteaEmptyCheckbox from '../../public/assets/img/svg/gitea-empty-checkbox.svg?raw';
import giteaExclamation from '../../public/assets/img/svg/gitea-exclamation.svg?raw';
import giteaFavicon from '../../public/assets/img/favicon.svg?raw';
import giteaRunning from '../../public/assets/img/svg/gitea-running.svg?raw';
import octiconArchive from '../../public/assets/img/svg/octicon-archive.svg?raw';
import octiconArrowLeft from '../../public/assets/img/svg/octicon-arrow-left.svg?raw';
import octiconArrowSwitch from '../../public/assets/img/svg/octicon-arrow-switch.svg?raw';
import octiconBlocked from '../../public/assets/img/svg/octicon-blocked.svg?raw';
import octiconBold from '../../public/assets/img/svg/octicon-bold.svg?raw';
import octiconCheck from '../../public/assets/img/svg/octicon-check.svg?raw';
import octiconCheckbox from '../../public/assets/img/svg/octicon-checkbox.svg?raw';
import octiconCheckCircleFill from '../../public/assets/img/svg/octicon-check-circle-fill.svg?raw';
import octiconChevronDown from '../../public/assets/img/svg/octicon-chevron-down.svg?raw';
import octiconChevronLeft from '../../public/assets/img/svg/octicon-chevron-left.svg?raw';
import octiconChevronRight from '../../public/assets/img/svg/octicon-chevron-right.svg?raw';
import octiconCircle from '../../public/assets/img/svg/octicon-circle.svg?raw';
import octiconClock from '../../public/assets/img/svg/octicon-clock.svg?raw';
import octiconCode from '../../public/assets/img/svg/octicon-code.svg?raw';
import octiconColumns from '../../public/assets/img/svg/octicon-columns.svg?raw';
import octiconCopy from '../../public/assets/img/svg/octicon-copy.svg?raw';
import octiconDiffAdded from '../../public/assets/img/svg/octicon-diff-added.svg?raw';
import octiconDiffModified from '../../public/assets/img/svg/octicon-diff-modified.svg?raw';
import octiconDiffRemoved from '../../public/assets/img/svg/octicon-diff-removed.svg?raw';
import octiconDiffRenamed from '../../public/assets/img/svg/octicon-diff-renamed.svg?raw';
import octiconDotFill from '../../public/assets/img/svg/octicon-dot-fill.svg?raw';
import octiconDownload from '../../public/assets/img/svg/octicon-download.svg?raw';
import octiconEye from '../../public/assets/img/svg/octicon-eye.svg?raw';
import octiconFile from '../../public/assets/img/svg/octicon-file.svg?raw';
import octiconFileCode from '../../public/assets/img/svg/octicon-file-code.svg?raw';
import octiconFileDirectoryFill from '../../public/assets/img/svg/octicon-file-directory-fill.svg?raw';
import octiconFileDirectoryOpenFill from '../../public/assets/img/svg/octicon-file-directory-open-fill.svg?raw';
import octiconFileRemoved from '../../public/assets/img/svg/octicon-file-removed.svg?raw';
import octiconFileSubmodule from '../../public/assets/img/svg/octicon-file-submodule.svg?raw';
import octiconFileSymlinkFile from '../../public/assets/img/svg/octicon-file-symlink-file.svg?raw';
import octiconFilter from '../../public/assets/img/svg/octicon-filter.svg?raw';
import octiconGear from '../../public/assets/img/svg/octicon-gear.svg?raw';
import octiconGitBranch from '../../public/assets/img/svg/octicon-git-branch.svg?raw';
import octiconGitCommit from '../../public/assets/img/svg/octicon-git-commit.svg?raw';
import octiconGitMerge from '../../public/assets/img/svg/octicon-git-merge.svg?raw';
import octiconGitPullRequest from '../../public/assets/img/svg/octicon-git-pull-request.svg?raw';
import octiconGitPullRequestClosed from '../../public/assets/img/svg/octicon-git-pull-request-closed.svg?raw';
import octiconGitPullRequestDraft from '../../public/assets/img/svg/octicon-git-pull-request-draft.svg?raw';
import octiconGrabber from '../../public/assets/img/svg/octicon-grabber.svg?raw';
import octiconHeading from '../../public/assets/img/svg/octicon-heading.svg?raw';
import octiconHistory from '../../public/assets/img/svg/octicon-history.svg?raw';
import octiconHorizontalRule from '../../public/assets/img/svg/octicon-horizontal-rule.svg?raw';
import octiconHome from '../../public/assets/img/svg/octicon-home.svg?raw';
import octiconImage from '../../public/assets/img/svg/octicon-image.svg?raw';
import octiconInfo from '../../public/assets/img/svg/octicon-info.svg?raw';
import octiconIssueClosed from '../../public/assets/img/svg/octicon-issue-closed.svg?raw';
import octiconIssueOpened from '../../public/assets/img/svg/octicon-issue-opened.svg?raw';
import octiconItalic from '../../public/assets/img/svg/octicon-italic.svg?raw';
import octiconKebabHorizontal from '../../public/assets/img/svg/octicon-kebab-horizontal.svg?raw';
import octiconLink from '../../public/assets/img/svg/octicon-link.svg?raw';
import octiconListOrdered from '../../public/assets/img/svg/octicon-list-ordered.svg?raw';
import octiconListUnordered from '../../public/assets/img/svg/octicon-list-unordered.svg?raw';
import octiconLock from '../../public/assets/img/svg/octicon-lock.svg?raw';
import octiconMeter from '../../public/assets/img/svg/octicon-meter.svg?raw';
import octiconMilestone from '../../public/assets/img/svg/octicon-milestone.svg?raw';
import octiconMirror from '../../public/assets/img/svg/octicon-mirror.svg?raw';
import octiconOrganization from '../../public/assets/img/svg/octicon-organization.svg?raw';
import octiconPlay from '../../public/assets/img/svg/octicon-play.svg?raw';
import octiconPlus from '../../public/assets/img/svg/octicon-plus.svg?raw';
import octiconProject from '../../public/assets/img/svg/octicon-project.svg?raw';
import octiconQuote from '../../public/assets/img/svg/octicon-quote.svg?raw';
import octiconRepo from '../../public/assets/img/svg/octicon-repo.svg?raw';
import octiconRepoForked from '../../public/assets/img/svg/octicon-repo-forked.svg?raw';
import octiconRepoTemplate from '../../public/assets/img/svg/octicon-repo-template.svg?raw';
import octiconRss from '../../public/assets/img/svg/octicon-rss.svg?raw';
import octiconScreenFull from '../../public/assets/img/svg/octicon-screen-full.svg?raw';
import octiconSearch from '../../public/assets/img/svg/octicon-search.svg?raw';
import octiconSidebarCollapse from '../../public/assets/img/svg/octicon-sidebar-collapse.svg?raw';
import octiconSidebarExpand from '../../public/assets/img/svg/octicon-sidebar-expand.svg?raw';
import octiconSkip from '../../public/assets/img/svg/octicon-skip.svg?raw';
import octiconStar from '../../public/assets/img/svg/octicon-star.svg?raw';
import octiconStop from '../../public/assets/img/svg/octicon-stop.svg?raw';
import octiconStrikethrough from '../../public/assets/img/svg/octicon-strikethrough.svg?raw';
import octiconSync from '../../public/assets/img/svg/octicon-sync.svg?raw';
import octiconTable from '../../public/assets/img/svg/octicon-table.svg?raw';
import octiconTag from '../../public/assets/img/svg/octicon-tag.svg?raw';
import octiconTrash from '../../public/assets/img/svg/octicon-trash.svg?raw';
import octiconTriangleDown from '../../public/assets/img/svg/octicon-triangle-down.svg?raw';
import octiconX from '../../public/assets/img/svg/octicon-x.svg?raw';
import octiconXCircleFill from '../../public/assets/img/svg/octicon-x-circle-fill.svg?raw';
import octiconZoomIn from '../../public/assets/img/svg/octicon-zoom-in.svg?raw';
import octiconZoomOut from '../../public/assets/img/svg/octicon-zoom-out.svg?raw';

const svgs = {
  'gitea-double-chevron-left': giteaDoubleChevronLeft,
  'gitea-double-chevron-right': giteaDoubleChevronRight,
  'gitea-empty-checkbox': giteaEmptyCheckbox,
  'gitea-exclamation': giteaExclamation,
  'gitea-favicon': giteaFavicon,
  'gitea-running': giteaRunning,
  'octicon-archive': octiconArchive,
  'octicon-arrow-left': octiconArrowLeft,
  'octicon-arrow-switch': octiconArrowSwitch,
  'octicon-blocked': octiconBlocked,
  'octicon-bold': octiconBold,
  'octicon-check': octiconCheck,
  'octicon-check-circle-fill': octiconCheckCircleFill,
  'octicon-checkbox': octiconCheckbox,
  'octicon-chevron-down': octiconChevronDown,
  'octicon-chevron-left': octiconChevronLeft,
  'octicon-chevron-right': octiconChevronRight,
  'octicon-circle': octiconCircle,
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
  'octicon-file-code': octiconFileCode,
  'octicon-file-directory-fill': octiconFileDirectoryFill,
  'octicon-file-directory-open-fill': octiconFileDirectoryOpenFill,
  'octicon-file-removed': octiconFileRemoved,
  'octicon-file-submodule': octiconFileSubmodule,
  'octicon-file-symlink-file': octiconFileSymlinkFile,
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
  'octicon-history': octiconHistory,
  'octicon-horizontal-rule': octiconHorizontalRule,
  'octicon-home': octiconHome,
  'octicon-image': octiconImage,
  'octicon-info': octiconInfo,
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
  'octicon-zoom-in': octiconZoomIn,
  'octicon-zoom-out': octiconZoomOut,
};

export type SvgName = keyof typeof svgs;

// TODO: use a more general approach to access SVG icons.
//  At the moment, developers must check, pick and fill the names manually,
//  most of the SVG icons in assets couldn't be used directly.

// retrieve an HTML string for given SVG icon name, size and additional classes
export function svg(name: SvgName, size = 16, classNames?: string | string[]): string {
  const className = Array.isArray(classNames) ? classNames.join(' ') : classNames;
  if (!(name in svgs)) throw new Error(`Unknown SVG icon: ${name}`);
  if (size === 16 && !className) return svgs[name];

  const document = parseDom(svgs[name], 'image/svg+xml');
  const svgNode = document.firstChild as SVGElement;
  if (size !== 16) {
    svgNode.setAttribute('width', String(size));
    svgNode.setAttribute('height', String(size));
  }
  if (className) svgNode.classList.add(...className.split(/\s+/).filter(Boolean as unknown as <T>(x: T | boolean) => x is T));
  return serializeXml(svgNode);
}

export function svgRaw(name: SvgName, size = 16, classNames?: string | string[]) {
  return htmlRaw(svg(name, size, classNames));
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
