# Grafana Dashboard Reporter App

This Grafana plugin app can create PDF reports of a given dashboard using `pdflatex` 
and [`grafana-image-renderer`](https://github.com/grafana/grafana-image-renderer). 

This plugin is based on the original work 
[grafana-reporter](https://github.com/IzakMarais/reporter). 
The core of the plugin app uses the same code base as the above stated work with some 
improvements and modernization. The current plugin app exposes the reporter as a 
custom API end point without needing to run the 
[grafana-reporter](https://github.com/IzakMarais/reporter) 
as a separate web service. The advantage of the plugin approach is the authenticated access 
to the reporter app is guaranteed by Grafana auth.

## Documentation

More documentation can be found in [README](./src/README.md)
