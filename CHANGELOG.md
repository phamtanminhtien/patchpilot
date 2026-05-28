# Changelog

## [0.5.0](https://github.com/phamtanminhtien/patchpilot/compare/patchpilot-v0.4.0...patchpilot-v0.5.0) (2026-05-28)


### Features

* **composer:** add workspace permission controls ([126b144](https://github.com/phamtanminhtien/patchpilot/commit/126b14496c214ddf62b6fda5663d37af326fdfd4))
* implement unified diff parsing and a dedicated UI component for rendering diffs with syntax highlighting and file-level collapsing ([852c64c](https://github.com/phamtanminhtien/patchpilot/commit/852c64ca576be1feadf105a3828d0a15c677233e))
* **workspace:** add git status bar controls ([784ba4a](https://github.com/phamtanminhtien/patchpilot/commit/784ba4a12b193091600d7545bc5e96e205ed2082))
* **workspace:** add indexed file search and command palette ([dc01054](https://github.com/phamtanminhtien/patchpilot/commit/dc01054eb151ed3b1256fab38da7edae0b9e2954))


### Bug Fixes

* **agent:** reject pending approvals on cancel ([1821d76](https://github.com/phamtanminhtien/patchpilot/commit/1821d76ad1ba08cf439665d8767e2d00c8933272))

## [0.4.0](https://github.com/phamtanminhtien/patchpilot/compare/patchpilot-v0.3.0...patchpilot-v0.4.0) (2026-05-27)


### Features

* add automatic background conversation title generation using a light model ([fa4bfc9](https://github.com/phamtanminhtien/patchpilot/commit/fa4bfc9aa562516b9e9446e52f33050d063d91ca))
* add settings page and corresponding API handlers for server configuration management ([5451ec6](https://github.com/phamtanminhtien/patchpilot/commit/5451ec6593a245668e66f74816755c6e22121992))
* implement code editor component using CodeMirror and add required language dependencies ([636d1f8](https://github.com/phamtanminhtien/patchpilot/commit/636d1f824d25cda1d099c80f6fb3708e59031af1))
* implement persistent terminal session management with database support and workspace integration ([3a7de4a](https://github.com/phamtanminhtien/patchpilot/commit/3a7de4a47d1279a1912aa56841286d2de46388f5))
* **vibe:** add tiptap composer suggestions ([#36](https://github.com/phamtanminhtien/patchpilot/issues/36)) ([57debba](https://github.com/phamtanminhtien/patchpilot/commit/57debbadfa8e570e20fe9d1cbd46c9f49222c0c2))

## [0.3.0](https://github.com/phamtanminhtien/patchpilot/compare/patchpilot-v0.2.2...patchpilot-v0.3.0) (2026-05-25)


### Features

* add support for human-readable skill names and implement use_skill tool call visualization ([a511c94](https://github.com/phamtanminhtien/patchpilot/commit/a511c9432968c65004694f4d75dd8041ad340c87))
* implement agent context and skill management system with new API endpoints and UI components ([0e8fb8b](https://github.com/phamtanminhtien/patchpilot/commit/0e8fb8be31ac28c77eb0c9378d5317b92201110c))

## [0.2.2](https://github.com/phamtanminhtien/patchpilot/compare/patchpilot-v0.2.1...patchpilot-v0.2.2) (2026-05-24)


### Bug Fixes

* **docker:** publish multi-arch release images ([#29](https://github.com/phamtanminhtien/patchpilot/issues/29)) ([f5358df](https://github.com/phamtanminhtien/patchpilot/commit/f5358df794351e7af682d1e04b63da3b7863c832))

## [0.2.1](https://github.com/phamtanminhtien/patchpilot/compare/patchpilot-v0.2.0...patchpilot-v0.2.1) (2026-05-24)


### Bug Fixes

* **deps:** update x/sys vulnerability ([5d9bbc7](https://github.com/phamtanminhtien/patchpilot/commit/5d9bbc7b14c50acd375e39a3a464d17897d36be8))
* implement command execution allowlisting and safely isolate git path processing ([6272174](https://github.com/phamtanminhtien/patchpilot/commit/62721744e6ed090ff6348719983076ec754002bb))
* **security:** block unsafe command execution ([dc36eef](https://github.com/phamtanminhtien/patchpilot/commit/dc36eefb859991a6b4e12e3a96114c882bed1425))
* **security:** harden high code scanning findings ([3790631](https://github.com/phamtanminhtien/patchpilot/commit/37906316bc5d7f95ae892c054358c0f03ae3fef6))
* **vibe:** keep auto-scroll following streamed updates ([aa2f368](https://github.com/phamtanminhtien/patchpilot/commit/aa2f36883370de7020029638663decbe106890fe))

## [0.2.0](https://github.com/phamtanminhtien/patchpilot/compare/patchpilot-v0.1.0...patchpilot-v0.2.0) (2026-05-24)


### Features

* add CodeQL and OpenSSF Scorecard workflows for security analysis ([81c75cd](https://github.com/phamtanminhtien/patchpilot/commit/81c75cd67066bb0eb4d7055873abe48411b22f18))
* add conversationId to URL state and include E2E tests in the build pipeline ([4f2be12](https://github.com/phamtanminhtien/patchpilot/commit/4f2be12ebfbeec6ff7db27b46878c5d84b5d1c51))
* add file search functionality to the workspace sidebar ([5c53039](https://github.com/phamtanminhtien/patchpilot/commit/5c530393ba112518306968304192f8fb3a85fc30))
* add Markdown rendering with GitHub-flavored Markdown support, syntax highlighting, and copyable code blocks to Vibe chat messages ([0c3b7ee](https://github.com/phamtanminhtien/patchpilot/commit/0c3b7eea2a5c117de85fd6558c5347e16c4f1339))
* align conversation run model ([0d59280](https://github.com/phamtanminhtien/patchpilot/commit/0d5928091c10f7bc1aa14e00fd0b06257dc0c2cb))
* **api:** add cursor pagination ([db4fd61](https://github.com/phamtanminhtien/patchpilot/commit/db4fd61f867742d96a9ad3c82047768150552552))
* implement auto-scroll timeline with jump-to-latest control and set default Vibe page state to new conversation ([7b2bcb7](https://github.com/phamtanminhtien/patchpilot/commit/7b2bcb785eb51417164288c40206036c6e92e64d))
* implement comprehensive database schema and repository layer for managing workspaces, commands, auth, and file indexing ([8adaf29](https://github.com/phamtanminhtien/patchpilot/commit/8adaf29b4189214c75a93a9ba787ec4c8a4bfd31))
* implement confirmation dialog for discarding git changes and add supporting UI components ([5bf2560](https://github.com/phamtanminhtien/patchpilot/commit/5bf25600b23cc56b9dcaab2fb85afe9554e0b527))
* implement conversation context summarization with sliding window message management and database tracking ([3b3ee9d](https://github.com/phamtanminhtien/patchpilot/commit/3b3ee9dc36322ff177800be0b6c647f20513f270))
* implement conversation search functionality with new UI dialog and backend filtering ([a0f26b0](https://github.com/phamtanminhtien/patchpilot/commit/a0f26b03a592d4b67ce5c84a5934cbda70185809))
* implement graceful shutdown for agent runs and active commands during server termination ([3a99682](https://github.com/phamtanminhtien/patchpilot/commit/3a99682e4258b5f149c5ac868881c2f50dfeab21))
* implement streaming support for OpenAI responses, add active run tracking, and enable run cancellation ([c294cef](https://github.com/phamtanminhtien/patchpilot/commit/c294cef4aa8b9d1056a62abafd03aff142315a64))
* replace custom select control with reusable Radix UI-based Select component ([68e4b8e](https://github.com/phamtanminhtien/patchpilot/commit/68e4b8e8ffadfde5900aafcfe41d75b872e6d742))


### Bug Fixes

* **commands:** lock user command safety policy ([e8ef080](https://github.com/phamtanminhtien/patchpilot/commit/e8ef0800ba623a940b99f9d9baf4c554fa36c5a0))
* **files:** implement manual text writes ([e42b8c6](https://github.com/phamtanminhtien/patchpilot/commit/e42b8c671d6cada1347f775e099b2692c182254d))

## 0.1.0 (2026-05-22)


### Features

* add configuration file for Air and enhance UI components with updated text sizing and new TextField tests ([ad0e0fa](https://github.com/phamtanminhtien/patchpilot/commit/ad0e0fa0001560ef508a1a727252a69299965219))
* add extensive collection of file and folder type SVG icons to public assets ([3e4e367](https://github.com/phamtanminhtien/patchpilot/commit/3e4e3670feb01105a3a992f60856878a5f0a014d))
* add Makefile and Git hooks for automated commit checks, implement initial web application structure with React, routing, and API integration ([63cd88d](https://github.com/phamtanminhtien/patchpilot/commit/63cd88da61335b5e68946ae18931217cef058788))
* enhance UI components with new Surface component and update styling for consistency across VibePage and WorkspacePage ([be5ca82](https://github.com/phamtanminhtien/patchpilot/commit/be5ca825634a75ff77d737f1bae24356ad2486ab))
* **file-api:** implement file search functionality and enhance file handling with size and ignored path checks ([c5d3a3e](https://github.com/phamtanminhtien/patchpilot/commit/c5d3a3ec767f6b75309000ae0e0c4da520bf8f67))
* **file-index:** implement file index management with refresh capability and enhance API for file indexing in workspaces ([7f3d0e8](https://github.com/phamtanminhtien/patchpilot/commit/7f3d0e84453fe95a0ceb6919d110f7d121391ff2))
* implement agent task framework with database persistence, API definitions, and core task handling logic ([b101e88](https://github.com/phamtanminhtien/patchpilot/commit/b101e88b313522ca09d23a9597eb8f97163e7470))
* implement authentication, port scanning, and proxying support alongside workspace patch management ([0f10c15](https://github.com/phamtanminhtien/patchpilot/commit/0f10c15691fefa987b9b525d0c69773410c84639))
* implement Git change parsing and enhance workspace UI with new components and styling updates ([82ec664](https://github.com/phamtanminhtien/patchpilot/commit/82ec6640d88cec6ad5d32b94cbf32ae444a1be72))
* implement Git staging, unstage, discard, and commit functionality in workspace view ([afef868](https://github.com/phamtanminhtien/patchpilot/commit/afef86831dd7bbb5468722873b443a40283eea2e))
* implement modular workspace layout with tabbed panels and integrated git change management ([4ecf710](https://github.com/phamtanminhtien/patchpilot/commit/4ecf7106a913e33cacb73b2178ef223a2891d181))
* implement path aliasing for frontend imports and enhance button variant functionality ([d90ddec](https://github.com/phamtanminhtien/patchpilot/commit/d90ddec1b0f2547b82ee232a5432b53eb2a0a6b4))
* implement PreviewPanel port management UI, integrate E2E testing, and update vibe page sidebar component ([071fbec](https://github.com/phamtanminhtien/patchpilot/commit/071fbec6b34a0ad47ff51b50379cc749fe99a6d5))
* implement process management, execution safety, and real-time workspace event streaming ([796961f](https://github.com/phamtanminhtien/patchpilot/commit/796961fdab5669e7382d277c66305dd6ab49f2d7))
* implement release automation with Release Please and add Playwright E2E tests to CI pipeline ([869eeaa](https://github.com/phamtanminhtien/patchpilot/commit/869eeaa7470e33954800e869b0b720c5dea8819c))
* implement workspace management features including listing workspaces, enhancing UI with AppShell, and adding theme preference functionality ([8de2816](https://github.com/phamtanminhtien/patchpilot/commit/8de2816604728ac9b4043218306a464f23002c3d))
* initialize PatchPilot with core functionality including workspace management, file handling, and Git integration ([1f6b056](https://github.com/phamtanminhtien/patchpilot/commit/1f6b056b4d129a0acb327cceac251f0ebf33d123))
* set web/dist as the default static directory and simplify server initialization ([6390934](https://github.com/phamtanminhtien/patchpilot/commit/639093443243e938fa1903ff070f69d454fc42a2))
* **workspace:** add file and git workspace panels ([a2d889f](https://github.com/phamtanminhtien/patchpilot/commit/a2d889fee33e770e0d7a5f9d26489f50e30c79b0))


### Bug Fixes

* add task status lifecycle management, improve UI layout containment, and include error details in API responses ([3f2f1eb](https://github.com/phamtanminhtien/patchpilot/commit/3f2f1eb7c7df4a0c9af20a4625da8c9c5e9feaa2))
* update Button component to prevent icon overflow and enhance accessibility with aria attributes ([5fd8254](https://github.com/phamtanminhtien/patchpilot/commit/5fd8254d455187f20e6920e5436edce7bf577244))
* use web commitlint in git hook ([274a8be](https://github.com/phamtanminhtien/patchpilot/commit/274a8be24197832a469305be7286689908a400f6))
* **workspace:** adjust layout and overflow properties for improved UI consistency in WorkspacePage and FilesPanel ([ae6e5c8](https://github.com/phamtanminhtien/patchpilot/commit/ae6e5c8674d1ee5f69881198e04f2c4b63468ff7))
