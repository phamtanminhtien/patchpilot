import { createBrowserRouter } from "react-router";

import { VibePage } from "../features/vibe/vibe-page";
import { WorkspacePage } from "../features/workspace/workspace-page";
import { DefaultRoute } from "./default-route";

export const router = createBrowserRouter([
  {
    element: <DefaultRoute />,
    path: "/",
  },
  {
    element: <VibePage />,
    path: "/vibe",
  },
  {
    element: <WorkspacePage />,
    path: "/workspace",
  },
]);
