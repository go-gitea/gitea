# Gitea Mixin

Gitea Mixin is a set of configurable, reusable, and extensible alerts and
dashboards based on the metrics exported by the Gitea built-in metrics endpoint. The mixin creates
recording and alerting rules for Prometheus and suitable dashboard descriptions
for Grafana.

## Generate config files

You can manually generate the alerts, dashboards and rules files, but first you
must install some tools:

```bash
go get github.com/jsonnet-bundler/jsonnet-bundler/cmd/jb
go get github.com/google/go-jsonnet/cmd/jsonnet
# or in brew: brew install go-jsonnet
```

For linting and formatting, you would also need `mixtool` and `jsonnetfmt` installed. If you
have a working Go development environment, it's easiest to run the following:

```bash
go get github.com/monitoring-mixins/mixtool/cmd/mixtool
go get github.com/google/go-jsonnet/cmd/jsonnetfmt
```

The `prometheus_alerts.yaml` and `prometheus_rules.yaml` file then need to passed
to your Prometheus server, and the files in `dashboards_out` need to be imported
into your Grafana server.  The exact details will be depending on your environment.

Edit `config.libsonnet` if required and then build JSON dashboard files for Grafana:

```bash
make
```

For more advanced uses of mixins, see
https://github.com/monitoring-mixins/docs.
