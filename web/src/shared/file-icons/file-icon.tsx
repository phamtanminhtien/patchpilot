import { cn } from "@/shared/ui/class-name";

const fileNameIcons: Record<string, string> = {
  ".dockerignore": "file_type_docker",
  ".env": "file_type_dotenv",
  ".env.example": "file_type_dotenv",
  ".eslintrc": "file_type_eslint",
  ".gitignore": "file_type_git",
  ".npmrc": "file_type_npm",
  ".prettierrc": "file_type_prettier",
  "commitlint.config.js": "file_type_commitlint",
  "commitlint.config.ts": "file_type_commitlint",
  dockerfile: "file_type_docker",
  "eslint.config.js": "file_type_eslint",
  "eslint.config.mjs": "file_type_eslint",
  "eslint.config.ts": "file_type_eslint",
  "go.mod": "file_type_go_package",
  "go.sum": "file_type_go",
  "go.work": "file_type_go_work",
  "package.json": "file_type_node",
  "pnpm-lock.yaml": "file_type_pnpm",
  "tsconfig.json": "file_type_tsconfig",
  "vite.config.ts": "file_type_vite",
  "vitest.config.ts": "file_type_vitest",
};

const extensionIcons: Record<string, string> = {
  bash: "file_type_shell",
  cjs: "file_type_js",
  css: "file_type_css",
  db: "file_type_db",
  diff: "file_type_diff",
  gif: "file_type_image",
  go: "file_type_go",
  htm: "file_type_html",
  html: "file_type_html",
  jpeg: "file_type_image",
  jpg: "file_type_image",
  js: "file_type_js",
  json: "file_type_json",
  jsx: "file_type_reactjs",
  log: "file_type_log",
  md: "file_type_markdown",
  mdx: "file_type_mdx",
  mjs: "file_type_js",
  pdf: "file_type_pdf",
  png: "file_type_image",
  sh: "file_type_shell",
  sql: "file_type_sql",
  sqlite: "file_type_sqlite",
  svg: "file_type_svg",
  ts: "file_type_typescript",
  tsx: "file_type_reactts",
  txt: "file_type_text",
  webp: "file_type_image",
  yaml: "file_type_yaml",
  yml: "file_type_yaml",
  zsh: "file_type_shell",
};

const folderIcons: Record<string, string> = {
  ".git": "folder_type_git",
  ".github": "folder_type_github",
  api: "folder_type_api",
  app: "folder_type_app",
  assets: "folder_type_asset",
  build: "folder_type_dist",
  client: "folder_type_client",
  components: "folder_type_component",
  config: "folder_type_config",
  coverage: "folder_type_coverage",
  dist: "folder_type_dist",
  docs: "folder_type_docs",
  images: "folder_type_images",
  node_modules: "folder_type_node",
  public: "folder_type_public",
  scripts: "folder_type_script",
  server: "folder_type_server",
  shared: "folder_type_shared",
  src: "folder_type_src",
  styles: "folder_type_style",
  test: "folder_type_test",
  tests: "folder_type_test",
  web: "folder_type_www",
};

export interface FileIconProps {
  className?: string;
  isDirectory?: boolean;
  isExpanded?: boolean;
  name?: string;
  path: string;
}

export function FileIcon({
  className,
  isDirectory = false,
  isExpanded = false,
  name,
  path,
}: FileIconProps) {
  const iconName = resolveFileIconName({
    isDirectory,
    isExpanded,
    name,
    path,
  });

  return (
    <img
      alt=""
      aria-hidden="true"
      className={cn("size-3 shrink-0", className)}
      draggable={false}
      src={iconUrl(iconName)}
    />
  );
}

export function resolveFileIconName({
  isDirectory = false,
  isExpanded = false,
  name,
  path,
}: Omit<FileIconProps, "className">) {
  const basename = normalizeName(name ?? basenameFromPath(path));

  if (isDirectory) {
    const folderIcon = folderIcons[basename];
    if (!folderIcon) {
      return isExpanded ? "default_folder_opened" : "default_folder";
    }
    return isExpanded ? `${folderIcon}_opened` : folderIcon;
  }

  const fileNameIcon = fileNameIcons[basename];
  if (fileNameIcon) {
    return fileNameIcon;
  }

  const extension = basename.slice(basename.lastIndexOf(".") + 1);
  return extensionIcons[extension] ?? "default_file";
}

function basenameFromPath(path: string) {
  const normalized = path.endsWith("/") ? path.slice(0, -1) : path;
  return normalized.slice(normalized.lastIndexOf("/") + 1) || normalized;
}

function normalizeName(name: string) {
  return name.toLowerCase();
}

function iconUrl(iconName: string) {
  return `/icons/${iconName}.svg`;
}
