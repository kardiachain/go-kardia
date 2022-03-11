# Monitoring metrics on Grafana

### Installation Prerequisites
* Install [prometheus](https://prometheus.io/docs/prometheus/latest/getting_started/).
* Install [grafana](https://grafana.com/docs/grafana/latest/installation/).

### Configuring Prometheus to monitor node
Save the following Prometheus configuration as a file named prometheus.yml:
```
global:
  scrape_interval:     15s # By default, scrape targets every 15 seconds.

scrape_configs:
  - job_name: 'node1'
    static_configs:
      - targets: ['<hostname>:6001']
```
To start Prometheus with your newly created configuration file, change to the directory containing the Prometheus binary and run:
```
./prometheus --config.file=prometheus.yml
```

### Check Prometheus metrics in Grafana Explore view
See [here](https://grafana.com/docs/grafana/latest/getting-started/getting-started-prometheus/) for more details.