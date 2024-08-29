import * as React from "react";
import { getBackendSrv, PluginPage } from "@grafana/runtime";
import { PageLayoutType } from "@grafana/data";
import { testIds } from "../../components/testIds";
import { useAsync } from "react-use";
import { Badge, Stack, LinkButton } from "@grafana/ui";
// import { prefixRoute } from "../../utils/utils.routing";
import { ROUTES, NAVIGATION } from "../../constants";

export const Status = () => {
  const { error, loading, value } = useAsync(() => {
    const backendSrv = getBackendSrv();

    return Promise.all([
      backendSrv.get(
        `api/plugins/mahendrapaipuri-dashboardreporter-app/health`
      ),
    ]);
  });

  if (loading) {
    return (
      <div data-testid={testIds.Status.container}>
        <span>Loading...</span>
      </div>
    );
  }

  if (error || !value) {
    return (
      <div data-testid={testIds.Status.container}>
        <span>Error: {error?.message}</span>
      </div>
    );
  }

  const [health] = value;
  return (
    <PluginPage layout={PageLayoutType.Canvas}>
      <div data-testid={testIds.Status.container}>
        <Stack>
          <h3>Plugin Health Check</h3>{" "}
          <span data-testid={testIds.Status.health}>
            {renderHealth(health?.message)}
          </span>
        </Stack>
        <div style={{ marginBottom: "25px" }}>
          Only users with Admin role can modify the configuration of the plugin
        </div>
        <div data-testid={testIds.Status.config}>
          <LinkButton icon="cog" href={NAVIGATION[ROUTES.Config].url}>
            Configuration
          </LinkButton>
        </div>
      </div>
    </PluginPage>
  );
};

function renderHealth(message: string | undefined) {
  switch (message) {
    case "ok":
      return <Badge color="green" text="OK" icon="heart" />;

    default:
      return <Badge color="red" text="BAD" icon="bug" />;
  }
}
