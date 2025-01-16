#!/bin/bash

# Constants
API_PATH="api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report"
DASH_UID="b3228ada-fd89-4aed-8605-d5f7b95aa237"

# CLI ARGS
VARIANT="$1"

# Panels to render
PANELS="includePanelID=1&includePanelID=5"

if [[ "$VARIANT" == "plain" ]]; then
    GRAFANA_PROTOCOL="http"
    GRAFANA_PORT="3080"
    QUERY_PARAMS="from=now-30m&to=now&var-job=$__all&var-instance=$__all&var-interval=1h&var-ds=PBFA97CFB590B2093&layout=simple&orientation=portrait&dashboardMode=default&$PANELS"
    REPORT_NAME="default"
else
    GRAFANA_PROTOCOL="https"
    GRAFANA_PORT="3443"
    QUERY_PARAMS="from=now-30m&to=now&var-job=$__all&var-instance=$__all&var-interval=1h&var-ds=PBFA97CFB590B2093&layout=grid&orientation=landscape&dashboardMode=full&$PANELS"
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
