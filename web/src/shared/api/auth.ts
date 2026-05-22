import { apiClient } from "./client";
import type { AuthSession, AuthSessionResponse } from "./types";

export async function login(token: string): Promise<AuthSessionResponse> {
  const response = await apiClient.post<AuthSessionResponse>("/auth/login", {
    token,
  });
  return response.data;
}

export async function getSession(): Promise<AuthSession> {
  const response = await apiClient.get<AuthSessionResponse>("/auth/session");
  return response.data.session;
}

export async function logout(): Promise<void> {
  await apiClient.post("/auth/logout");
}
