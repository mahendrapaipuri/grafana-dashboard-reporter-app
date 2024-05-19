import React, { useState, ChangeEvent } from 'react';
import { lastValueFrom } from 'rxjs';
import { css } from '@emotion/css';
import {
  Button,
  useStyles2,
  Field,
  Input,
  FieldSet,
  Switch,
  RadioButtonGroup,
  SecretInput,
} from "@grafana/ui";
import {
  PluginConfigPageProps,
  AppPluginMeta,
  PluginMeta,
  GrafanaTheme2,
} from "@grafana/data";
import { getBackendSrv } from "@grafana/runtime";
import { testIds } from "../testIds";

export type JsonData = {
  appUrl?: string;
  skipTlsCheck?: boolean;
  orientation?: string;
  layout?: string;
  dashboardMode?: string;
  timeZone?: string;
  logo?: string;
  maxRenderWorkers?: number;
  persistData?: boolean;
};

type State = {
  // PDF report orientation (portrait or landscape)
  orientation: string;
  // If orientation has changed
  orientationChanged: boolean;
  // Layout in report (grid or simple)
  layout: string;
  // If layout has changed
  layoutChanged: boolean;
  // dashboardMode (default or full)
  dashboardMode: string;
  // If dashboardMode has changed
  dashboardModeChanged: boolean;
  // time zone in IANA format
  timeZone: string;
  // If timeZone has changed
  timeZoneChanged: boolean;
  // base64 encoded logo
  logo: string;
  // If logo has changed
  logoChanged: boolean;
  // Maximum rendering workers
  maxRenderWorkers: number;
  // If maxRenderWorkers has changed
  maxRenderWorkersChanged: boolean;
  // Whether to persist templated files for debugging
  persistData: boolean;
  // If persistData has changed
  persistDataChanged: boolean;
  // Tells us if the Service Account's token is set.
  // Set to `true` ONLY if it has already been set and haven't been changed.
  // (We unfortunately need an auxiliray variable for this, as `secureJsonData` is never exposed to the browser after it is set)
  isSaTokenSet: boolean;
  // A Service account's token used to make requests to Grafana API.
  saToken: string;
};

interface Props extends PluginConfigPageProps<AppPluginMeta<JsonData>> {}

export const AppConfig = ({ plugin }: Props) => {
  const s = useStyles2(getStyles);
  const { enabled, pinned, jsonData, secureJsonFields } = plugin.meta;
  const [state, setState] = useState<State>({
    orientation: jsonData?.orientation || "portrait",
    orientationChanged: false,
    layout: jsonData?.layout || "simple",
    layoutChanged: false,
    dashboardMode: jsonData?.dashboardMode || "default",
    dashboardModeChanged: false,
    timeZone: jsonData?.timeZone || "",
    timeZoneChanged: false,
    logo: jsonData?.logo || "",
    logoChanged: false,
    maxRenderWorkers: jsonData?.maxRenderWorkers || 2,
    maxRenderWorkersChanged: false,
    persistData: jsonData?.persistData || false,
    persistDataChanged: false,
    saToken: "",
    isSaTokenSet: Boolean(secureJsonFields?.saToken),
  });

  // Pass through appUrl and SkipTlsCheck as they cannot be configured via UI
  const appUrl = jsonData?.appUrl;
  const skipTlsCheck = jsonData?.skipTlsCheck;

  const orientationOptions = [
    { label: "Portrait", value: "portrait", icon: "gf-portrait" },
    { label: "Landscape", value: "landscape", icon: "gf-landscape" },
  ];

  const layoutOptions = [
    { label: "Simple", value: "simple", icon: "gf-layout-simple" },
    { label: "Grid", value: "grid", icon: "gf-grid" },
  ];

  const dashboardModeOptions = [
    { label: "Default", value: "default" },
    { label: "Full", value: "full" },
  ];

  const onChangeLayout = (value: string) => {
    setState({
      ...state,
      layout: value,
      layoutChanged: true,
    });
  };

  const onChangeOrientation = (value: string) => {
    setState({
      ...state,
      orientation: value,
      orientationChanged: true,
    });
  };

  const onChangeDashboardMode = (value: string) => {
    setState({
      ...state,
      dashboardMode: value,
      dashboardModeChanged: true,
    });
  };

  const onChangetimeZone = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      timeZone: event.target.value,
      timeZoneChanged: true,
    });
  };


  const onChangeLogo = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      logo: event.target.value,
      logoChanged: true,
    });
  };

  const onChangeMaxWorkers = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      maxRenderWorkers: event.target.valueAsNumber,
      maxRenderWorkersChanged: true,
    });
  };

  const onChangePersistData = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      persistData: event.target.checked,
      persistDataChanged: true,
    });
  };

  const onResetSaToken = () =>
    setState({
      ...state,
      saToken: "",
      isSaTokenSet: false,
    });

  const onChangeSaToken = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      saToken: event.target.value.trim(),
    });
  };

  return (
    <div data-testid={testIds.appConfig.container}>
      {/* ENABLE / DISABLE PLUGIN */}
      <FieldSet label="Enable / Disable">
        {!enabled && (
          <>
            <div className={s.colorWeak}>
              The plugin is currently not enabled.
            </div>
            <Button
              className={s.marginTop}
              variant="primary"
              onClick={() =>
                updatePluginAndReload(plugin.meta.id, {
                  enabled: true,
                  pinned: true,
                  jsonData: {
                    appUrl: appUrl,
                    skipTlsCheck: skipTlsCheck,
                    maxRenderWorkers: state.maxRenderWorkers,
                    orientation: state.orientation,
                    layout: state.layout,
                    dashboardMode: state.dashboardMode,
                    timeZone: state.timeZone,
                    logo: state.logo,
                    persistData: state.persistData,
                  },
                  // This cannot be queried later by the frontend.
                  // We don't want to override it in case it was set previously and left untouched now.
                  secureJsonData: state.isSaTokenSet
                    ? undefined
                    : {
                        saToken: state.saToken,
                      },
                })
              }
            >
              Enable plugin
            </Button>
          </>
        )}

        {/* Disable the plugin */}
        {enabled && (
          <>
            <div className={s.colorWeak}>The plugin is currently enabled.</div>
            <Button
              className={s.marginTop}
              variant="destructive"
              onClick={() =>
                updatePluginAndReload(plugin.meta.id, {
                  enabled: false,
                  pinned: false,
                  jsonData: {
                    appUrl: appUrl,
                    skipTlsCheck: skipTlsCheck,
                    maxRenderWorkers: state.maxRenderWorkers,
                    orientation: state.orientation,
                    layout: state.layout,
                    dashboardMode: state.dashboardMode,
                    timeZone: state.timeZone,
                    logo: state.logo,
                    persistData: state.persistData,
                  },
                  // This cannot be queried later by the frontend.
                  // We don't want to override it in case it was set previously and left untouched now.
                  secureJsonData: state.isSaTokenSet
                    ? undefined
                    : {
                        saToken: state.saToken,
                      },
                })
              }
            >
              Disable plugin
            </Button>
          </>
        )}
      </FieldSet>

      {/* CUSTOM SETTINGS */}
      <FieldSet label="Plugin Settings" className={s.marginTopXl}>
        {/* Service account token */}
        <Field
          label="Service Account Token"
          description="This token will be used to make API requests to Grafana for generating reports."
          data-testid={testIds.appConfig.saToken}
        >
          <SecretInput
            width={60}
            id="sa-token"
            value={state.saToken}
            isConfigured={state.isSaTokenSet}
            placeholder={
              state.isSaTokenSet
                ? "configured"
                : "Your service account token here"
            }
            onChange={onChangeSaToken}
            onReset={onResetSaToken}
          />
        </Field>

        {/* Use Grid Layout */}
        <Field
          label="Layout"
          description="Display the panels in their positions on the dashboard."
          data-testid={testIds.appConfig.layout}
          className={s.marginTop}
        >
          <RadioButtonGroup
            options={layoutOptions}
            value={state.layout}
            onChange={onChangeLayout}
          />
        </Field>

        {/* Report Orientation */}
        <Field
          label="Report Orientation"
          description="Orientation of the report."
          data-testid={testIds.appConfig.orientation}
          className={s.marginTop}
        >
          <RadioButtonGroup
            options={orientationOptions}
            value={state.orientation}
            onChange={onChangeOrientation}
          />
        </Field>

        {/* Dashboard Mode */}
        <Field
          label="Dashboard Mode"
          description="Whether to render full dashboard by uncollapsing panels in all rows or to render default dashboard without panels in collapsed rows."
          data-testid={testIds.appConfig.dashboardMode}
          className={s.marginTop}
        >
          <RadioButtonGroup
            options={dashboardModeOptions}
            value={state.dashboardMode}
            onChange={onChangeDashboardMode}
          />
        </Field>

        {/* Time zone */}
        <Field
          label="Time Zone"
          description="Time Zone in IANA format. By default time zone of the server will be used."
          data-testid={testIds.appConfig.tz}
          className={s.marginTop}
        >
          <Input
            type="string"
            width={60}
            id="tz"
            label={`Time Zone`}
            value={state.timeZone}
            onChange={onChangetimeZone}
          />
        </Field>

        {/* Branding logo */}
        <Field
          label="Branding Logo"
          description="Base 64 encoded logo to include in the report."
          data-testid={testIds.appConfig.logo}
          className={s.marginTop}
        >
          <Input
            type="string"
            width={60}
            id="logo"
            label={`Logo`}
            value={state.logo}
            onChange={onChangeLogo}
          />
        </Field>

        {/* Persist data */}
        <Field
          label="Persist Data Files"
          description="Persist templated data files for debugging. Files will be kept at $GF_DATA_PATH/reports/debug folder on server."
          data-testid={testIds.appConfig.persistData}
          className={s.marginTop}
        >
          <Switch
            id="persit-data"
            value={state.persistData}
            onChange={onChangePersistData}
          />
        </Field>

        {/* Max workers */}
        <Field
          label="Maximum Render Workers"
          description="Maximum number of workers for rendering panels into PNGs. Default is 2."
          className={s.marginTop}
        >
          <Input
            type="number"
            width={60}
            id="max-workers"
            data-testid={testIds.appConfig.maxWorkers}
            label={`Maximum Render Workers`}
            value={state.maxRenderWorkers}
            onChange={onChangeMaxWorkers}
          />
        </Field>

        <div className={s.marginTop}>
          <Button
            type="submit"
            data-testid={testIds.appConfig.submit}
            onClick={() =>
              updatePluginAndReload(plugin.meta.id, {
                enabled,
                pinned,
                jsonData: {
                  appUrl: appUrl,
                  skipTlsCheck: skipTlsCheck,
                  maxRenderWorkers: state.maxRenderWorkers,
                  orientation: state.orientation,
                  layout: state.layout,
                  dashboardMode: state.dashboardMode,
                  timeZone: state.timeZone,
                  logo: state.logo,
                  persistData: state.persistData,
                },
                // This cannot be queried later by the frontend.
                // We don't want to override it in case it was set previously and left untouched now.
                secureJsonData: state.isSaTokenSet
                  ? undefined
                  : {
                      saToken: state.saToken,
                    },
              })
            }
            disabled={Boolean(
              !state.layoutChanged &&
                !state.orientationChanged &&
                !state.dashboardModeChanged &&
                !state.timeZoneChanged &&
                !state.logoChanged &&
                !state.maxRenderWorkersChanged &&
                !state.persistDataChanged &&
                !state.saToken
            )}
          >
            Save settings
          </Button>
        </div>
      </FieldSet>
    </div>
  );
};

const getStyles = (theme: GrafanaTheme2) => ({
  colorWeak: css`
    color: ${theme.colors.text.secondary};
  `,
  marginTop: css`
    margin-top: ${theme.spacing(3)};
  `,
  marginTopXl: css`
    margin-top: ${theme.spacing(6)};
  `,
});

const updatePluginAndReload = async (pluginId: string, data: Partial<PluginMeta<JsonData>>) => {
  try {
    await updatePlugin(pluginId, data);

    // Reloading the page as the changes made here wouldn't be propagated to the actual plugin otherwise.
    // This is not ideal, however unfortunately currently there is no supported way for updating the plugin state.
    window.location.reload();
  } catch (e) {
    console.error('Error while updating the plugin', e);
  }
};

export const updatePlugin = async (pluginId: string, data: Partial<PluginMeta>) => {
  const response = await getBackendSrv().fetch({
    url: `/api/plugins/${pluginId}/settings`,
    method: 'POST',
    data,
  });

  return lastValueFrom(response);
};
