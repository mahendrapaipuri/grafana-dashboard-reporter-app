# Grafana Dashboard Reporter

A Grafana plugin to create PDF reports from dashboard panels. This app has been 
heavily inspired from the original work [grafana-reporter](https://github.com/IzakMarais/reporter).
The core backend follows very closely to the original work and plugin app integrates 
the frontend components to be able to configure the reporter from the Configuration page.

By default the user needs to be authenticated with Grafana to access the service and 
must have role `Viewer` on the dashboard that user wants to create a PDF report. 

## Prerequisites

This plugin app depends on following:

- Grafana >= 10

- Another Grafana plugin 
[`grafana-image-renderer`](https://github.com/grafana/grafana-image-renderer) to render
panels into PNG files

- `pdflatex` must be installed on the host and be on `PATH` to compile TeX into PDFs

## Installation

### Installation via `grafana-cli`

TODO

### Local installation

Download the [latest Grafana Dashboard Reporter]().

Create a directory for grafana to access your custom-plugins 
_e.g._ `/var/lib/grafana/plugins/mahendrapaipuri-dashboardreporter-app`.

The following shell script downloads and extracts the latest plugin source 
code into the the current working directory. Run the following inside your grafana 
plugin directory:

```
cd /var/lib/grafana/plugins
./scripts/bootstrap-dashboard-reporter-app.sh
```

The current example assumes the following configuration is set for Grafana

- Edit the `paths.plugins` directive in your `grafana.ini`:

```
[paths]
plugins = /var/lib/grafana/plugins
```

- **OR** set the relevant environment variable where Grafana is started:

```
GF_PATHS_PLUGINS=/var/lib/grafana/plugins
```

### Install with Docker-compose

There is a docker compose file provided in the repo. Create a directory `dist` in the 
root of the repo and extract the latest version of the plugin app into this folder `dist`.
Once this is done, starting a Grafana server with plugin installed can be done 
as follows:

```
docker-compose -f docker-compose.yaml up
```

## Using plugin

The prerequisite is the user must have at least `Viewer` role on the dashboard that they 
want to create a PDF report. After the user authenticates with Grafana, creating a 
dashboard report is done by visiting the following end point

```
<grafanaAppUrl>/api/plugins/dashboard-reporter-app/resources/api?dashUid=<UID of dashboard>
```

In addition to `dashUid` query parameter, it is possible to pass time range query 
parameters `from`, `to` and also dashboard variables that have `var-` prefix. This 
permits to integrate the dashboard reporter app into Dashboard links. 

Following steps will configure a dashboard link to create PDF report for that dashboard

- Go to Settings of Dashboard
- Go to Links in the side bar and click on `Add Dashboard Link`
- Use Report for `Title` field, set `Type` to `Link`
- Now set `URL` to `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/api?dashUid=<UID of dashboard>`
- Set `Tooltip` to `Create a PDF report` and set `Icon` to `doc`
- By checking `Include current time range` and `Include current template variables values`, 
  time range and dashboard variables will be added to query parameters while creating 
  PDF report.

Now there should be link in the right top corner of dashboard named `Report` and clicking
this link will create a new PDF report of the dashboard.

## Configuring the plugin

- `Maximum Render Workers`: Number of concurrent workers to create PNGs of panels in the
  dashboard. Do not use too high value as it starve the machine

- `Use Grid Layout`: Use grid layout in the PDF report

- `TeX Template`: The template that will be used to create PDF reports. Default templates 
  are available in the [repo](../pkg/plugin/texTemplate.go).

## Development

See [DEVELOPMENT.md](../DEVELOPMENT.md)
