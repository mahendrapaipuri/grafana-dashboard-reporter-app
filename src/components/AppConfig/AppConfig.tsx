import React, { useState, ChangeEvent } from 'react';
import { lastValueFrom } from 'rxjs';
import { css } from '@emotion/css';
import {
  Button,
  useStyles2,
  Field,
  Input,
  TextArea,
  FieldSet,
  RadioButtonGroup,
} from "@grafana/ui";
import {
  PluginConfigPageProps,
  AppPluginMeta,
  PluginMeta,
  GrafanaTheme2,
} from "@grafana/data";
import { config, getBackendSrv } from "@grafana/runtime";
import { testIds } from "../testIds";

export type JsonData = {
  appUrl?: string;
  orientation?: string;
  layout?: string;
  maxRenderWorkers?: number;
  texTemplate?: string;
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
  // Maximum rendering workers
  maxRenderWorkers: number;
  // If maxRenderWorkers has changed
  maxRenderWorkersChanged: boolean;
  // The custom TeX template.
  texTemplate: string;
};

interface Props extends PluginConfigPageProps<AppPluginMeta<JsonData>> {}

export const AppConfig = ({ plugin }: Props) => {
  const s = useStyles2(getStyles);
  const { enabled, pinned, jsonData } = plugin.meta;
  const [state, setState] = useState<State>({
    orientation: jsonData?.orientation || "portrait",
    orientationChanged: false,
    layout: jsonData?.layout || "simple",
    layoutChanged: false,
    maxRenderWorkers: jsonData?.maxRenderWorkers || 2,
    maxRenderWorkersChanged: false,
    texTemplate: jsonData?.texTemplate || "",
  });

  const appUrl = config.appUrl;

  const orientationOptions = [
    { label: "Portrait", value: "portrait", icon: "gf-portrait" },
    { label: "Landscape", value: "landscape", icon: "gf-landscape" },
  ];

  const layoutOptions = [
    { label: "Simple", value: "simple", icon: "gf-layout-simple" },
    { label: "Grid", value: "grid", icon: "gf-grid" },
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

  const onChangeMaxWorkers = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      maxRenderWorkers: event.target.valueAsNumber,
      maxRenderWorkersChanged: true,
    });
  };

  const onChangeTexTemplate = (event: ChangeEvent<HTMLTextAreaElement>) => {
    setState({
      ...state,
      texTemplate: event.target.value,
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
                    maxRenderWorkers: state.maxRenderWorkers,
                    orientation: state.orientation,
                    layout: state.layout,
                    texTemplate: state.texTemplate,
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
                    maxRenderWorkers: state.maxRenderWorkers,
                    orientation: state.orientation,
                    layout: state.layout,
                    texTemplate: state.texTemplate,
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
        {/* Use Grid Layout */}
        <Field
          label="Layout"
          description="Display the panels in their positions on the dashboard."
          className={s.marginTop}
        >
          <RadioButtonGroup
            options={layoutOptions}
            value={state.layout}
            onChange={onChangeLayout}
          />
        </Field>

        {/* Report Orientation */}
        <Field label="Report Orientation" className={s.marginTop}>
          <RadioButtonGroup
            options={orientationOptions}
            value={state.orientation}
            onChange={onChangeOrientation}
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

        {/* Tex Template */}
        <Field label="TeX Template" description="Custom TeX template to use." className={s.marginTop}>
          <TextArea
            type="text"
            aria-label="TeX Template"
            data-testid={testIds.appConfig.texTemplate}
            value={state?.texTemplate}
            rows={20}
            onChange={onChangeTexTemplate}
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
                  maxRenderWorkers: state.maxRenderWorkers,
                  orientation: state.orientation,
                  layout: state.layout,
                  texTemplate: state.texTemplate,
                },
              })
            }
            disabled={Boolean(
                !state.texTemplate &&
                !state.layoutChanged &&
                !state.orientationChanged &&
                !state.maxRenderWorkersChanged
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
