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

- If `grafana-image-renderer` is installed as Grafana plugin, no other external 
dependencies are required for the plugin to work. `grafana-image-renderer` ships the 
plugin with a standalone instance of `chromium` and the same `chromium` will be used 
to render PDF reports. If `grafana-image-renderer` is deployed as a service on a 
different host, `chromium` must be installed on the host where Grafana is installed. 

> [!IMPORTANT] 
> `grafana-image-renderer` advises to install `chromium` to ensure that all the 
dependent libraries of the `chromium` are available on the host.

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

Download the [latest Grafana Dashboard Reporter](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/releases/latest).

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
data = /var/lib/grafana
```

- **OR** set the relevant environment variable where Grafana is started:

```
GF_PATHS_DATA=/var/lib/grafana
```
> [!IMPORTANT] 
> The final step is to _whitelist_ the plugin as it is an unsigned plugin and Grafana,
by default, does not load any unsigned plugins even if they are installed. In order to
whitelist the plugin, we need to add following to the Grafana configuration file

```
[plugins]
allow_loading_unsigned_plugins = mahendrapaipuri-dashboardreporter-app
```

Once this configuration is added, restart the Grafana server and it should load the 
plugin. The loading of plugin can be verified by the following log lines

```
logger=plugin.signature.validator t=2024-03-21T11:16:54.738077851Z level=warn msg="Permitting unsigned plugin. This is not recommended" pluginID=mahendrapaipuri-dashboardreporter-app
logger=plugin.loader t=2024-03-21T11:16:54.738166325Z level=info msg="Plugin registered" pluginID=mahendrapaipuri-dashboardreporter-app

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

After successful installation of the plugin, it will be, by default, disabled. We can 
enable it in different ways.

- From Grafana UI, navigating to `Apps > Dashboard Reporter App > Configuration` will
show [this page](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/src/img/light.png) 
and plugin can be enabled there. The configuration page can also be
accessed by URL `<Grafana URL>/plugins/mahendrapaipuri-dashboardreporter-app`. 
> [!NOTE]
> The warning about `Invalid plugin signature` is not fatal and it is simply saying
that plugin has not been signed by Grafana Labs.

- By using [Grafana Provisioning](https://grafana.com/docs/grafana/latest/administration/provisioning/).
An example provision config is provided in the [repo](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/provisioning/plugins/app.yaml)
and it can be installed at `/etc/grafana/provisioning/plugins/reporter.yml`. After installing
this YAML file, by restarting Grafana server, the plugin will be enabled with config
options set in the `reporter.yml` file.

Grafana Provisioning is a programatic way of configuring the plugin app. However, it is 
possible to configure the app from Grafana UI as well as explained in the first option.
Different configuration options are explained below:

### Report parameters

All the configuration options can only be modified by `Admin` role.

- `Layout`: Layout of the report. Using grid layout renders the report as it is rendered
  in the browser. A simple layout will render the report with one panel per row

- `Orientation`: Portrait or Landscape

- `Dashboard Mode`: Whether to render default dashboard or full dashboard. In default 
  mode, collapsed rows are ignored and only visible panels are included in the report.
  Whereas in full mode, rows are uncollapsed and all the panels are included in the 
  report

Although these options can only be changed by users with `Admin` role for whole instance
of Grafana, it is possible to override the global defaults for a particular report
by using query parameters. It is enough to add query parameters to dashboard report URL
to set these values.

- Query field for layout is `layout` and it takes either `simple` or `grid` as value.
  Example is `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>&layout=grid`

- Query field for orientation is `orientation` and it takes either `portrait` or `landscape`
  as value. Example is `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>&orientation=landscape`

- Query field for dashboard mode is `dashboardMode` and it takes either `default` or `full`
  as value. Example is `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>&dashboardMode=full`

### Advanced parameters

- `Maximum Render Workers`: Number of concurrent workers to create PNGs of panels in the
  dashboard. Do not use too high value as it starve the machine

- `Persist Data`: Enable it to inspect the generated HTML files from templates 
  and dashboard data. Use it to only debug the issues. When this option is turned on,
  the data files will be retained at `/var/lib/grafana/plugins/reports/debug` folder.


## Using plugin

The prerequisite is the user must have at least `Viewer` role on the dashboard that they 
want to create a PDF report. After the user authenticates with Grafana, creating a 
dashboard report is done by visiting the following end point

```
<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>
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

## Examples

Here are the example reports that are generated out of the test dashboards

- [Report with portrait orientation, simple layout and full dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_portrait_simple_full.pdf)
- [Report with landscape orientation, simple layout and full dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_landscape_simple_full.pdf)
- [Report with portrait orientation, grid layout and full dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_portrait_grid_full.pdf)
- [Report with landscape orientation, grid layout and full dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_landscape_grid_full.pdf)
- [Report with portrait orientation, grid layout and default dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_portrait_grid_default.pdf)

## Development

See [DEVELOPMENT.md](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/DEVELOPMENT.md)

## Troubleshooting

- If `chromium` fails to run, it suggests that there are missing dependent libraries on the host. In that case, we advise to install `chromium` on the machine which will install all the dependent libraries.

- If the generated report is malformed, we suggest to turn on `Persist Data Files` config option of the plugin from Grafana UI and re-run the report generation. Now, the files created by the plugin will be persisted at `$GF_DATA_PATH/reports/debug` folder. While reporting bugs, please attach the found `report.html` along with JSON
model of dashboard.
