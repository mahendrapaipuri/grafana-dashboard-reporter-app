import * as React from "react";
import { getBackendSrv } from "@grafana/runtime";
import { testIds } from "../../components/testIds";
import { useAsync } from "react-use";
import { Badge, HorizontalGroup } from "@grafana/ui";

export const Status = () => {
  const { error, loading, value } = useAsync(() => {
    const backendSrv = getBackendSrv();

    return Promise.all([
      backendSrv.get(`api/plugins/dashboard-reporter-app/health`),
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
    <div data-testid={testIds.Status.container}>
      <HorizontalGroup>
        <h3>Plugin Health Check</h3>{" "}
        <span data-testid={testIds.Status.health}>
          {renderHealth(health?.message)}
        </span>
      </HorizontalGroup>
    </div>
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
