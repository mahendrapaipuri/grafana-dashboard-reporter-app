import { NavModelItem } from "@grafana/data";
import pluginJson from "./plugin.json";

export const PLUGIN_ID = `${pluginJson.id}`;
export const PLUGIN_BASE_URL = `/a/${PLUGIN_ID}`;

export enum ROUTES {
  Status = "status",
  Config = "config",
}

export const NAVIGATION_TITLE = "Grafana Reporting App";
export const NAVIGATION_SUBTITLE =
  "A Grafana plugin app that generates PDF reports from Grafana dashboards";

// Add a navigation item for each route you would like to display in the navigation bar
export const NAVIGATION: Record<string, NavModelItem> = {
  [ROUTES.Status]: {
    id: ROUTES.Status,
    text: "Status",
    icon: "heart",
    url: `${PLUGIN_BASE_URL}/status`,
  },
  [ROUTES.Config]: {
    id: ROUTES.Config,
    text: "Configuration",
    icon: "cog",
    url: `plugins/${PLUGIN_ID}`,
  },
};
