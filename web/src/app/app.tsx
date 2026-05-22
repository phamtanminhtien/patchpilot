import {
  QueryClient,
  QueryClientProvider,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { NuqsAdapter } from "nuqs/adapters/react-router/v7";
import { type FormEvent, useState } from "react";
import { RouterProvider } from "react-router";

import { router } from "@/app/routes";
import { ThemeProvider } from "@/app/theme";
import { apiErrorMessage, getSession, login } from "@/shared/api";
import { Button, Surface, TextField } from "@/shared/ui";

export function App() {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            refetchOnWindowFocus: false,
            retry: 1,
            staleTime: 15_000,
          },
        },
      }),
  );

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AuthGate />
      </ThemeProvider>
    </QueryClientProvider>
  );
}

function AuthGate() {
  const queryClient = useQueryClient();
  const [token, setToken] = useState("");
  const sessionQuery = useQuery({
    queryFn: getSession,
    queryKey: ["auth-session"],
    retry: false,
  });
  const loginMutation = useMutation({
    mutationFn: login,
    onSuccess: () => {
      setToken("");
      void queryClient.invalidateQueries({ queryKey: ["auth-session"] });
    },
  });

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (token.trim().length === 0 || loginMutation.isPending) {
      return;
    }
    loginMutation.mutate(token.trim());
  }

  if (sessionQuery.isPending) {
    return (
      <main className="bg-app text-ink grid h-screen w-screen place-items-center p-4">
        <p className="text-muted text-sm">Checking session...</p>
      </main>
    );
  }

  if (sessionQuery.isError) {
    return (
      <main className="bg-app text-ink grid h-screen w-screen place-items-center p-4">
        <Surface className="grid w-full max-w-sm gap-4 p-4" as="section">
          <div className="grid gap-1">
            <h1 className="text-xl font-semibold">PatchPilot</h1>
            <p className="text-muted text-sm">Sign in with the admin token.</p>
          </div>
          <form className="grid gap-3" onSubmit={handleSubmit}>
            <TextField
              autoComplete="current-password"
              label="Admin token"
              onChange={(event) => setToken(event.target.value)}
              type="password"
              value={token}
            />
            {loginMutation.error ? (
              <p className="text-warning text-sm font-medium">
                {apiErrorMessage(loginMutation.error)}
              </p>
            ) : null}
            <Button disabled={loginMutation.isPending} type="submit">
              Sign in
            </Button>
          </form>
        </Surface>
      </main>
    );
  }

  return (
    <NuqsAdapter>
      <RouterProvider router={router} />
    </NuqsAdapter>
  );
}
