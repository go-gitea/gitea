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
  //set this for using label colors on graphs
  issueLabels: [
      {
        label: "bug",
        color: "#ee0701"
      },
      {
        label: "duplicate",
        color: "#cccccc"
      },
      {
        label: "invalid",
        color: "#e6e6e6"
      },
      {
        label: "enhancement",
        color: "#84b6eb"
      },
      {
        label: "help wanted",
        color: "#128a0c"
      },
      {
        label: "question",
        color: "#cc317c"
      },
    ]
  },
}
