# Grafana Dashboard Reporter App

|         |                                                                                                                                                                                                                                                                                                                                                                                                                 |
| ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| CI/CD   | [![ci](https://github.com/asanluis/grafana-dashboard-reporter-app/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/asanluis/grafana-dashboard-reporter-app/actions/workflows/ci.yml?query=branch%3Amain)                                                |
| Docs    | [![docs](https://img.shields.io/badge/docs-passing-green?style=flat&link=https://github.com/asanluis/grafana-dashboard-reporter-app/blob/main/src/README.md)](https://github.com/asanluis/grafana-dashboard-reporter-app/blob/main/src/README.md)                                                                                                                                                                                                                               |
| Package | [![Release](https://img.shields.io/github/v/release/mahendrapaipuri/grafana-dashboard-reporter-app.svg?include_prereleases)](https://github.com/asanluis/grafana-dashboard-reporter-app/releases/latest)                                                                                                                                                                     |
| Meta    | [![GitHub License](https://img.shields.io/github/license/mahendrapaipuri/grafana-dashboard-reporter-app)](https://gitlab.com/mahendrapaipuri/grafana-dashboard-reporter-app) [![Go Report Card](https://goreportcard.com/badge/github.com/asanluis/grafana-dashboard-reporter-app)](https://goreportcard.com/report/github.com/asanluis/grafana-dashboard-reporter-app) [![code style](https://img.shields.io/badge/code%20style-gofmt-blue.svg)](https://pkg.go.dev/cmd/gofmt) |

This Grafana plugin app can create PDF reports of a given dashboard using headless `chromium`
and [`grafana-image-renderer`](https://github.com/grafana/grafana-image-renderer).

![Sample report](https://github.com/asanluis/grafana-dashboard-reporter-app/blob/main/docs/pngs/sample_report.png)

## 🎯 Features

This plugin has been inspired from the original work
[grafana-reporter](https://github.com/IzakMarais/reporter).

- The current plugin uses HTML templates and headless chromium to generate reports.
  `grafana-image-renderer` is a prerequisite for generating panel PNGs.

- The current plugin app exposes the reporter as a custom API end point of Grafana.
  The advantage of the plugin approach is the authenticated
  access to the reporter app is guaranteed by Grafana auth.

- The plugin is capable of including all the repeated rows and/or panels in the
  generated report.

- The plugin can include selected panels data into report which can be chosen using
  query parameters to the report API.

- The plugin can be configured by Admins from [Configuration Page](./src/img/light.png)
  and users using query parameters to the report API.

## ⚡️ Documentation

More documentation can be found in [README](./src/README.md)

## ⭐️ Project assistance

If you want to say **thank you** or/and support active development of plugin:

- Add a [GitHub Star](https://github.com/asanluis/grafana-dashboard-reporter-app) to the project.
- Write articles about project on [Dev.to](https://dev.to/), [Medium](https://medium.com/) or personal blog.
