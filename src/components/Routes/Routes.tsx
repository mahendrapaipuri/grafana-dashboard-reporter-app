import * as React from "react";
import { Route, Switch } from "react-router-dom";
import { Status } from "../../pages/Status";
import { AppConfig } from "../AppConfig";
import { useNavigation, prefixRoute } from "../../utils/utils.routing";
import { ROUTES } from "../../constants";

export const Routes = () => {
  useNavigation();

  return (
    <Switch>
      <Route
        exact
        path={prefixRoute(`${ROUTES.Config}`)}
        component={AppConfig}
      />
      {/* Status page */}
      <Route component={Status} />
    </Switch>
  );
};
