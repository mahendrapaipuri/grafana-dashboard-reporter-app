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
parameters set in the `reporter.yml` file.

Grafana Provisioning is a programmatic way of configuring the plugin app. However, it is 
possible to configure the app from Grafana UI as well as explained in the first option.

Different configuration parameters are explained below:

### Grafana related parameters

The following configuration parameters are directly tied to Grafana instance

- `appUrl`: The URL at which Grafana is running. By default, `http://localhost:3000` is
  used which should work for most of the deployments. If environment variable `GF_APP_URL`
  is set, that will take the precedence over the value configured in the provisioning file. 

- `skipTlsCheck`: If Grafana instance is configured to use TLS with self signed certificates
  set this parameter to `true` to skip TLS certificate check. This can be set using 
  environment variable `IGNORE_HTTPS_ERRORS` as well. If the environment variable is 
  found, it will take precedence over the value set it the config.

> [!IMPORTANT] 
> These config parameters are dependent on Grafana instance 
and cannot be changed without restarting Grafana instance. Hence, these config parameters 
can only be set using provisioning method and **it is not possible to configure them in Grafana UI**.

> [!NOTE] 
> Starting from `v1.4.0`, config parameter `dataPath` is not needed anymore as the plugin
will get the Grafana's data path based on its own executable path. If the existing provisioned
configs have this parameter set, it will be ignored while loading the plugin's configuration.

### Authentication settings

This config section allows to configure authentication related settings.

- `Service Account Token`: A service account token that will be used to generate reports
  _via_ API requests. More details on how to use it is briefed in 
  [Using Grafana API](#using-grafana-api) section.

### Report settings

All the configuration parameters can only be modified by `Admin` role from Grafana UI.

- `Layout`: Layout of the report. Using grid layout renders the report as it is rendered
  in the browser. A simple layout will render the report with one panel per row

- `Orientation`: Portrait or Landscape

- `Dashboard Mode`: Whether to render default dashboard or full dashboard. In default 
  mode, collapsed rows are ignored and only visible panels are included in the report.
  Whereas in full mode, rows are un collapsed and all the panels are included in the 
  report

- `Time Zone`: The time zone that will be used in the report. It has to conform to the 
  [IANA format](https://www.iana.org/time-zones). By default, local Grafana server's 
  time zone will be used.

- `Branding Logo`: This parameter takes a base64 encoded image that will be included
  in the footer of each page in the report. Typically, operators can include their 
  organization logos to have "customized" reports. Images of format PNG and JPG are 
  accepted.

#### Overriding report settings

Although report settings can only be modified by users with `Admin` role for whole instance
of Grafana, it is possible to override the global defaults for a particular report
by using query parameters. It is enough to add query parameters to dashboard report URL
to set these values. Currently, the supported query parameters are:

- Query field for layout is `layout` and it takes either `simple` or `grid` as value.
  Example is `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>&layout=grid`

- Query field for orientation is `orientation` and it takes either `portrait` or `landscape`
  as value. Example is `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>&orientation=landscape`

- Query field for dashboard mode is `dashboardMode` and it takes either `default` or `full`
  as value. Example is `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>&dashboardMode=full`

- Query field for dashboard mode is `timeZone` and it takes a value in [IANA format](https://www.iana.org/time-zones)
  as value. **Note** that it should be encoded to escape URL specific characters. For example
  to use `America/New_York` query parameter should be 
  `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>&timeZone=America%2FNew_York`

Besides there are two special query parameters available namely:

- `includePanelID`: This can be used to include only panels with IDs set in the query in 
  the generated report. An example can be 
  `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>&includePanelID=1&includePanelID=5&includePanelID=8`.
  This request will only include the panels `1`, `5` and `8` in the report and ignoring the rest. 
  When `grid` layout is used with `includePanelID`, the report layout will leave the gaps 
  in the place of panels that are not included in the report.

- `excludePanelID`: This can be used to exclude any unwanted panels in 
  the generated report. An example can be 
  `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>&excludePanelID=2&excludePanelID=7`.
  This request will only exclude panels `2`, and `7` in the report and including the rest. 
  When `grid` layout is used with `excludePanelID`, the report layout will leave the gaps 
  in the place of panels that are excluded in the report.

> [!NOTE] 
> If a given panel ID is set in both `includePanelID` and `excludePanelID` query parameter,
  it will be **included** in the report. Query parameter `includePanelID` has more 
  precedence over `excludePanelID`.

### Additional settings

Additional settings that can only be modified by the users with `Admin` role.

- `Maximum Render Workers`: Number of concurrent workers to create PNGs of panels in the
  dashboard. Do not use too high value as it starve the machine

## Using plugin

### Using Grafana web UI

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

### Using Grafana API

The plugin can generate reports programmatically using Grafana API by using 
[Grafana service accounts](https://grafana.com/docs/grafana/latest/administration/service-accounts/).

Once a service account is created with appropriate permissions by following 
[Grafana docs](https://grafana.com/docs/grafana/latest/administration/service-accounts/#to-create-a-service-account), 
generate an [API token](https://grafana.com/docs/grafana/latest/administration/service-accounts/#add-a-token-to-a-service-account-in-grafana) 
from the service account. This API token must be set in the plugin configuration as well 
and this can be done either by [provisioning](#configuring-the-plugin) or from the 
Grafana UI. Once the token has been generated and configured in the plugin, reports 
can be created using

```
$ curl --output=report.pdf -H "Authorization: Bearer <supersecrettoken>" "https://example.grafana.com/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>"
```

The above example shows on how to generate report using `curl` but this can be done with
any HTTP client of your favorite programming language.

> [!IMPORTANT] 
> If you are using Grafana >= 10.3.0, there is a feature flag called `externalServiceAccounts`
that can create a service account and provision a service account token automatically for
the plugin. Hence, there is no need to configure the service account token to the plugin. 
To enable this feature, it is necessary to set 
`enable = externalServiceAccounts` in `feature_toggles` section of Grafana configuration. 
However, the user will still need to create a service account and token to make the API
requests to generate report. 

### Security

When reports are generated from browser, there is minimal to no security risks as the 
plugin forward the current Grafana cookie in the request to make API requests to other
Grafana resources. This ensures that user will not be able to generate reports for 
the dashboards that contains data sources that they do not have permissions to query. The 
plugin _always_ prioritizes the cookie for authentication when found.

In the case, when the cookie is not found and a service account token is either configured 
with the plugin or `externalServiceAccounts` feature is enabled (for Grafana >= 10.3.0),
the plugin will use the service account token to make API requests to Grafana. In this 
case, the user _may_ generate reports of dashboards on the data sources that they do not
have permissions to. 

In order to avoid such a situation, disable the basic auth for Grafana. This will prevent
regular users from making API requests to generate reports and force them to always
use cookie for authentication.

## Examples

Here are the example reports that are generated out of the test dashboards

- [Report with portrait orientation, simple layout and full dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_portrait_simple_full.pdf)
- [Report with landscape orientation, simple layout and full dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_landscape_simple_full.pdf)
- [Report with portrait orientation, grid layout and full dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_portrait_grid_full.pdf)
- [Report with landscape orientation, grid layout and full dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_landscape_grid_full.pdf)
- [Report with portrait orientation, grid layout and default dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_portrait_grid_default.pdf)

## Troubleshooting

- When TLS is enabled on Grafana server, `grafana-image-renderer` tends to throw 
certificate errors even when the TLS certificates are signed by well-known CA. Typical
error messages will be as follows:

  ```
  logger=plugin.grafana-image-renderer t=2024-05-09T10:46:00.117454724+02:00 level=error msg="Browser request failed" url="https://localhost/d-solo/f5a26bea-adf2-4f2c-8522-79159ba26c0f/_?from=now-24h&height=500&panelId=6&theme=light&to=now&width=1000&render=1" method=GET failure=net::ERR_CERT_COMMON_NAME_INVALID
  logger=plugin.grafana-image-renderer t=2024-05-09T10:46:00.118784778+02:00 level=error msg="Error while trying to prepare page for screenshot" url="https://localhost:443/d-solo/f5a26bea-adf2-4f2c-8522-79159ba26c0f/_?from=now-24h&height=500&panelId=6&theme=light&to=now&width=1000&render=1" err="Error: net::ERR_CERT_COMMON_NAME_INVALID"
  ```

  To solve this issue set environment variables `GF_RENDERER_PLUGIN_IGNORE_HTTPS_ERRORS=true` 
  and `IGNORE_HTTPS_ERRORS=true` for the `grafana-image-renderer` service.

- If `chromium` fails to run, it suggests that there are missing dependent libraries on 
the host. In that case, we advise to install `chromium` on the machine which will 
install all the dependent libraries.

## Development

See [DEVELOPMENT.md](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/DEVELOPMENT.md)
