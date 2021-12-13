local grafana = import 'github.com/grafana/grafonnet-lib/grafonnet/grafana.libsonnet';
local prometheus = grafana.prometheus;

{

  grafanaDashboards+:: {

    local giteaSelector = 'job="$job", instance="$instance"',
    local giteaStatsPanel = grafana.statPanel.new(
                              'Gitea stats',
                              datasource='$datasource',
                              reducerFunction='last',
                              graphMode='none',
                              colorMode='value',
                            )
                            .addTargets(
                              [
                                prometheus.target(expr='%s{%s}' % [metric.name, giteaSelector], legendFormat=metric.description, intervalFactor=10)
                                for metric in $._config.giteaStatMetrics
                              ]
                            )
                            + {
                              fieldConfig+: {
                                defaults+: {
                                  color: {
                                    fixedColor: 'blue',
                                    mode: 'fixed',
                                  },
                                },
                              },
                            },

    local giteaUptimePanel = grafana.statPanel.new(
                               'Uptime',
                               datasource='$datasource',
                               reducerFunction='last',
                               graphMode='area',
                               colorMode='value',
                             )
                             .addTarget(prometheus.target(expr='time()-process_start_time_seconds{%s}' % giteaSelector, intervalFactor=1))
                             + {
                               fieldConfig+: {
                                 defaults+: {
                                   color: {
                                     fixedColor: 'blue',
                                     mode: 'fixed',
                                   },
                                   unit: 's',
                                 },
                               },
                             },

    local giteaMemoryPanel = grafana.graphPanel.new(
      'Memory(rss) usage',
      datasource='$datasource',
      format='decbytes',
      lines=true,
      fill=1,
      legend_show=false
    )
                             .addTarget(prometheus.target(expr='process_resident_memory_bytes{%s}' % giteaSelector, intervalFactor=2)),

    local giteaCpuPanel = grafana.graphPanel.new(
      'CPU usage',
      datasource='$datasource',
      format='percent',
      lines=true,
      fill=1,
      legend_show=false
    )
                          .addTarget(prometheus.target(expr='rate(process_cpu_seconds_total{%s}[$__rate_interval])*100' % giteaSelector, intervalFactor=2)),

    local giteaFileDescriptorsPanel = grafana.graphPanel.new(
      'File descriptors usage',
      datasource='$datasource',
      format='',
      lines=true,
      fill=1,
      legend_show=false
    )
                                      .addTarget(prometheus.target(expr='process_open_fds{%s}' % giteaSelector, intervalFactor=2))
                                      .addTarget(prometheus.target(expr='process_max_fds{%s}' % giteaSelector, intervalFactor=2))
                                      .addSeriesOverride(
      {
        alias: '/process_max_fds.+/',
        color: '#F2495C',  // red
        dashes: true,
        fill: 0,
      },
    ),

    local giteaChangesPanel = grafana.graphPanel.new(
      '',
      datasource='$datasource',
      lines=false,
      points=false,
      bars=true,
      stack=true,
      interval="$agg_interval",
      maxDataPoints=10000,
      legend_values=true,
      legend_total=true,
    )
                              .addTarget(prometheus.target(expr='changes(process_start_time_seconds{%s}[$__interval]) > 0' % [giteaSelector], legendFormat='Restarts', intervalFactor=1))
                              .addTargets(
      [
        prometheus.target(expr='floor(increase(%s{%s}[$__interval])) > 0' % [metric.name, giteaSelector], legendFormat=metric.description, intervalFactor=1)
        for metric in $._config.giteaStatMetrics
      ]
    ),
    local giteaChangesPanelTotal = grafana.statPanel.new(
                              '',
                              datasource='-- Dashboard --',
                              reducerFunction='sum',
                              graphMode='none',
                              colorMode='value',
                            )
                            + {
                              targets+: [
                                  {
                                    panelId: 10, // id of giteaChangesPanel
                                    refId: "A"
                                  },
                              ],
                            }
                            + {
                              fieldConfig+: {
                                defaults+: {
                                  color: {
                                    fixedColor: 'blue',
                                    mode: 'fixed',
                                  },
                                },
                              },
                            },

    'gitea-overview.json':
      grafana.dashboard.new(
        '%s Overview' % $._config.dashboardNamePrefix,
        time_from='now-1h',
        editable=false,
        tags=($._config.dashboardTags),
        timezone='utc',
        refresh='1m',
        graphTooltip='shared_crosshair',
        uid='gitea-overview'
      )
      .addTemplate(
        {
          current: {
            text: 'Prometheus',
            value: 'Prometheus',
          },
          hide: 0,
          label: null,
          name: 'datasource',
          options: [],
          query: 'prometheus',
          refresh: 1,
          regex: '',
          type: 'datasource',
        },
      )
      .addTemplate(
        {
          hide: 0,
          label: null,
          name: 'job',
          options: [],
          query: 'label_values(gitea_organizations, job)',
          refresh: 1,
          regex: '',
          type: 'query',
        },
      )
      .addTemplate(
        {
          hide: 0,
          label: null,
          name: 'instance',
          options: [],
          query: 'label_values(gitea_organizations{job="$job"}, instance)',
          refresh: 1,
          regex: '',
          type: 'query',
        },
      )
     .addTemplate(
        {
          hide: 0,
          label: 'aggregation interval',
          name: 'agg_interval',
          current: 'auto',
          auto_min: '1m',
          auto: true,
          query: '1m,10m,1h,1d,7d',
          type: 'interval',
        },
      )
      .addPanel(
        grafana.row.new(title='General'), gridPos={
          x: 0,
          y: 0,
          w: 0,
          h: 0,
        },
      )
      .addPanel(
        giteaStatsPanel, gridPos={
          x: 0,
          y: 0,
          w: 16,
          h: 4,
        }
      )
      .addPanel(
        giteaUptimePanel, gridPos={
          x: 16,
          y: 0,
          w: 8,
          h: 4,
        }
      )
      .addPanel(
        giteaMemoryPanel, gridPos={
          x: 0,
          y: 4,
          w: 8,
          h: 6,
        }
      )
      .addPanel(
        giteaCpuPanel, gridPos={
          x: 8,
          y: 4,
          w: 8,
          h: 6,
        }
      )
      .addPanel(
        giteaFileDescriptorsPanel, gridPos={
          x: 16,
          y: 4,
          w: 8,
          h: 6,
        }
      )
      .addPanel(
        grafana.row.new(
          title='Changes',
          collapse=false
        ),
        gridPos={
          x: 0,
          y: 10,
          w: 24,
          h: 8,
        }
      )
      .addPanel(
        giteaChangesPanelTotal,
        gridPos={
          x: 0,
          y: 12,
          w: 6,
          h: 8,
        }
      )
      .addPanel(
        giteaChangesPanel,
        gridPos={
          x: 6,
          y: 12,
          w: 18,
          h: 8,
        }
      ),

  },
}
