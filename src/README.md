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
dependent libraries of the `chromium` are available on the host. Ensure to install
a more recent version of `chromium` as few issues were noticed with `chromium <= 90`.

## Installation

### Installation via `grafana-cli`

Grafana Enterprise offers a very similar plugin [reports](https://grafana.com/docs/grafana/latest/dashboards/create-reports/#export-dashboard-as-pdf)
and hence, their plugin policies do not allow to publish the current plugin in their
official catalog.

> [!NOTE]
> It is important to note that the current plugin does not offer all the functionalities
offered by Enterprise plugin and it is only relevant if users would like to create a
PDF report of a given dashboard. If users needs more advanced functionalities like
generating and sending reports automatically, they should look into official plugin.

However, it is still possible to install this plugin using `grafana-cli` by overriding
`pluginUrl` by using URL from [releases](https://github.com/asanluis/grafana-dashboard-reporter-app/releases).
For example following command will install plugin version `1.7.1`

```bash
VERSION=1.7.1; grafana-cli --pluginUrl "https://github.com/asanluis/grafana-dashboard-reporter-app/releases/download/v${VERSION}/mahendrapaipuri-dashboardreporter-app-${VERSION}.zip" plugins install mahendrapaipuri-dashboardreporter-app
```

Similarly, `nightly` version can be installed suing

```bash
grafana-cli --pluginUrl  https://github.com/asanluis/grafana-dashboard-reporter-app/releases/download/nightly/mahendrapaipuri-dashboardreporter-app-nightly.zip plugins install mahendrapaipuri-dashboardreporter-app
```

> [!TIP]
> If the above command is executed as `root`, the plugins folder might be owned by
`root` user and group which makes it inaccessible for `grafana`. If that is the case,
change ownership to the user running Grafana server which is usually `grafana`.

The plugin needs to run as unsigned plugin on on-premise Grafana installations and it
needs to be whitelisted.

> [!IMPORTANT]
> The final step is to _whitelist_ the plugin as it is an unsigned plugin and Grafana,
by default, does not load any unsigned plugins even if they are installed. In order to
whitelist the plugin, we need to add following to the Grafana configuration file

```ini
[plugins]
allow_loading_unsigned_plugins = mahendrapaipuri-dashboardreporter-app
```

Once this configuration is added, restart the Grafana server and it should load the
plugin. The loading of plugin can be verified by the following log lines

```bash
logger=plugin.signature.validator t=2024-03-21T11:16:54.738077851Z level=warn msg="Permitting unsigned plugin. This is not recommended" pluginID=mahendrapaipuri-dashboardreporter-app
logger=plugin.loader t=2024-03-21T11:16:54.738166325Z level=info msg="Plugin registered" pluginID=mahendrapaipuri-dashboardreporter-app

```

The plugin depends on following features flags and it is **strongly** recommended to enable
them on Grafana server.

- `accessControlOnCall`: Available in `Grafana >= 10.4.0`
- `idForwarding`: Available in `Grafana >= 10.4.0`
- `externalServiceAccounts`: Available in `Grafana >= 10.3.0`

This can be done using `feature_toggles` section of Grafana as follows:

```ini
[feature_toggles]
enable = accessControlOnCall,idForwarding,externalServiceAccounts
```

The plugin can work without enabling any of the above feature flags for `Grafana <= 10.4.3`.
However, for `Grafana > 10.4.4`, feature either `externalServiceAccounts` must be enabled for
the plugin to work or a manually created service account token must be configured with the plugin.

> [!IMPORTANT]
> From Grafana v11.3.0+, to use `externalServiceAccounts` feature, the following configuration
must be added to `auth` section of Grafana.

```ini
[auth]
managed_service_accounts_enabled = true
```

More details can be found in Grafana [docs](https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/#managed_service_accounts_enabled)

### Install with Docker-compose

There is a docker compose file provided in the repo. Create a directory `dist` in the
root of the repo and extract the latest version of the plugin app into this folder `dist`.
Once this is done, starting a Grafana server with plugin installed can be done
as follows:

```bash
docker-compose -f docker-compose.yaml up
```

### Chromium

As stated in the introduced, the plugin uses chromium to generate PDF reports of the dashboards.
If `grafana-image-renderer` plugin is installed on the same server as Grafana, the current plugin
will use the pre-built `chromium` shipped by `grafana-image-renderer` which _should_ work in most
of the cases. NOTE that in edge cases it might not work out-of-the-box with pre-built `chromium`
of `grafana-image-renderer`. In that case install `chromium` on the Grafana server from the
package manager.

> [!IMPORTANT]
> Use recent version of `chromium` to avoid any incompatibilities. We noticed issues with
`chromium <= 90`.

If `grafana-image-renderer` is not installed on the same server as Grafana or operators do not
want to install `chromium` on the server, it is possible to use a remote instance of `chromium`
for the plugin. In this case, the plugin needs to be provisioned appropriately with
configuration parameter that uses remote chromium. More details on how to configure it is in the
[next](#configuring-the-plugin) section.

> [!IMPORTANT]
> If the remote chromium is running on a different server ensure to encrypt the traffic between
Grafana server and remote chromium instance.

## Configuring the plugin

After successful installation of the plugin, it will be, by default, disabled. We can
enable it in different ways.

- From Grafana UI, navigating to `Apps > Dashboard Reporter App > Configuration` will
show [this page](https://github.com/asanluis/grafana-dashboard-reporter-app/blob/main/src/img/light.png)
and plugin can be enabled there. The configuration page can also be
accessed by URL `<Grafana URL>/plugins/mahendrapaipuri-dashboardreporter-app`.

> [!NOTE]
> The warning about `Invalid plugin signature` is not fatal and it is simply saying
that plugin has not been signed by Grafana Labs.

- By using [Grafana Provisioning](https://grafana.com/docs/grafana/latest/administration/provisioning/).
An example provision config is provided in the [repo](https://github.com/asanluis/grafana-dashboard-reporter-app/blob/main/provisioning/plugins/app.yaml)
and it can be installed at `/etc/grafana/provisioning/plugins/reporter.yml`. After installing
this YAML file, by restarting Grafana server, the plugin will be enabled with config
settings used in the `reporter.yml` file.

Grafana Provisioning is a programmatic way of configuring the plugin app. Some of the
configuration settings can be set from the environment variables too. Note that any
configured environment variable takes precedence over configuration file settings. Thus,
the plugin app can be configured at install time using either provisioning through YAML
file or using environment variables or mix of both. It is possible to modify these
settings at the runtime using Grafana UI.

To resume, the configuration settings can be set in the following ways:

- Using provisioning through a YAML file at install time
- Using environment variables set on Grafana server at install time
- Using Grafana UI at runtime

The configuration options set in the above stated methods are applied `Org` wide
in Grafana acting as baseline configuration for the plugin. Hence, these settings can
only be changed by a user with a `Admin` role using Grafana UI.

Different configuration settings are explained below. As each configuration option can
be set with different sources, the name of the option in each source is identified as
well. `file` stands for provisioning through YAML file, `env` stands for environment
variable and `ui` stands for name in Grafana UI. When a source is emitted, it means
that it is not possible to set that configuration option using that specific source.

### Authentication settings

This config section allows to configure authentication related settings. **This section
is only relevant when `externalServiceAccounts` feature flag is not enabled**.

- `file:saToken; ui:Service Account Token`: A service account token that will be used
   to generate reports _via_ API requests. More details on how to use it is briefed in
  [Using Grafana API](#using-grafana-api) section.

> [!IMPORTANT]
> When creating a service account, `Admin` role must be chosen as the plugin needs few
additional permissions. Once a service account with an `Admin` role has been created,
a new service token can be generated and configured with the plugin.

### Report settings

This config section allows to configure report related settings.

- `file:theme; env:GF_REPORTER_PLUGIN_REPORT_THEME; ui:Theme`: The report and the panels in
  the report will be generated using the chosen theme.

- `file:orientation; env:GF_REPORTER_PLUGIN_REPORT_ORIENTATION; ui:Orientation`: Orientation
  of the report. Available options: `portrait` and `landscape`.

- `file:layout; env:GF_REPORTER_PLUGIN_REPORT_LAYOUT; ui:Layout`: Layout of the report.
  Using grid layout renders the report as it is rendered in the browser. A simple
  layout will render the report with one panel per row. Available options: `simple`
  and `grid`. When using `grid` layout, we recommend to use `landscape` orientation
  for better readability.

- `file:dashboardMode; env:GF_REPORTER_PLUGIN_REPORT_DASHBOARD_MODE; ui:Dashboard Mode`:
  Whether to render default dashboard or full dashboard. In default mode, collapsed rows
  are ignored and only visible panels are included in the report. Whereas in full mode,
  rows are un collapsed and all the panels are included in the report. Available options:
  `default` and `full`.

- `file:timeZone; env:GF_REPORTER_PLUGIN_REPORT_TIMEZONE; ui:Time Zone`: The time zone
  that will be used in the report. It has to conform to the
  [IANA format](https://www.iana.org/time-zones). By default, local Grafana server's
  time zone will be used.

> [!NOTE]
> Starting from Grafana v11.3.0, the dashboard's configured time zone is exposed as a
query parameter in the dashboard URL and it will be used to set the time zone of the report.
Hence, for deployments with Grafana v11.3.0 or above, this parameter will not have effect. For
deployments with Grafana < v11.3.0, the time zone must be configured on
[grafana-image-renderer](https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/#rendering_timezone)
as well to render the panels in that given time zone.

- `file:timeFormat; env:GF_REPORTER_PLUGIN_REPORT_TIMEFORMAT; ui:Time Format`: The time format
  that will be used in the report. It has to conform to the
  [Golang time Layout](https://pkg.go.dev/time#Layout). By default,  format
  "Mon Jan _2 15:04:05 MST 2006" is used.

- `file:logo; env: GF_REPORTER_PLUGIN_REPORT_LOGO; ui:Branding Logo`: This parameter
  takes a base64 encoded image that will be included in the footer of each page in the
  report. Typically, operators can include their organization logos to have "customized"
  reports. Images of format PNG and JPG are accepted. **There is no need to add the base64 header**.
  Based on the content, Mime type will be detected and appropriate header will be added.

The following settings are advanced settings that allow to customize the header and footer
of the report using custom HTML templates.

- `file:headerTemplate; env:GF_REPORTER_PLUGIN_REPORT_HEADER_TEMPLATE; ui:Header Template`:
  HTML template that will be added as header to the report.

- `file:footerTemplate; env:GF_REPORTER_PLUGIN_REPORT_FOOTER_TEMPLATE; ui:Footer Template`:
  HTML template that will be added as footer to the report.

Templates must conform to [Go's template](https://pkg.go.dev/text/template) style
using `{{ }}` as delimiters. The following variables are available in the templates:

- `.Title`: Dashboard title
- `.VariableValues`: Comma separated list of dashboard variable values
- `.From`: Dashboard's `from` time
- `.To`: Dashboard's `to` time
- `.Date`: Current date time.

Default [header](https://github.com/asanluis/grafana-dashboard-reporter-app/blob/main/pkg/plugin/report/templates/header.gohtml) and [footer](https://github.com/asanluis/grafana-dashboard-reporter-app/blob/main/pkg/plugin/report/templates/footer.gohtml) templates can be used as a base to further
customize the reports using custom templates.

### Additional settings

The following configuration settings allow more control over plugin's functionality.

- `file:appUrl; env: GF_REPORTER_PLUGIN_APP_URL; ui: Grafana Hostname`: The URL at which
  Grafana is running. By default, `http://localhost:3000` is used which should work for
  most of the deployments.

- `file:skipTlsCheck; env: GF_REPORTER_PLUGIN_SKIP_TLS_CHECK; ui: Skip TLS Verification`:
  If Grafana instance is configured to use TLS with self signed certificates
  set this parameter to `true` to skip TLS certificate check.

- `file:remoteChromeUrl; env: GF_REPORTER_PLUGIN_REMOTE_CHROME_URL; ui: Remote Chrome URL`:
  A URL of a running remote chrome instance which will be used in report generation. Grafana
  running on k8s can opt to use this option when installing `chromium` inside Grafana
  container is not desired. An example [docker-compose file](https://github.com/asanluis/grafana-dashboard-reporter-app/blob/main/docker-compose.yaml) shows how to run `chromium` in an `init` container. When remote chrome instance is being used, ensure
  that `appUrl` is accessible to remote chrome.

- `file:maxBrowserWorkers; env: GF_REPORTER_PLUGIN_MAX_BROWSER_WORKERS; ui: Maximum Browser Workers`:
  Maximum number of workers for interacting with chrome browser.

- `file:maxRenderWorkers; env: GF_REPORTER_PLUGIN_MAX_RENDER_WORKERS; ui: Maximum Render Workers`:
  Maximum number of workers for generating panel PNGs.

> [!NOTE]
> Starting from `v1.4.0`, config parameter `dataPath` is not needed anymore as the plugin
will get the Grafana's data path based on its own executable path. If the existing provisioned
configs have this parameter set, it will be ignored while loading the plugin's configuration.

More advanced settings on HTTP client can be configured using provisioned config file which are
presented in [Advanced Settings](#advanced-settings).

#### Overriding global report settings

Although configuration settings can only be modified by users with `Admin` role for whole `Org`
of Grafana, it is possible to override the global defaults for a particular report
by using query parameters. It is enough to add query parameters to dashboard report URL
to set these values. Currently, the supported query parameters are:

- Query field for theme is `theme` and it takes either `light` or `dark` as value.
  Example is `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>&theme=dark`

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

- Query field for dashboard mode is `timeFormat` and it takes a value in [Golang time layout](https://pkg.go.dev/time#Layout)
  as value. **Note** that it should be encoded to escape URL specific characters. For example
  to use `Monday, 02-Jan-06 15:04:05 MST` query parameter should be
  `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>&timeFormat=Monday%2C+02-Jan-06+15%3A04%3A05+MST`

Besides there are **two** special query parameters available namely:

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
  it will be **excluded** in the report.

#### Rendering tabular data in the report

The plugin can fetch panel data and render it as tables at the end of the dashboard report. However,
by default no data is rendered and user can request the tabular data by using `includePanelDataID`
query parameter. For instance, an API request like `<grafanaAppUrl>/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>&includePanelDataID=1&includePanelDataID=5&includePanelDataID=8` will  include tabular data for
the panels `1`, `5` and `8` at the end of the report.

### Grafana API Token

The plugin needs to make API requests to Grafana to fetch resources like dashboard models,
panels, _etc._  Depending on the Grafana version the operators need to perform some
extra configuration to get an API token from Grafana.

- `Grafana <= 10.4.3`: Until Grafana 10.4.3, Grafana was forwarding the user cookies to
  plugin apps and the plugin will use the same user cookie to make API requests to Grafana.
  Thus, if `Grafana <= 10.4.3` is being used, there is no need to provide any API token
  to the the plugin.

- `Grafana > 10.4.3`: For these Grafana deployments, the plugin needs an API token from
  Grafana to make API requests to Grafana. This can be done automatically by enabling
  feature flag `externalServiceAccounts`, which will create a service account and
  provision a service account token automatically for the plugin. Please consult
  [Installation](#installation) on how to configure the feature flags on
  Grafana server.

> [!NOTE]
> If the operators do not wish or cannot use `externalServiceAccounts` feature flag on
their Grafana deployment, it is possible to manually create an API token and set it in
the [plugin configuration options](#authentication-settings).

### Multiple Orgs

Grafana does not support yet automatically provisioning the plugins with service tokens
using `externalServiceAccounts`. More details can be found in this [GH issue](https://github.com/grafana/grafana/issues/91844).
A workaround in this case is to turn off the feature flag `externalServiceAccounts` and
manually create service account token for each Org. and setting it in the plugin
configuration file. In this case, the provisioned config for the plugin will look like this:

```yaml
apps:
  - type: mahendrapaipuri-dashboardreporter-app
    org_id: 1
    org_name: Main Org.
    disabled: false
    secureJsonData:
      saToken: <ServiceAccountTokenForMainOrg>
    jsonData:
      appUrl: http://localhost:3000

  - type: mahendrapaipuri-dashboardreporter-app
    org_id: 2
    org_name: Test Org.
    disabled: false
    secureJsonData:
      saToken: <ServiceAccountTokenForTestOrg>
    jsonData:
      appUrl: http://localhost:3000
```

> [!IMPORTANT]
> It is compulsory to disable `externalServiceAccounts` feature flag in multiple Org. setting
as plugin wont work as expected with this feature flag.

## Using plugin

### Using Grafana web UI

The prerequisite is the user must have at least `Viewer` role on the dashboard that they
want to create a PDF report. After the user authenticates with Grafana, creating a
dashboard report is done by visiting the following end point

```bash
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
from the service account. If `externalServiceAccounts` feature flag is not enabled,
either the same or another API token must be added to the
[plugin configuration](#authentication-settings) as well. Once the token has been
generated and configured in the plugin, reports can be created using

```bash
curl --output=report.pdf -H "Authorization: Bearer <supersecrettoken>" "https://example.grafana.com/api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report?dashUid=<UID of dashboard>"
```

The above example shows on how to generate report using `curl` but this can be done with
any HTTP client of your favorite programming language.

## Security

All the feature flags listed in the [Installation](#installation) section
must be enabled on Grafana server for secure operation of your Grafana instance.
These feature flags enables the plugin to verify
the if the user who is making the request to generate the report has
enough permissions to view the dashboard before generating the report.
The plugin _always_ prioritizes the cookie for authentication when found.

<!-- ### `Grafana >= 10.4.4 and mahendrapaipuri-dashboardreporter-app <= 1.5.0`

If you are using `Grafana >= 10.4.4` along with plugin `<= 1.5.0`, depending on your
deployment, it is possible for a user who do not have `View` permissions on a dashboard
to generate a report on that dashboard. So, we **strongly** advise to update your plugin
to a version `> 1.5.0` ASAP. -->

## Examples

Here are the example reports that are generated out of the test dashboards

- [Report with portrait orientation, simple layout and full dashboard mode](https://github.com/asanluis/grafana-dashboard-reporter-app/blob/main/docs/reports/report_portrait_simple_full.pdf)
- [Report with portrait orientation, simple layout, full dashboard mode and tabular data](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_portrait_simple_full_table.pdf)
- [Report with landscape orientation, simple layout and full dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_landscape_simple_full.pdf)
- [Report with portrait orientation, grid layout and full dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_portrait_grid_full.pdf)
- [Report with landscape orientation, grid layout and full dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_landscape_grid_full.pdf)
- [Report with portrait orientation, grid layout and default dashboard mode](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/docs/reports/report_portrait_grid_default.pdf)

## Advanced Settings

The plugin makes API requests to Grafana to fetch PNGs of individual panels in a dashboard.
If a wider time window is selected in the dashboard during report generation, API requests
might need a bigger timeout for the panel data to load in its entirety. By default, a timeout
of 30 seconds is used in the HTTP client. If a bigger timeout is needed for a particular use
case, it can be done using provisioned config file. A basic provisioned config file with
to set timeout to 120 seconds can be defined as follows:

```yaml
apiVersion: 1

apps:
  - type: mahendrapaipuri-dashboardreporter-app
    org_id: 1
    org_name: Main Org.
    disabled: false
    
    jsonData:
      # HTTP Client timeout in seconds
      #
      timeout: 120
```

Along with timeout, there are several other HTTP client options can be set through
provisioned config file which are shown below along with their default values:

```yaml
apiVersion: 1

apps:
  - type: mahendrapaipuri-dashboardreporter-app
    org_id: 1
    org_name: Main Org.
    disabled: false
    
    jsonData:
      # HTTP Client timeout in seconds
      #
      timeout: 30

      # HTTP Client dial timeout in seconds
      #
      dialTimeout: 10

      # HTTP Keep Alive timeout in seconds
      #
      httpKeepAlive: 30

      # HTTP TLS handshake timeout in seconds
      #
      httpTLSHandshakeTimeout: 10

      # HTTP idle connection timeout in seconds
      #
      httpIdleConnTimeout: 90

      # HTTP max connections per host
      #
      httpMaxConnsPerHost: 0

      # HTTP max idle connections
      #
      httpMaxIdleConns: 100

      # HTTP max idle connections per host
      #
      httpMaxIdleConnsPerHost: 100
```

> [!NOTE]
> These settings can be configured only through provisioned config file and it is not
possible to set them using either environment variables or Grafana UI.

## Troubleshooting

- When TLS is enabled on Grafana server, `grafana-image-renderer` tends to throw
certificate errors even when the TLS certificates are signed by well-known CA. Typical
error messages will be as follows:

  ```bash
  logger=plugin.grafana-image-renderer t=2024-05-09T10:46:00.117454724+02:00 level=error msg="Browser request failed" url="https://localhost/d-solo/f5a26bea-adf2-4f2c-8522-79159ba26c0f/_?from=now-24h&height=500&panelId=6&theme=light&to=now&width=1000&render=1" method=GET failure=net::ERR_CERT_COMMON_NAME_INVALID
  logger=plugin.grafana-image-renderer t=2024-05-09T10:46:00.118784778+02:00 level=error msg="Error while trying to prepare page for screenshot" url="https://localhost:443/d-solo/f5a26bea-adf2-4f2c-8522-79159ba26c0f/_?from=now-24h&height=500&panelId=6&theme=light&to=now&width=1000&render=1" err="Error: net::ERR_CERT_COMMON_NAME_INVALID"
  ```

  To solve this issue set environment variables `GF_RENDERER_PLUGIN_IGNORE_HTTPS_ERRORS=true`
  and `IGNORE_HTTPS_ERRORS=true` for the `grafana-image-renderer` service.

- If `chromium` fails to run, it suggests that there are missing dependent libraries on
the host. In that case, we advise to install `chromium` on the machine which will
install all the dependent libraries.

- On Ubuntu, for a more hassle-free experience, install `google-chrome`
from [DEB package](https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb)
instead of installing `chromium` from `snap`.

- If Grafana server is running inside a systemd service file, sometimes users might
see errors as follows:

  ```bash
  couldn't create browser context: chrome failed to start:\nchrome_crashpad_handler: --database is required\nTry 'chrome_crashpad_handler --help' for more information.\n[147301:147301:0102/092026.518581:ERROR:socket.cc(120)] recvmsg: Connection reset by peer (104)\n"
  ```

  This is due to `google-chrome`/`chromium` not able to create user profile
  directories. A solution is to set environment variables
  `XDG_CONFIG_HOME=/tmp/.chrome` and `XDG_CACHE_HOME=/tmp/.chrome` on the Grafana
  process. If users do not wish to use `/tmp`, any folder where Grafana process
  has write permissions can be used.

- If you get `permission denied` response when generating a report, it is due to the
  user not having `View` permissions on the dashboard that they are attempting to generate
  the report.

## Development

See [DEVELOPMENT.md](https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/blob/main/DEVELOPMENT.md)
