import * as React from "react";
import { Route, Switch } from "react-router-dom";
import { Status } from "../../pages/Status";

export const Routes = () => {
  return (
    <Switch>
      {/* Status page */}
      <Route component={Status} />
    </Switch>
  );
};
