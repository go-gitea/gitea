package metrics

import (
	"code.gitea.io/gitea/models"
	"github.com/prometheus/client_golang/prometheus"
)

//User: 1,
//Org: 1,
//PublicKey: 0,
//Repo: 1,
//Watch: 1,
//Star: 0,
//Action: 1,
//Access: 0,
//Issue: 0,
//Comment: 0,
//Oauth: 0,
//Follow: 0,
//Mirror: 0,
//Release: 0,
//LoginSource: 0,
//Webhook: 0,
//Milestone: 0,
//Label: 0,
//HookTask: 0,
//Team: 1,
//UpdateTask: 0,
//Attachment: 0

type Collector struct {
	Users         *prometheus.Desc
	Organizations *prometheus.Desc
	PublicKeys *prometheus.Desc
}

func NewCollector() Collector {
	return Collector{
		Users: prometheus.NewDesc(
			"gitea_users",
			"Number of users registered with gitea",
			nil, nil,
		),
		Organizations:prometheus.NewDesc(
			"gitea_origanizations",
			"Number of gitea origanizations",
			nil,nil,
		),
		PublicKeys:prometheus.NewDesc(
			"gitea_public_keys",
			"Number of public keys in gitea",
			nil,nil,
		),
	}
}

func (c Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Users
	ch <- c.Organizations
	ch <- c.PublicKeys
}

func (c Collector) Collect(ch chan<- prometheus.Metric) {
	stats := models.GetStatistic()

	ch <- prometheus.MustNewConstMetric(
		c.Users,
		prometheus.GaugeValue,
		float64(stats.Counter.User),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Organizations,
		prometheus.GaugeValue,
		float64(stats.Counter.Org),
	)
	ch <- prometheus.MustNewConstMetric(
		c.PublicKeys,
		prometheus.GaugeValue,
		float64(stats.Counter.PublicKey),
	)
}
