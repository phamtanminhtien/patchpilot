import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  CheckCircle2,
  Loader2,
  MonitorCog,
  RefreshCw,
  ServerCog,
  Settings,
  Trash2,
  Upload,
} from "lucide-react";
import { useQueryState } from "nuqs";
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router";

import { AppShell } from "@/app/app-shell";
import { useAppearance } from "@/app/appearance";
import { useThemePreference } from "@/app/theme";
import { skillDisplayName } from "@/features/vibe/lib/skills";
import { agentModels, reasoningEfforts } from "@/features/vibe/vibe-options";
import {
  type AgentModel,
  type AgentReasoningEffort,
  apiErrorMessage,
  deleteSettingsFont,
  getAgentContext,
  getSettings,
  getWorkspace,
  installSettingsFont,
  patchSettingsPreferences,
  setAgentSkillEnabled,
  type SettingsFont,
  type SettingsPreferences,
} from "@/shared/api";
import {
  Button,
  cn,
  Select,
  StatusPill,
  Surface,
  Switch,
  ThemeSwitcher,
} from "@/shared/ui";
import { workspaceIdParser } from "@/shared/url";

const categories = [
  { id: "appearance", label: "Appearance", icon: MonitorCog },
  { id: "agent", label: "Agent", icon: Settings },
  { id: "skills", label: "Skills", icon: CheckCircle2 },
  { id: "mcp", label: "MCP", icon: ServerCog },
  { id: "server", label: "Server", icon: ServerCog },
] as const;

type CategoryId = (typeof categories)[number]["id"];

const builtinFonts = [
  { label: "System UI", value: "Inter, ui-sans-serif, system-ui, sans-serif" },
  {
    label: "System mono",
    value: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
  },
];

export function SettingsPage() {
  const [workspaceId] = useQueryState("workspaceId", workspaceIdParser);
  const [activeCategory, setActiveCategory] =
    useState<CategoryId>("appearance");
  const settingsQuery = useQuery({
    queryFn: getSettings,
    queryKey: ["settings"],
  });
  const workspaceQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => getWorkspace(workspaceId),
    queryKey: ["workspace", workspaceId],
  });
  const { preference, setPreference } = useThemePreference();
  const appearance = useAppearance();

  useEffect(() => {
    const preferences = settingsQuery.data?.preferences;
    if (!preferences) {
      return;
    }
    setPreference(preferences.theme);
    appearance.applyPreferences(preferences);
    storeAgentDefaults(preferences);
  }, [appearance, setPreference, settingsQuery.data?.preferences]);

  if (workspaceId.length === 0) {
    return (
      <main className="bg-canvas grid h-screen w-screen place-items-center p-4">
        <Surface className="grid w-full max-w-md gap-4 p-4" as="section">
          <div className="flex items-center justify-between gap-3">
            <div className="grid gap-1">
              <h1 className="text-ink text-lg font-semibold">Settings</h1>
              <p className="text-muted text-sm">
                Open a repo to manage app settings.
              </p>
            </div>
            <ThemeSwitcher onChange={setPreference} value={preference} />
          </div>
          <Button asChild variant="action">
            <Link to="/vibe">Open repo</Link>
          </Button>
        </Surface>
      </main>
    );
  }

  return (
    <AppShell
      mode="settings"
      workspace={workspaceQuery.data}
      workspaceId={workspaceId}
    >
      <main className="grid h-[calc(100vh-2.5rem)] min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden lg:grid-cols-[13rem_minmax(0,1fr)] lg:grid-rows-1">
        <nav className="bg-panel flex gap-1 overflow-x-auto px-2 py-2 shadow-sm lg:grid lg:min-h-0 lg:content-start lg:overflow-auto">
          {categories.map((item) => (
            <button
              className={cn(
                "text-muted hover:bg-hover hover:text-ink min-h-touch inline-flex min-w-28 items-center justify-center gap-2 rounded-md px-3 text-sm font-medium transition lg:justify-start",
                activeCategory === item.id
                  ? "bg-accent-soft text-accent shadow-sm"
                  : undefined,
              )}
              key={item.id}
              onClick={() => setActiveCategory(item.id)}
              type="button"
            >
              <item.icon aria-hidden="true" className="size-4" />
              <span className="truncate">{item.label}</span>
            </button>
          ))}
        </nav>

        <section className="min-h-0 overflow-auto px-3 py-3 sm:px-4">
          <SettingsFontFaces fonts={settingsQuery.data?.fonts ?? []} />
          <div className="mx-auto grid max-w-5xl gap-3">
            {settingsQuery.isLoading ? (
              <Surface className="text-muted p-4 text-sm">
                Loading settings
              </Surface>
            ) : null}
            {settingsQuery.error ? (
              <Surface className="text-warning p-4 text-sm font-medium">
                {apiErrorMessage(settingsQuery.error)}
              </Surface>
            ) : null}
            {settingsQuery.data ? (
              <SettingsContent
                activeCategory={activeCategory}
                settings={settingsQuery.data}
                workspaceId={workspaceId}
              />
            ) : null}
          </div>
        </section>
      </main>
    </AppShell>
  );
}

function SettingsContent({
  activeCategory,
  settings,
  workspaceId,
}: {
  activeCategory: CategoryId;
  settings: Awaited<ReturnType<typeof getSettings>>;
  workspaceId: string;
}) {
  if (activeCategory === "appearance") {
    return <AppearanceSettings settings={settings} />;
  }
  if (activeCategory === "agent") {
    return <AgentSettings preferences={settings.preferences} />;
  }
  if (activeCategory === "skills") {
    return <SkillsSettings workspaceId={workspaceId} />;
  }
  if (activeCategory === "mcp") {
    return <McpSettings workspaceId={workspaceId} />;
  }
  return <ServerSettings status={settings.serverStatus} />;
}

function AppearanceSettings({
  settings,
}: {
  settings: Awaited<ReturnType<typeof getSettings>>;
}) {
  const { preference, setPreference } = useThemePreference();
  const queryClient = useQueryClient();
  const appearance = useAppearance();
  const mutation = useMutation({
    mutationFn: patchSettingsPreferences,
    onSuccess: (data) => {
      setPreference(data.preferences.theme);
      appearance.applyPreferences(data.preferences);
      void queryClient.setQueryData(["settings"], data);
    },
  });
  const fontOptions = useMemo(
    () => [
      ...builtinFonts,
      ...settings.fonts.map((font) => ({
        label: font.family,
        value: font.family,
      })),
    ],
    [settings.fonts],
  );
  const preferences = settings.preferences;

  function patch(preferencePatch: Partial<SettingsPreferences>) {
    mutation.mutate(preferencePatch);
  }

  return (
    <Surface className="grid gap-4 p-4" as="section">
      <SectionHeader title="Appearance" detail="Theme and local font roles." />
      <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <SettingRow label="Theme">
          <ThemeSwitcher
            onChange={(theme) => {
              setPreference(theme);
              patch({ theme });
            }}
            value={preference}
          />
        </SettingRow>
        <FontSelect
          label="App font"
          onChange={(appFontFamily) => patch({ appFontFamily })}
          options={fontOptions}
          value={preferences.appFontFamily}
        />
        <FontSelect
          label="Code font"
          onChange={(codeFontFamily) => patch({ codeFontFamily })}
          options={fontOptions}
          value={preferences.codeFontFamily}
        />
        <FontSelect
          label="Terminal font"
          onChange={(terminalFontFamily) => patch({ terminalFontFamily })}
          options={fontOptions}
          value={preferences.terminalFontFamily}
        />
      </div>
      <FontPreview preferences={preferences} />
      <InstalledFonts fonts={settings.fonts} />
      {mutation.error ? (
        <p className="text-warning text-sm font-medium">
          {apiErrorMessage(mutation.error)}
        </p>
      ) : null}
    </Surface>
  );
}

function FontSelect({
  label,
  onChange,
  options,
  value,
}: {
  label: string;
  onChange: (value: string) => void;
  options: { label: string; value: string }[];
  value: string;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const selectOptions = useMemo(() => {
    if (options.some((option) => option.value === value)) {
      return options;
    }
    return [{ label: value, value }, ...options];
  }, [options, value]);

  function commitDraft() {
    const trimmed = inputRef.current?.value.trim() ?? "";
    if (trimmed.length > 0 && trimmed !== value) {
      onChange(trimmed);
    }
  }

  return (
    <SettingRow label={label}>
      <div className="grid gap-2">
        <Select
          label={label}
          onValueChange={(nextValue) => {
            if (inputRef.current) {
              inputRef.current.value = nextValue;
            }
            onChange(nextValue);
          }}
          options={selectOptions}
          size="small"
          triggerClassName="w-full"
          value={value}
        />
        <input
          className="bg-panel text-ink focus-visible:outline-focus focus-visible:outline-offset-focus focus-visible:outline-width-focus min-h-9 rounded-md px-2.5 text-xs shadow-sm"
          defaultValue={value}
          key={value}
          onBlur={commitDraft}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.currentTarget.blur();
            }
          }}
          placeholder="Font family, fallback"
          ref={inputRef}
        />
      </div>
    </SettingRow>
  );
}

function FontPreview({ preferences }: { preferences: SettingsPreferences }) {
  return (
    <div className="bg-hover grid gap-2 rounded-md p-3 text-sm">
      <p
        className="text-ink font-semibold"
        style={{ fontFamily: preferences.appFontFamily }}
      >
        App preview: Settings, approvals, workspace controls
      </p>
      <pre
        className="min-w-0 overflow-hidden text-xs text-ellipsis"
        style={{ fontFamily: preferences.codeFontFamily }}
      >
        {"const patch = await pilot.apply(change);"}
      </pre>
      <pre
        className="min-w-0 overflow-hidden text-xs text-ellipsis"
        style={{ fontFamily: preferences.terminalFontFamily }}
      >
        $ pnpm --dir web test
      </pre>
    </div>
  );
}

function InstalledFonts({ fonts }: { fonts: SettingsFont[] }) {
  const [file, setFile] = useState<File | null>(null);
  const [family, setFamily] = useState("");
  const queryClient = useQueryClient();
  const installMutation = useMutation({
    mutationFn: () => installSettingsFont(file as File, family),
    onSuccess: () => {
      setFile(null);
      setFamily("");
      void queryClient.invalidateQueries({ queryKey: ["settings"] });
    },
  });
  const deleteMutation = useMutation({
    mutationFn: deleteSettingsFont,
    onSuccess: () =>
      void queryClient.invalidateQueries({ queryKey: ["settings"] }),
  });

  return (
    <div className="grid gap-3">
      <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
        <input
          className="bg-panel text-ink min-h-touch rounded-md px-3 text-sm shadow-sm"
          onChange={(event) => setFamily(event.target.value)}
          placeholder="Font family"
          value={family}
        />
        <input
          accept=".woff2,.woff,.ttf,.otf"
          className="bg-panel text-ink min-h-touch rounded-md px-3 py-2 text-sm shadow-sm"
          onChange={(event) => setFile(event.target.files?.[0] ?? null)}
          type="file"
        />
        <Button
          disabled={
            !file || family.trim().length === 0 || installMutation.isPending
          }
          icon={
            installMutation.isPending ? (
              <Loader2 className="animate-spin" />
            ) : (
              <Upload />
            )
          }
          onClick={() => installMutation.mutate()}
          type="button"
          variant="action"
        >
          Install
        </Button>
      </div>
      <div className="grid gap-1">
        {fonts.length ? (
          fonts.map((font) => (
            <div
              className="bg-hover grid min-h-11 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 rounded-md px-3"
              key={font.id}
            >
              <div className="min-w-0">
                <p className="text-ink truncate text-sm font-medium">
                  {font.family}
                </p>
                <p className="text-muted truncate text-xs">{font.filename}</p>
              </div>
              <Button
                aria-label={`Delete ${font.family}`}
                disabled={deleteMutation.isPending}
                icon={<Trash2 />}
                onClick={() => deleteMutation.mutate(font.id)}
                size="small"
                type="button"
                variant="secondary"
              />
            </div>
          ))
        ) : (
          <p className="text-muted text-sm">No local fonts installed.</p>
        )}
      </div>
      {installMutation.error || deleteMutation.error ? (
        <p className="text-warning text-sm font-medium">
          {apiErrorMessage(installMutation.error ?? deleteMutation.error)}
        </p>
      ) : null}
    </div>
  );
}

function AgentSettings({ preferences }: { preferences: SettingsPreferences }) {
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: patchSettingsPreferences,
    onSuccess: (data) => {
      storeAgentDefaults(data.preferences);
      void queryClient.setQueryData(["settings"], data);
    },
  });
  return (
    <Surface className="grid gap-4 p-4" as="section">
      <SectionHeader
        title="Agent defaults"
        detail="Used for future composer submissions."
      />
      <div className="grid gap-3 sm:grid-cols-2">
        <SettingRow label="Default model">
          <Select
            label="Default model"
            onValueChange={(defaultModel) =>
              mutation.mutate({ defaultModel: defaultModel as AgentModel })
            }
            options={agentModels.map((value) => ({ value }))}
            value={preferences.defaultModel}
          />
        </SettingRow>
        <SettingRow label="Reasoning">
          <Select
            label="Reasoning"
            onValueChange={(defaultReasoningEffort) =>
              mutation.mutate({
                defaultReasoningEffort:
                  defaultReasoningEffort as AgentReasoningEffort,
              })
            }
            options={reasoningEfforts.map((value) => ({ value }))}
            value={preferences.defaultReasoningEffort}
          />
        </SettingRow>
      </div>
      {mutation.error ? (
        <p className="text-warning text-sm font-medium">
          {apiErrorMessage(mutation.error)}
        </p>
      ) : null}
    </Surface>
  );
}

function SkillsSettings({ workspaceId }: { workspaceId: string }) {
  const queryClient = useQueryClient();
  const contextQuery = useQuery({
    queryFn: () => getAgentContext(workspaceId),
    queryKey: ["agent-context", workspaceId],
  });
  const mutation = useMutation({
    mutationFn: ({ key, enabled }: { key: string; enabled: boolean }) =>
      setAgentSkillEnabled(workspaceId, key, enabled),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ["agent-context", workspaceId],
      }),
  });
  const skills = contextQuery.data?.skills ?? [];
  return (
    <Surface className="grid gap-4 p-4" as="section">
      <SectionHeader
        title="Skills"
        detail="Local skills discovered for agent context."
      />
      {contextQuery.isLoading ? (
        <p className="text-muted text-sm">Loading skills</p>
      ) : null}
      <div className="grid gap-1">
        {skills.map((skill) => (
          <div
            className="bg-hover grid min-h-14 grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-md px-3 py-2"
            key={skill.key}
          >
            <div className="min-w-0">
              <p className="text-ink truncate text-sm font-medium">
                {skillDisplayName(skill)}
              </p>
              <p className="text-muted line-clamp-2 text-xs">
                {skill.description || skill.source}
              </p>
            </div>
            <Switch
              aria-label={`Toggle ${skillDisplayName(skill)}`}
              checked={skill.enabled}
              disabled={mutation.isPending}
              onCheckedChange={(enabled) =>
                mutation.mutate({ key: skill.key, enabled })
              }
            />
          </div>
        ))}
      </div>
      {contextQuery.error || mutation.error ? (
        <p className="text-warning text-sm font-medium">
          {apiErrorMessage(contextQuery.error ?? mutation.error)}
        </p>
      ) : null}
    </Surface>
  );
}

function McpSettings({ workspaceId }: { workspaceId: string }) {
  const contextQuery = useQuery({
    queryFn: () => getAgentContext(workspaceId),
    queryKey: ["agent-context", workspaceId],
  });
  const servers = contextQuery.data?.mcpServers ?? [];
  const tools = contextQuery.data?.mcpTools ?? [];
  return (
    <Surface className="grid gap-4 p-4" as="section">
      <SectionHeader
        title="MCP"
        detail="Configured local MCP servers and tools."
      />
      <div className="grid gap-2">
        {servers.length ? (
          servers.map((server) => (
            <div className="bg-hover grid gap-2 rounded-md p-3" key={server.id}>
              <div className="flex min-w-0 items-center justify-between gap-2">
                <p className="text-ink truncate text-sm font-medium">
                  {server.name}
                </p>
                <StatusPill status={server.status} />
              </div>
              <p className="text-muted text-xs">
                {server.transport} · approval {server.approvalPolicy}
              </p>
              {server.lastError ? (
                <p className="text-warning text-xs">{server.lastError}</p>
              ) : null}
              <p className="text-muted text-xs">
                {tools.filter((tool) => tool.serverId === server.id).length}{" "}
                tools
              </p>
            </div>
          ))
        ) : (
          <p className="text-muted text-sm">No MCP servers configured.</p>
        )}
      </div>
    </Surface>
  );
}

function ServerSettings({
  status,
}: {
  status: Awaited<ReturnType<typeof getSettings>>["serverStatus"];
}) {
  const rows = [
    ["Provider", status.providerConfigured ? "configured" : "missing"],
    ["OpenAI base URL", status.openAIBaseUrlHost || "default"],
    ["Light model", status.lightModel],
    ["Allowed roots", String(status.allowedRootsCount)],
    ["Log format", status.logFormat || "console"],
    ["Static files", status.staticDirConfigured ? "configured" : "missing"],
  ];
  return (
    <Surface className="grid gap-4 p-4" as="section">
      <SectionHeader title="Server" detail="Safe runtime status only." />
      <div className="grid gap-1">
        {rows.map(([label, value]) => (
          <div
            className="bg-hover grid min-h-11 grid-cols-[9rem_minmax(0,1fr)] items-center gap-2 rounded-md px-3"
            key={label}
          >
            <p className="text-muted text-xs font-semibold uppercase">
              {label}
            </p>
            <p className="text-ink min-w-0 truncate text-sm font-medium">
              {value}
            </p>
          </div>
        ))}
      </div>
    </Surface>
  );
}

function SettingRow({
  children,
  label,
}: {
  children: ReactNode;
  label: string;
}) {
  return (
    <label className="bg-hover grid min-w-0 gap-2 rounded-md p-3">
      <span className="text-muted text-xs font-semibold uppercase">
        {label}
      </span>
      {children}
    </label>
  );
}

function SectionHeader({ detail, title }: { detail: string; title: string }) {
  return (
    <div className="flex min-w-0 items-center justify-between gap-3">
      <div className="grid min-w-0 gap-1">
        <h1 className="text-ink truncate text-lg font-semibold">{title}</h1>
        <p className="text-muted text-sm">{detail}</p>
      </div>
      <RefreshCw aria-hidden="true" className="text-muted size-4" />
    </div>
  );
}

function SettingsFontFaces({ fonts }: { fonts: SettingsFont[] }) {
  const css = fonts
    .map(
      (font) =>
        `@font-face{font-family:${JSON.stringify(font.family)};src:url(${JSON.stringify(font.url)});font-display:swap;}`,
    )
    .join("\n");
  return css ? <style>{css}</style> : null;
}

function storeAgentDefaults(preferences: SettingsPreferences) {
  try {
    globalThis.localStorage?.setItem(
      "patchpilot.agentDefaults",
      JSON.stringify({
        model: preferences.defaultModel,
        reasoningEffort: preferences.defaultReasoningEffort,
      }),
    );
  } catch {
    // Default persistence is best-effort; backend settings remain source of truth.
  }
}
