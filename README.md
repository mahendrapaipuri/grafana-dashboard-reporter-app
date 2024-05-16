# Grafana Dashboard Reporter App

|         |                                                                                                                                                                                                                                                                                                                                                                                                                 |
| ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| CI/CD   | [![ci](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/workflows/CI/badge.svg)](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app)                                                |
| Docs    | [![docs](https://img.shields.io/badge/docs-passing-green?style=flat&link=https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/src/README.md)](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/src/README.md)                                                                                                                                                                                                                               |
| Package | [![Release](https://img.shields.io/github/v/release/mahendrapaipuri/grafana-dashboard-reporter-app.svg?include_prereleases)](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/releases/latest)                                                                                                                                                                     |
| Meta    | [![GitHub License](https://img.shields.io/github/license/mahendrapaipuri/grafana-dashboard-reporter-app)](https://gitlab.com/mahendrapaipuri/grafana-dashboard-reporter-app) [![Go Report Card](https://goreportcard.com/badge/github.com/mahendrapaipuri/grafana-dashboard-reporter-app)](https://goreportcard.com/report/github.com/mahendrapaipuri/grafana-dashboard-reporter-app) [![code style](https://img.shields.io/badge/code%20style-gofmt-blue.svg)](https://pkg.go.dev/cmd/gofmt) |

This Grafana plugin app can create PDF reports of a given dashboard using headless `chromium` 
and [`grafana-image-renderer`](https://github.com/grafana/grafana-image-renderer).

![Sample report](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/pngs/sample_report.png)

This plugin is based on the original work 
[grafana-reporter](https://github.com/IzakMarais/reporter). 
The core of the plugin is heavily inspired from the above stated work with some 
improvements and modernization. 

- The current plugin uses HTML templates and headless chromium to generate reports 
  instead of LaTeX. `grafana-image-renderer` is a prerequisite for both current and 
  original plugins.

- The current plugin app exposes the reporter as a custom API end point of Grafana instance without 
  needing to run the [grafana-reporter](https://github.com/IzakMarais/reporter) 
  as a separate web service. The advantage of the plugin approach is the authenticated 
  access to the reporter app is guaranteed by Grafana auth.

- The plugin is capable of including all the repeated rows and/or panels in the 
  generated report.

- The plugin can be configured by Admins and users either from 
  [Configuration Page](./src/img/light.png) or query parameters to the report API.

## Documentation

More documentation can be found in [README](./src/README.md)
