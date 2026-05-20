import { Navigate } from "react-router";

import { createDefaultMode } from "./use-default-mode";

export function DefaultRoute() {
  return (
    <Navigate
      replace
      to={createDefaultMode() === "vibe" ? "/vibe" : "/workspace"}
    />
  );
}
