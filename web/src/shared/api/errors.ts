import { AxiosError } from "axios";

import type { RestErrorResponse } from "./types";

export function apiErrorMessage(error: unknown): string {
  if (error instanceof AxiosError) {
    const data = error.response?.data as Partial<RestErrorResponse> | undefined;
    return data?.error?.message ?? error.message;
  }

  if (error instanceof Error) {
    return error.message;
  }

  return "Request failed";
}
