# Grafana Dashboard Reporter App

This Grafana plugin app can create PDF reports of a given dashboard using headless `chromium` 
and [`grafana-image-renderer`](https://github.com/grafana/grafana-image-renderer). 

This plugin is based on the original work 
[grafana-reporter](https://github.com/IzakMarais/reporter). 
The core of the plugin is heavily inspired from the above stated work with some 
improvements and modernization. 

- The current plugin uses HTML templates and headless chromium to generate reports 
  instead of LaTeX. `grafana-image-renderer` is a prerequisite for both current and 
  original plugins.

- The current plugin app exposes the reporter as a custom API end point without 
  needing to run the [grafana-reporter](https://github.com/IzakMarais/reporter) 
  as a separate web service. The advantage of the plugin approach is the authenticated 
  access to the reporter app is guaranteed by Grafana auth.

- The plugin can be configured by Admins and users either from 
  [Configuration Page](./src/img/light.png) or query parameters to the report API.

## Documentation

More documentation can be found in [README](./src/README.md)
