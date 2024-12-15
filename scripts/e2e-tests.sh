#!/bin/bash

# Constants
API_PATH="api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report"
DASH_UID="fdlwjnyim1la8f"

# CLI ARGS
VARIANT="$1"

if [[ "$VARIANT" == "plain" ]]; then
    GRAFANA_PROTOCOL="http"
    GRAFANA_PORT="3080"
    QUERY_PARAMS="layout=simple&orientation=portrait&dashboardMode=default&var-testvar0=All&var-testvar1=foo&var-testvar2=1"
    REPORT_NAME="default"
else
    GRAFANA_PROTOCOL="https"
    GRAFANA_PORT="3443"
    QUERY_PARAMS="layout=grid&orientation=landscape&dashboardMode=full&from=now-5m&to=now&var-testvar0=All&var-testvar1=foo&var-testvar2=1"
    REPORT_NAME="alternative"
fi

# Tests
echo "Making basic report generation request by admin"
RESP_CODE=$(curl -k -o "${REPORT_NAME}.pdf" -w "%{http_code}" "$GRAFANA_PROTOCOL://admin:admin@localhost:$GRAFANA_PORT/$API_PATH?dashUid=$DASH_UID&$QUERY_PARAMS")

# Check response
if [[ "$RESP_CODE" != "200" ]]; then
    echo "Expected 200 got $RESP_CODE"
    exit 1
fi
