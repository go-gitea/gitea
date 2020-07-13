import octiconChevronDown from '../../public/img/svg/octicon-chevron-down.svg';
import octiconChevronRight from '../../public/img/svg/octicon-chevron-right.svg';
import octiconGitMerge from '../../public/img/svg/octicon-git-merge.svg';
import octiconGitPullRequest from '../../public/img/svg/octicon-git-pull-request.svg';
import octiconInternalRepo from '../../public/img/svg/octicon-internal-repo.svg';
import octiconIssueClosed from '../../public/img/svg/octicon-issue-closed.svg';
import octiconIssueOpened from '../../public/img/svg/octicon-issue-opened.svg';
import octiconLink from '../../public/img/svg/octicon-link.svg';
import octiconLock from '../../public/img/svg/octicon-lock.svg';
import octiconRepo from '../../public/img/svg/octicon-repo.svg';
import octiconRepoClone from '../../public/img/svg/octicon-repo-clone.svg';
import octiconRepoForked from '../../public/img/svg/octicon-repo-forked.svg';
import octiconRepoTemplate from '../../public/img/svg/octicon-repo-template.svg';
import octiconRepoTemplatePrivate from '../../public/img/svg/octicon-repo-template-private.svg';

export const svgs = {
  'octicon-chevron-down': octiconChevronDown,
  'octicon-chevron-right': octiconChevronRight,
  'octicon-git-merge': octiconGitMerge,
  'octicon-git-pull-request': octiconGitPullRequest,
  'octicon-internal-repo': octiconInternalRepo,
  'octicon-issue-closed': octiconIssueClosed,
  'octicon-issue-opened': octiconIssueOpened,
  'octicon-link': octiconLink,
  'octicon-lock': octiconLock,
  'octicon-repo': octiconRepo,
  'octicon-repo-clone': octiconRepoClone,
  'octicon-repo-forked': octiconRepoForked,
  'octicon-repo-template': octiconRepoTemplate,
  'octicon-repo-template-private': octiconRepoTemplatePrivate,
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
