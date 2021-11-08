{
  _config+:: {
    local c = self,
    dashboardNamePrefix: 'Gitea',
    dashboardTags: ['gitea'],

    // add or remove metrics from dashboard
    giteaStatMetrics: [
      {
        name: 'gitea_organizations',
        description: 'Organizations',
      },
      {
        name: 'gitea_teams',
        description: 'Teams',
      },
      {
        name: 'gitea_users',
        description: 'Users',
      },
      {
        name: 'gitea_repositories',
        description: 'Repositories',
      },
      {
        name: 'gitea_milestones',
        description: 'Milestones',
      },
      {
        name: 'gitea_stars',
        description: 'Stars',
      },
      {
        name: 'gitea_releases',
        description: 'Releases',
      },
      {
        name: 'gitea_issues',
        description: 'Issues',
      },
      {
        name: 'gitea_comments',
        description: 'Comments',
      },
    ],
  },
}
