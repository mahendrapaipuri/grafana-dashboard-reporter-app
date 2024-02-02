#!/bin/bash

# Set repository information
REPO_OWNER="mahendrapaipuri"
REPO_NAME="grafana-dashboard-reporter-app"
API_URL="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases"

# Fetch all release information
ALL_RELEASES=$(curl -s $API_URL)

# Check if a valid release list is found
if [[ $ALL_RELEASES == *"Not Found"* ]]; then
    echo "Releases not found for the $REPO_NAME repository."
    exit 1
fi

# Extract the latest pre-release tag from the release information
if [[ -z "$NIGHTLY" ]]; then
    LATEST_RELEASE_TAG=$(echo "$ALL_RELEASES" | grep -Eo '"tag_name": "v[^"]*' | sed -E 's/"tag_name": "//' | head -n 1)
    echo "The latest release tag of $REPO_NAME is: $LATEST_RELEASE_TAG"
else 
    echo "Using latest nightly release"
    LATEST_RELEASE_TAG="nightly"
fi

curl -L https://github.com/mahendrapaipuri/grafana-dashboard-reporter-app/releases/download/$LATEST_RELEASE_TAG/mahendrapaipuri-dashboardreporter-app.zip --output mahendrapaipuri-dashboardreporter-app.zip
unzip mahendrapaipuri-dashboardreporter-app.zip -d . 
