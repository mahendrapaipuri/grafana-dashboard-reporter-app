# Grafana Dashboard Reporter

A Grafana plugin to create PDF reports from dashboard panels. This app has been 
heavily inspired from the original work [grafana-reporter](https://github.com/IzakMarais/reporter).
The core backend follows very closely to the original work. Instead of using LaTeX 
to generate reports, the current plugin generates it from HTML templates using headless
chromium similar the reporting app in Grafana Enterprise offering. The plugin app also integrates 
the frontend components to be able to configure the reporter from the Configuration page.

By default the user needs to be authenticated with Grafana to access the service and 
must have role `Viewer` on the dashboard that user wants to create a PDF report. 

## Prerequisites

This plugin app depends on following:

- Grafana >= 10

- Another Grafana plugin 
[`grafana-image-renderer`](https://github.com/grafana/grafana-image-renderer) to render
panels into PNG files

- `chromium` must be installed on the host.

## Installation

### Installation via `grafana-cli`

Grafana Enterprise offers a very similar plugin [reports](https://grafana.com/docs/grafana/latest/dashboards/create-reports/#export-dashboard-as-pdf) 
and hence, their plugin policies do not allow to publish the current plugin in their
official catalog. 

It is important to note that the current plugin does not offer all the functionalities 
offered by Enterprise plugin and it is only relevant if users would like to create a
PDF report of a given dashboard. If users needs more advanced functionalities like 
generating and sending reports automatically, they should look into official plugin.

However, it is still possible to install this plugin on on-premise Grafana installations 
as an unsigned plugin. The installation procedure is briefed in 
[Local installation](#local-installation) section below.

### Local installation

Download the [latest Grafana Dashboard Reporter]().

Create a directory for grafana to access your custom-plugins 
_e.g._ `/var/lib/grafana/plugins/mahendrapaipuri-dashboardreporter-app`.

The following shell script downloads and extracts the latest plugin source 
code into the the current working directory. Run the following inside your grafana 
plugin directory:

```
cd /var/lib/grafana/plugins
curl https://raw.githubusercontent.com/mahendrapaipuri/grafana-dashboard-reporter-app/main/scripts/bootstrap-dashboard-reporter-app.sh | bash
```

This will install the latest release of plugin in the `/var/lib/grafana/plugins` folder 
and upon Grafana restart, the plugin will be loaded.

If user wants to install the latest nightly release, it is enough to add a environment
variable `NIGHTLY` to `bash`

```
cd /var/lib/grafana/plugins
curl https://raw.githubusercontent.com/mahendrapaipuri/grafana-dashboard-reporter-app/main/scripts/bootstrap-dashboard-reporter-app.sh | NIGHTLY=1 bash
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

## Configuring the plugin

All the configuration options can only be modified by `Admin` role.

- `Layout`: Layout of the report. Using grid layout renders the report as it is rendered
  in the browser. A simple layout will render the report with one panel per row

- `Orientation`: Portrait or Landscape

### Advanced parameters

- `Maximum Render Workers`: Number of concurrent workers to create PNGs of panels in the
  dashboard. Do not use too high value as it starve the machine

- `Persist Data`: Set it to `Enable` to inspect the generated HTML files from templates 
  and dashboard data. Use it to only debug the issues. When this option is turned on,
  the data files will be retained at `/var/lib/grafana/plugins/mahendrapaipuri-dashboardreporter-app/staging/debug`
  folder.

## Using plugin

The prerequisite is the user must have at least `Viewer` role on the dashboard that they 
want to create a PDF report. After the user authenticates with Grafana, creating a 
dashboard report is done by visiting the following end point

```
<grafanaAppUrl>/api/plugins/mahendrapaipuri-reporter-app/resources/report?dashUid=<UID of dashboard>
```

In addition to `dashUid` query parameter, it is possible to pass time range query 
parameters `from`, `to` and also dashboard variables that have `var-` prefix. This 
permits to integrate the dashboard reporter app into Dashboard links. 

The layout and orientation options can be passed by query parameters which will override 
the global values set by admins in the plugin configuration. `layout` will take either 
`simple` or `grid` as query parameter and `orientation` will take `portrait` or 
`landscape` as parameters.

Following steps will configure a dashboard link to create PDF report for that dashboard

- Go to Settings of Dashboard
- Go to Links in the side bar and click on `Add Dashboard Link`
- Use Report for `Title` field, set `Type` to `Link`
- Now set `URL` to `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>`
- Set `Tooltip` to `Create a PDF report` and set `Icon` to `doc`
- By checking `Include current time range` and `Include current template variables values`, 
  time range and dashboard variables will be added to query parameters while creating 
  PDF report.

Now there should be link in the right top corner of dashboard named `Report` and clicking
this link will create a new PDF report of the dashboard.

## Development

See [DEVELOPMENT.md](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/DEVELOPMENT.md)
