import octiconChevronDown from '../../public/img/svg/octicon-chevron-down.svg';
import octiconChevronRight from '../../public/img/svg/octicon-chevron-right.svg';
import octiconGitMerge from '../../public/img/svg/octicon-git-merge.svg';
import octiconGitPullRequest from '../../public/img/svg/octicon-git-pull-request.svg';
import octiconIssueClosed from '../../public/img/svg/octicon-issue-closed.svg';
import octiconIssueOpened from '../../public/img/svg/octicon-issue-opened.svg';
import octiconLink from '../../public/img/svg/octicon-link.svg';
import octiconLock from '../../public/img/svg/octicon-lock.svg';
import octiconMirror from '../../public/img/svg/octicon-mirror.svg';
import octiconRepo from '../../public/img/svg/octicon-repo.svg';
import octiconRepoForked from '../../public/img/svg/octicon-repo-forked.svg';
import octiconRepoTemplate from '../../public/img/svg/octicon-repo-template.svg';

export const svgs = {
  'octicon-chevron-down': octiconChevronDown,
  'octicon-chevron-right': octiconChevronRight,
  'octicon-git-merge': octiconGitMerge,
  'octicon-git-pull-request': octiconGitPullRequest,
  'octicon-issue-closed': octiconIssueClosed,
  'octicon-issue-opened': octiconIssueOpened,
  'octicon-link': octiconLink,
  'octicon-lock': octiconLock,
  'octicon-mirror': octiconMirror,
  'octicon-repo': octiconRepo,
  'octicon-repo-forked': octiconRepoForked,
  'octicon-repo-template': octiconRepoTemplate,
};

const parser = new DOMParser();
const serializer = new XMLSerializer();

// retrieve a HTML string for given SVG icon name and size in pixels
export function svg(name, size = 16) {
  if (name in svgs) {
    if (size === 16) return svgs[name];

    const document = parser.parseFromString(svgs[name], 'image/svg+xml');
    const svgNode = document.firstChild;
    svgNode.setAttribute('width', String(size));
    svgNode.setAttribute('height', String(size));
    return serializer.serializeToString(svgNode);
  }
  return '';
}
