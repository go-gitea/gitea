{
  _config+:: {
    local c = self,
    dashboardNamePrefix: 'CodeNest',
    dashboardTags: ['codenest'],
    dashboardPeriod: 'now-1h',
    dashboardTimezone: 'default',
    dashboardRefresh: '1m',

    // please see https://docs.gitea.com/administration/config-cheat-sheet#metrics-metrics
    // Show issue by repository metrics with format codenest_issues_by_repository{repository="org/repo"} 5.
    // Requires CodeNest 1.16.0 with ENABLED_ISSUE_BY_REPOSITORY set to true.
    showIssuesByRepository: true,
    // Show graphs for issue by label metrics with format codenest_issues_by_label{label="bug"} 2.
    // Requires CodeNest 1.16.0 with ENABLED_ISSUE_BY_LABEL set to true.
    showIssuesByLabel: true,

    // Requires CodeNest 1.16.0.
    showIssuesOpenClose: true,

    // add or remove metrics from dashboard
    codenestStatMetrics:
      [
        {
          name: 'codenest_organizations',
          description: 'Organizations',
        },
        {
          name: 'codenest_teams',
          description: 'Teams',
        },
        {
          name: 'codenest_users',
          description: 'Users',
        },
        {
          name: 'codenest_repositories',
          description: 'Repositories',
        },
        {
          name: 'codenest_milestones',
          description: 'Milestones',
        },
        {
          name: 'codenest_stars',
          description: 'Stars',
        },
        {
          name: 'codenest_releases',
          description: 'Releases',
        },
      ]
      +
      if c.showIssuesOpenClose then
        [
          {
            name: 'codenest_issues_open',
            description: 'Issues opened',
          },
          {
            name: 'codenest_issues_closed',
            description: 'Issues closed',
          },
        ] else
        [
          {
            name: 'codenest_issues',
            description: 'Issues',
          },
        ],
    //set this for using label colors on graphs
    issueLabels: [
      {
        label: 'bug',
        color: '#ee0701',
      },
      {
        label: 'duplicate',
        color: '#cccccc',
      },
      {
        label: 'invalid',
        color: '#e6e6e6',
      },
      {
        label: 'enhancement',
        color: '#84b6eb',
      },
      {
        label: 'help wanted',
        color: '#128a0c',
      },
      {
        label: 'question',
        color: '#cc317c',
      },
    ],
  },
}
