import React, { useState, ChangeEvent } from 'react';
import { lastValueFrom } from 'rxjs';
import { css } from '@emotion/css';
import { Button, useStyles2, Field, Switch, Input, TextArea, FieldSet } from '@grafana/ui';
import { PluginConfigPageProps, AppPluginMeta, PluginMeta, GrafanaTheme2 } from '@grafana/data';
import { config, getBackendSrv } from '@grafana/runtime';
import { testIds } from '../testIds';

export type JsonData = {
  appUrl?: string;
  useGridLayout?: boolean;
  texTemplate?: string;
  maxRenderWorkers?: number;
};

type State = {
  // Use Grid layout in report
  useGridLayout: boolean;
  // The custom TeX template.
  texTemplate: string;
  // Maximum rendering workers
  maxRenderWorkers: number;
};

interface Props extends PluginConfigPageProps<AppPluginMeta<JsonData>> {}

export const AppConfig = ({ plugin }: Props) => {
  const s = useStyles2(getStyles);
  const { enabled, pinned, jsonData } = plugin.meta;
  const [state, setState] = useState<State>({
    useGridLayout: jsonData?.useGridLayout || false,
    texTemplate: jsonData?.texTemplate || '',
    maxRenderWorkers: jsonData?.maxRenderWorkers || 2,
  });

  const appUrl = config.appUrl;

  const texTemplatePlaceholder = `%use square brackets as golang text templating delimiters
\\documentclass{article}
\\usepackage{graphicx}
\\usepackage[margin=1in]{geometry}

\\graphicspath{ {images/} }
\\begin{document}
\\title{[[.Title]] [[if .VariableValues]] \\\\ \\large [[.VariableValues]] [[end]] [[if .Description]] \\\\ \\small [[.Description]] [[end]]}
\\date{[[.FromFormatted]]\\to\\[[.ToFormatted]]}
\\maketitle
\\begin{center}
[[range .Panels]][[if .IsSingleStat]]\\begin{minipage}{0.3\\textwidth}
\\includegraphics[width=\\textwidth]{image[[.Id]]}
\\end{minipage}
[[else]]\\par
\\vspace{0.5cm}
\\includegraphics[width=\\textwidth]{image[[.Id]]}
\\par
\\vspace{0.5cm}
[[end]][[end]]

\\end{center}
\\end{document}`

  const onChangeTexTemplate = (event: ChangeEvent<HTMLTextAreaElement>) => {
    setState({
      ...state,
      texTemplate: event.target.value,
    });
  };

  const onChangeGridLayout = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      useGridLayout: event.target.checked,
    });
  };

  const onChangeMaxWorkers = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      maxRenderWorkers: event.target.valueAsNumber,
    });
  };

  return (
    <div data-testid={testIds.appConfig.container}>
      {/* ENABLE / DISABLE PLUGIN */}
      <FieldSet label="Enable / Disable">
        {!enabled && (
          <>
            <div className={s.colorWeak}>The plugin is currently not enabled.</div>
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
                    useGridLayout: state.useGridLayout,
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
                    useGridLayout: state.useGridLayout,
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
        {/* Max workers */}
        <Field label="Maximum Render Workers" description="Maximum number of workers for rendering panels into PNGs. Default is 2." className={s.marginTop}>
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

        {/* Use Grid Layout */}
        <Field label="Use Grid Layout" description="If custom template is defined, this option will be ignored" className={s.marginTop}>
          <Switch
            id="grid-layout"
            label={`Use Grid Layout`}
            value={state?.useGridLayout}
            onChange={onChangeGridLayout}
          />
        </Field>

        {/* Tex Template */}
        <Field label="TeX Template" description="" className={s.marginTop}>
          <TextArea
            type="text"
            aria-label="TeX Template"
            data-testid={testIds.appConfig.texTemplate}
            value={state?.texTemplate}
            placeholder={texTemplatePlaceholder}
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
                  useGridLayout: state.useGridLayout,
                  texTemplate: state.texTemplate,
                },
              })
            }
            disabled={Boolean(!state.texTemplate && !state.useGridLayout && state.maxRenderWorkers === 2)}
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
