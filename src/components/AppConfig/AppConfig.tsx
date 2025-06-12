import React, { useState, ChangeEvent } from 'react';
import { lastValueFrom } from 'rxjs';
import { css } from '@emotion/css';
import { ConfigSection, ConfigSubSection } from "@grafana/experimental";
import {
  Button,
  useStyles2,
  Field,
  Input,
  TextArea,
  Switch,
  FieldSet,
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
  appVersion?: string;
  skipTlsCheck?: boolean;
  theme?: string;
  orientation?: string;
  layout?: string;
  dashboardMode?: string;
  timeZone?: string;
  timeFormat?: string;
  logo?: string;
  headerTemplate?: string;
  footerTemplate?: string;
  maxBrowserWorkers?: number;
  maxRenderWorkers?: number;
  remoteChromeUrl?: string;
  customHttpHeaders?: Record<string, string>;
  timeout?: number;
  dialTimeout?: number;
  httpKeepAlive?: number;
  httpTLSHandshakeTimeout?: number;
  httpIdleConnTimeout?: number;
  httpMaxConnsPerHost?: number;
  httpMaxIdleConns?: number;
  httpMaxIdleConnsPerHost?: number;
};

type State = {
  // URL of grafana (override auto-detection)
  appUrl: string;
  // If appUrl has changed
  appUrlChanged: boolean;
  // Skip TLS verification to grafana
  skipTlsCheck: boolean;
  // Grafana version
  appVersion: string;
  // If skipTlsCheck has changed
  skipTlsCheckChanged: boolean;
  // Theme of panels (light or dark)
  theme: string;
  // If theme has changed
  themeChanged: boolean;
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
  // time format in Golang layout
  timeFormat: string;
  // If timeFormat has changed
  timeFormatChanged: boolean;
  // base64 encoded logo
  logo: string;
  // If logo has changed
  logoChanged: boolean;
  // HTML header template
  headerTemplate: string;
  // If header template has changed
  headerTemplateChanged: boolean;
  // HTML footer template
  footerTemplate: string;
  // If footer template has changed
  footerTemplateChanged: boolean;
  // Maximum browser workers
  maxBrowserWorkers: number;
  // If maxRenderWorkers has changed
  maxBrowserWorkersChanged: boolean;
  // Maximum rendering workers
  maxRenderWorkers: number;
  // If maxRenderWorkers has changed
  maxRenderWorkersChanged: boolean;
  // Address of an chrome remote instance
  remoteChromeUrl: string;
  // If remoteChromeUrl has changed
  remoteChromeUrlChanged: boolean;
  // Custom HTTP headers for render plugin
  customHttpHeaders: string;
  // If customHttpHeaders has changed
  customHttpHeadersChanged: boolean;
  // Tells us if the Service Account's token is set.
  // Set to `true` ONLY if it has already been set and haven't been changed.
  // (We unfortunately need an auxiliray variable for this, as `secureJsonData` is never exposed to the browser after it is set)
  isSaTokenSet: boolean;
  // A Service account's token used to make requests to Grafana API.
  saToken: string;
  // If the service has been reset
  isSaTokenReset: boolean;
  // Timemouts
  timeout: number;
  dialTimeout: number;
  httpKeepAlive: number;
  httpTLSHandshakeTimeout: number;
  httpIdleConnTimeout: number;
  httpMaxConnsPerHost: number;
  httpMaxIdleConns: number;
  httpMaxIdleConnsPerHost: number;
};

interface Props extends PluginConfigPageProps<AppPluginMeta<JsonData>> {}

export const AppConfig = ({ plugin }: Props) => {
  const s = useStyles2(getStyles);
  const { enabled, pinned, jsonData, secureJsonFields } = plugin.meta;
  const [state, setState] = useState<State>({
    appUrl: jsonData?.appUrl || "",
    appUrlChanged: false,
    appVersion: window?.grafanaBootData?.settings?.buildInfo?.version || "0.0.0", 
    skipTlsCheck: jsonData?.skipTlsCheck || false,
    skipTlsCheckChanged: false,
    theme: jsonData?.theme || "light",
    themeChanged: false,
    orientation: jsonData?.orientation || "portrait",
    orientationChanged: false,
    layout: jsonData?.layout || "simple",
    layoutChanged: false,
    dashboardMode: jsonData?.dashboardMode || "default",
    dashboardModeChanged: false,
    timeZone: jsonData?.timeZone || "",
    timeZoneChanged: false,
    timeFormat: jsonData?.timeFormat || "",
    timeFormatChanged: false,
    logo: jsonData?.logo || "",
    logoChanged: false,
    headerTemplate: jsonData?.headerTemplate || "",
    headerTemplateChanged: false,
    footerTemplate: jsonData?.footerTemplate || "",
    footerTemplateChanged: false,
    maxBrowserWorkers: jsonData?.maxBrowserWorkers || 2,
    maxBrowserWorkersChanged: false,
    maxRenderWorkers: jsonData?.maxRenderWorkers || 2,
    maxRenderWorkersChanged: false,
    remoteChromeUrl: jsonData?.remoteChromeUrl || "",
    remoteChromeUrlChanged: false,
    customHttpHeaders: jsonData?.customHttpHeaders ? JSON.stringify(jsonData.customHttpHeaders, null, 2) : "",
    customHttpHeadersChanged: false,
    saToken: "",
    isSaTokenSet: Boolean(secureJsonFields?.saToken),
    isSaTokenReset: false,
    timeout: jsonData?.timeout || 30,
    dialTimeout: jsonData?.dialTimeout || 10,
    httpKeepAlive: jsonData?.httpKeepAlive || 30,
    httpTLSHandshakeTimeout: jsonData?.httpTLSHandshakeTimeout || 10,
    httpIdleConnTimeout: jsonData?.httpIdleConnTimeout || 90,
    httpMaxConnsPerHost: jsonData?.httpMaxConnsPerHost || 0,
    httpMaxIdleConns: jsonData?.httpMaxIdleConns || 100,
    httpMaxIdleConnsPerHost: jsonData?.httpMaxIdleConnsPerHost || 100,
  });

  const themeOptions = [
    { label: "Light", value: "light" },
    { label: "Dark", value: "dark" },
  ];

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

  const onChangeURL = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      appUrl: event.target.value,
      appUrlChanged: true,
    });
  };

  const onChangeSkipTlsCheck = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      skipTlsCheck: event.target.checked,
      skipTlsCheckChanged: true,
    });
  };

  const onChangeTheme = (value: string) => {
    setState({
      ...state,
      theme: value,
      themeChanged: true,
    });
  };

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

  const onChangetimeFormat = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      timeFormat: event.target.value,
      timeFormatChanged: true,
    });
  };

  const onChangeLogo = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      logo: event.target.value,
      logoChanged: true,
    });
  };

  const onChangeHeaderTemplate = (event: ChangeEvent<HTMLTextAreaElement>) => {
    setState({
      ...state,
      headerTemplate: event.target.value,
      headerTemplateChanged: true,
    });
  };

  const onChangeFooterTemplate = (event: ChangeEvent<HTMLTextAreaElement>) => {
    setState({
      ...state,
      footerTemplate: event.target.value,
      footerTemplateChanged: true,
    });
  };

  const onChangeMaxBrowserWorkers = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      maxBrowserWorkers: event.target.valueAsNumber,
      maxBrowserWorkersChanged: true,
    });
  };

  const onChangeMaxRenderWorkers = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      maxRenderWorkers: event.target.valueAsNumber,
      maxRenderWorkersChanged: true,
    });
  };

  const onChangeRemoteChromeURL = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      remoteChromeUrl: event.target.value,
      remoteChromeUrlChanged: true,
    });
  };

  const onChangeCustomHttpHeaders = (event: ChangeEvent<HTMLTextAreaElement>) => {
    setState({
      ...state,
      customHttpHeaders: event.target.value,
      customHttpHeadersChanged: true,
    });
  };

  const onResetSaToken = () =>
    setState({
      ...state,
      saToken: "",
      isSaTokenSet: false,
      isSaTokenReset: true,
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
                    appUrl: state.appUrl,
                    appVersion: state.appVersion,
                    skipTlsCheck: state.skipTlsCheck,
                    theme: state.theme,
                    orientation: state.orientation,
                    layout: state.layout,
                    dashboardMode: state.dashboardMode,
                    timeZone: state.timeZone,
                    timeFormat: state.timeFormat,
                    logo: state.logo,
                    headerTemplate: state.headerTemplate,
                    footerTemplate: state.footerTemplate,
                    maxBrowserWorkers: state.maxBrowserWorkers,
                    maxRenderWorkers: state.maxRenderWorkers,
                    remoteChromeUrl: state.remoteChromeUrl,
                    customHttpHeaders: state.customHttpHeaders ? JSON.parse(state.customHttpHeaders) : {},
                    timeout: state.timeout,
                    dialTimeout: state.dialTimeout,
                    httpKeepAlive: state.httpKeepAlive,
                    httpTLSHandshakeTimeout: state.httpTLSHandshakeTimeout,
                    httpIdleConnTimeout: state.httpIdleConnTimeout,
                    httpMaxConnsPerHost: state.httpMaxConnsPerHost,
                    httpMaxIdleConns: state.httpMaxIdleConns,
                    httpMaxIdleConnsPerHost: state.httpMaxIdleConnsPerHost,
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
                    appUrl: state.appUrl,
                    appVersion: state.appVersion,
                    skipTlsCheck: state.skipTlsCheck,
                    theme: state.theme,
                    orientation: state.orientation,
                    layout: state.layout,
                    dashboardMode: state.dashboardMode,
                    timeZone: state.timeZone,
                    timeFormat: state.timeFormat,
                    logo: state.logo,
                    headerTemplate: state.headerTemplate,
                    footerTemplate: state.footerTemplate,
                    maxBrowserWorkers: state.maxBrowserWorkers,
                    maxRenderWorkers: state.maxRenderWorkers,
                    remoteChromeUrl: state.remoteChromeUrl,
                    customHttpHeaders: state.customHttpHeaders ? JSON.parse(state.customHttpHeaders) : {},
                    timeout: state.timeout,
                    dialTimeout: state.dialTimeout,
                    httpKeepAlive: state.httpKeepAlive,
                    httpTLSHandshakeTimeout: state.httpTLSHandshakeTimeout,
                    httpIdleConnTimeout: state.httpIdleConnTimeout,
                    httpMaxConnsPerHost: state.httpMaxConnsPerHost,
                    httpMaxIdleConns: state.httpMaxIdleConns,
                    httpMaxIdleConnsPerHost: state.httpMaxIdleConnsPerHost,
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

      {/* Authentication Settings */}
      <hr className={`${s.hrTopSpace} ${s.hrBottomSpace}`} />
      <ConfigSection
        title="Authentication"
        description="Use this section to configure service account tokens when Grafana < 10.3.0 is used or externalServiceAccounts feature is not enabled"
      >
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
      </ConfigSection>

      {/* Report Settings */}
      <hr className={`${s.hrTopSpace} ${s.hrBottomSpace}`} />
      <ConfigSection
        title="Report Settings"
        description="Use this section to customise the generated report"
      >
        {/* Report Panels Theme  */}
        <Field
          label="Theme"
          description="Report theme."
          data-testid={testIds.appConfig.theme}
          className={s.marginTop}
        >
          <RadioButtonGroup
            options={themeOptions}
            value={state.theme}
            onChange={onChangeTheme}
          />
        </Field>

        {/* Report Layout */}
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
          description="Time Zone in IANA format. By default time zone of the server will be used. Only relevant for Grafana < 11.3.0."
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

        {/* Time format */}
        <Field
          label="Time Format"
          description="Time Format as Golang time Layout (https://pkg.go.dev/time#Layout)."
          data-testid={testIds.appConfig.tf}
          className={s.marginTop}
        >
          <Input
            type="string"
            width={60}
            id="tz"
            label={`Time Format`}
            value={state.timeFormat}
            onChange={onChangetimeFormat}
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

        <ConfigSubSection
          title="Advanced Settings"
          description="Use this section to customize the reports"
          isCollapsible
          isInitiallyOpen={false}
        >
          {/* Header Template */}
          <Field
            label="Report Header Template"
            description="HTML template used in the header of the report."
            data-testid={testIds.appConfig.headerTemplate}
          >
            <TextArea
              id="headerTemplate"
              rows={20}
              className={s.textarea}
              label={`Header Template`}
              onChange={onChangeHeaderTemplate}
            >{state.headerTemplate}
            </TextArea>
          </Field>

          {/* Footer Template */}
          <Field
            label="Report Footer Template"
            description="HTML template used in the footer of the report."
            data-testid={testIds.appConfig.footerTemplate}
          >
            <TextArea
              id="footerTemplate"
              rows={20}
              className={s.textarea}
              label={`Footer Template`}
              onChange={onChangeFooterTemplate}
            >{state.footerTemplate}
            </TextArea>
          </Field>
        </ConfigSubSection>
      </ConfigSection>

      {/* Additional Settings */}
      <hr className={`${s.hrTopSpace} ${s.hrBottomSpace}`} />
      <ConfigSection
        title="Additional Settings"
        description="Additional settings are optional settings that can be configured for more control over the plugin app."
        isCollapsible
        isInitiallyOpen={false}
      >
        {/* Grafana Hostname */}
        <Field
          label="Grafana Hostname"
          description="Overrides the automatic grafana hostname detection. Use this if you have a reverse proxy in front of Grafana."
          data-testid={testIds.appConfig.appUrl}
          className={s.marginTop}
        >
          <Input
            type="url"
            width={60}
            id="appUrl"
            label={`Grafana Hostname`}
            value={state.appUrl}
            onChange={onChangeURL}
          />
        </Field>

        {/* Skip TLS verification */}
        <Field
          label="Skip TLS Verification"
          description="Do not validate TLS certificates when connecting to Grafana. NOTE: If using an remote chrome instance, set --ignore-certificate-errors flag in chrome."
          data-testid={testIds.appConfig.skipTlsCheck}
          className={s.marginTop}
        >
          <Switch
            id="skipTlsCheck"
            value={state.skipTlsCheck}
            onChange={onChangeSkipTlsCheck}
          />
        </Field>

        {/* Remote Chrome URL */}
        <Field
          label="Remote Chrome URL"
          description="Address to a running chrome instance with an listening chrome remote debug socket"
          data-testid={testIds.appConfig.remoteChromeUrl}
          className={s.marginTop}
        >
          <Input
            type="url"
            width={60}
            id="remoteChromeUrl"
            label={`Remote Chrome URL`}
            value={state.remoteChromeUrl}
            onChange={onChangeRemoteChromeURL}
          />
        </Field>

        {/* Max browser workers */}
        <Field
          label="Maximum Browser Workers"
          description="Maximum number of workers for interacting with chrome browser. Default is 2."
          className={s.marginTop}
        >
          <Input
            type="number"
            width={60}
            id="max-browser-workers"
            data-testid={testIds.appConfig.maxBrowserWorkers}
            label={`Maximum Browser Workers`}
            pattern={`[0-9]{1,2}`}
            value={state.maxBrowserWorkers}
            onChange={onChangeMaxBrowserWorkers}
          />
        </Field>

        {/* Max render workers */}
        <Field
          label="Maximum Render Workers"
          description="Maximum number of workers for rendering panels into PNGs. Default is 2."
          className={s.marginTop}
        >
          <Input
            type="number"
            width={60}
            id="max-render-workers"
            data-testid={testIds.appConfig.maxRenderWorkers}
            label={`Maximum Render Workers`}
            pattern={`[0-9]{1,2}`}
            value={state.maxRenderWorkers}
            onChange={onChangeMaxRenderWorkers}
          />
        </Field>

        {/* Custom HTTP Headers */}
        <Field
          label="Custom HTTP Headers"
          description="Custom HTTP headers to send to the render plugin. Provide as JSON object (e.g., {&quot;X-Custom-Header&quot;: &quot;value&quot;, &quot;Authorization&quot;: &quot;Bearer token&quot;})"
          className={s.marginTop}
        >
          <TextArea
            id="customHttpHeaders"
            rows={8}
            className={s.textarea}
            label={`Custom HTTP Headers`}
            placeholder='{\n  "X-Custom-Header": "value",\n  "Authorization": "Bearer token"\n}'
            value={state.customHttpHeaders}
            onChange={onChangeCustomHttpHeaders}
          />
        </Field>
      </ConfigSection>

      <div className={s.marginTop}>
        <Button
          type="submit"
          data-testid={testIds.appConfig.submit}
          onClick={() =>
            updatePluginAndReload(plugin.meta.id, {
              enabled,
              pinned,
              jsonData: {
                appUrl: state.appUrl,
                appVersion: state.appVersion,
                skipTlsCheck: state.skipTlsCheck,
                theme: state.theme,
                orientation: state.orientation,
                layout: state.layout,
                dashboardMode: state.dashboardMode,
                timeZone: state.timeZone,
                timeFormat: state.timeFormat,
                logo: state.logo,
                headerTemplate: state.headerTemplate,
                footerTemplate: state.footerTemplate,
                maxBrowserWorkers: state.maxBrowserWorkers,
                maxRenderWorkers: state.maxRenderWorkers,
                remoteChromeUrl: state.remoteChromeUrl,
                customHttpHeaders: state.customHttpHeaders ? JSON.parse(state.customHttpHeaders) : {},
                timeout: state.timeout,
                dialTimeout: state.dialTimeout,
                httpKeepAlive: state.httpKeepAlive,
                httpTLSHandshakeTimeout: state.httpTLSHandshakeTimeout,
                httpIdleConnTimeout: state.httpIdleConnTimeout,
                httpMaxConnsPerHost: state.httpMaxConnsPerHost,
                httpMaxIdleConns: state.httpMaxIdleConns,
                httpMaxIdleConnsPerHost: state.httpMaxIdleConnsPerHost,
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
            !state.appUrlChanged &&
              !state.skipTlsCheckChanged &&
              !state.themeChanged &&
              !state.layoutChanged &&
              !state.orientationChanged &&
              !state.dashboardModeChanged &&
              !state.timeZoneChanged &&
              !state.timeFormatChanged &&
              !state.logoChanged &&
              !state.headerTemplateChanged &&
              !state.footerTemplateChanged &&
              !state.maxBrowserWorkersChanged &&
              !state.maxRenderWorkersChanged &&
              !state.remoteChromeUrlChanged &&
              !state.customHttpHeadersChanged &&
              !state.isSaTokenReset &&
              !state.saToken
          )}
        >
          Save settings
        </Button>
      </div>
    </div>
  );
};

const getStyles = (theme: GrafanaTheme2) => ({
  colorWeak: css({
    color: `${theme.colors.text.secondary}`,
  }),
  marginTop: css({
    marginTop: `${theme.spacing(3)}`,
  }),
  marginTopXl: css({
    marginTop: `${theme.spacing(6)}`,
  }),
  hrBottomSpace: css({
    marginBottom: "56px",
  }),
  hrTopSpace: css({
    marginTop: "50px",
  }),
  reportSettings: css({
    paddingTop: "32px",
  }),
  textarea: css({
    width: "620px",
  })
});

const updatePluginAndReload = async (
  pluginId: string,
  data: Partial<PluginMeta<JsonData>>
) => {
  try {
    await updatePlugin(pluginId, data);

    // Reloading the page as the changes made here wouldn't be propagated to the actual plugin otherwise.
    // This is not ideal, however unfortunately currently there is no supported way for updating the plugin state.
    window.location.reload();
  } catch (e) {
    console.error("Error while updating the plugin", e);
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
