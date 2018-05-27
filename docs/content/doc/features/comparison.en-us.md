---
date: "2018-05-07T13:00:00+02:00"
title: "Gitea compared to other Git hosting options"
slug: "comparison"
weight: 5
toc: true
draft: false
menu:
  sidebar:
    parent: "features"
    name: "Comparison"
    weight: 5
    identifier: "comparison"
---

# Gitea compared to other Git hosting options

To help decide if Gitea is suited for your needs here is how it compares to other Git self hosted options.

Be warned that we don't regularly check for feature changes in other products so this list can be outdated. If you find anything that needs to be updated in table below please report [issue on Github](https://github.com/go-gitea/gitea/issues).

_Symbols used in table:_

* _✓ - supported_

* _⁄ - supported with limited functionality_

* _✘ - unsupported_

<table border="1" cellpadding="4">
  <thead>
    <tr>
      <td>Feature</td>
      <td>Gitea</td>
      <td>Gogs</td>
      <td>GitHub EE</td>
      <td>GitLab CE</td>
      <td>GitLab EE</td>
      <td>BitBucket</td>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>Open source and free</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✘</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Issue tracker</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Pull/Merge requests</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Squash merging</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Rebase merging</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
      <td>⁄</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Pull/Merge request inline comments</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Pull/Merge request approval</td>
      <td>✘</td>
      <td>✘</td>
      <td>⁄</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Merge conflict resolution</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Restrict push and merge access to certain users</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>⁄</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Markdown support</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Issues and pull/merge requests templates</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Revert specific commits or a merge request</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Labels</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Time tracking</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Multiple assignees for issues</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Related issues</td>
      <td>✘</td>
      <td>✘</td>
      <td>⁄</td>
      <td>✘</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Confidential issues</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Comment reactions</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Lock Discussion</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Batch issue handling</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Issue Boards</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Create new branches from issues</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Commit graph</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Web code editor</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Branch manager</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Create new branches</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Repository topics</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Repository code search</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Global code search</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Issue search</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Global issue search</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Git LFS 2.0</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>⁄</td>
    </tr>
    <tr>
      <td>Integrated Git-powered wiki</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Static Git-powered pages</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Group Milestones</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Granular user roles (Code, Issues, Wiki etc)</td>
      <td>✓</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Cherry-picking changes</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>GPG Signed Commits</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Reject unsigned commits</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Verified Committer</td>
      <td>✘</td>
      <td>✘</td>
      <td>?</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Subgroups: groups within groups</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Custom Git Hooks</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Repository Activity page</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Deploy Tokens</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Repository Tokens with write rights</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Easy upgrade process</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Built-in Container Registry</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>External git mirroring</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>AD / LDAP integration</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Multiple LDAP / AD server support</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>LDAP user synchronization</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>OpenId Connect support</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>?</td>
    </tr>
    <tr>
      <td>OAuth 2.0 integration (external authorization)</td>
      <td>✓</td>
      <td>✘</td>
      <td>⁄</td>
      <td>✓</td>
      <td>✓</td>
      <td>?</td>
    </tr>
    <tr>
      <td>Act as OAuth 2.0 provider</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Two factor authentication (2FA)</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>FIDO U2F (2FA)</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Webhook support</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Mattermost/Slack integration</td>
      <td>✓</td>
      <td>✓</td>
      <td>⁄</td>
      <td>✓</td>
      <td>✓</td>
      <td>⁄</td>
    </tr>
    <tr>
      <td>Discord integration</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Built-in CI/CD</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>External CI/CD status display</td>
      <td>✓</td>
      <td>✘</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Multiple database support</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
      <td>⁄</td>
      <td>⁄</td>
      <td>✓</td>
    </tr>
    <tr>
      <td>Multiple OS support</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
    </tr>
    <tr>
      <td>Low resource usage (RAM/CPU)</td>
      <td>✓</td>
      <td>✓</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
      <td>✘</td>
    </tr>
  </tbody>
</table>
