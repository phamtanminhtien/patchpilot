import { apiClient } from "./client";
import type {
  PatchSettingsPreferencesRequest,
  SettingsFontListResponse,
  SettingsFontResponse,
  SettingsResponse,
} from "./types";

export async function getSettings(): Promise<SettingsResponse> {
  const response = await apiClient.get<SettingsResponse>("/settings");
  return response.data;
}

export async function patchSettingsPreferences(
  preferences: PatchSettingsPreferencesRequest,
): Promise<SettingsResponse> {
  const response = await apiClient.patch<SettingsResponse>(
    "/settings/preferences",
    preferences,
  );
  return response.data;
}

export async function listSettingsFonts(): Promise<SettingsFontListResponse> {
  const response =
    await apiClient.get<SettingsFontListResponse>("/settings/fonts");
  return response.data;
}

export async function installSettingsFont(
  file: File,
  family: string,
): Promise<SettingsFontResponse> {
  const body = new FormData();
  body.append("family", family);
  body.append("file", file);
  const response = await apiClient.post<SettingsFontResponse>(
    "/settings/fonts",
    body,
  );
  return response.data;
}

export async function deleteSettingsFont(fontId: string): Promise<void> {
  await apiClient.delete(`/settings/fonts/${encodeURIComponent(fontId)}`);
}
