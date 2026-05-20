import { createBrowserRouter } from "react-router";

import { DefaultRoute } from "@/app/default-route";
import { VibePage } from "@/features/vibe/vibe-page";
import { WorkspacePage } from "@/features/workspace/workspace-page";

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
