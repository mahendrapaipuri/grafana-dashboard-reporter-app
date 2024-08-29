import { test as setup } from "@grafana/plugin-e2e";

// Auth header
const btoa = (str: string) => Buffer.from(str).toString("base64");

setup(
  "authenticate",
  async ({ request, login, createUser, user, grafanaAPICredentials }) => {
    if (
      user &&
      (user.user !== grafanaAPICredentials.user ||
        user.password !== grafanaAPICredentials.password)
    ) {
      await createUser();

      // We remove the Viewer role on the dashboard here so that created user
      // should not be able to generate report
      const changePermissionReq = await request.post(
        `/api/dashboards/uid/fdlwjnyim1la8f/permissions`,
        {
          data: {
            items: [{ role: "Editor", permission: 2 }],
          },
          headers: {
            Authorization:
              "Basic " +
              btoa(
                `${grafanaAPICredentials.user}:${grafanaAPICredentials.password}`
              ),
          },
        }
      );

      if (!changePermissionReq.ok()) {
        throw new Error(
          `Could not change permissions on dashboard: status ${changePermissionReq.status()}`
        );
      }
    }
    await login();
  }
);
