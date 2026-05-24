# Release Checklist

PatchPilot releases are created through Release Please on `main`. This checklist
keeps the manual verification around that automation explicit.

## Before merging the release PR

- Confirm `CHANGELOG.md` describes the intended release.
- Confirm `.release-please-manifest.json` contains the intended version.
- Run the release verification commands:

```sh
go test ./...
pnpm --dir web test
pnpm --dir web lint
pnpm --dir web build
```

## After merging the release PR

- Confirm the GitHub Release and tag were created with the expected
  `patchpilot-v<version>` tag.
- Confirm the Release Please workflow published GHCR image tags:

```txt
ghcr.io/phamtanminhtien/patchpilot:patchpilot-v<version>
ghcr.io/phamtanminhtien/patchpilot:<version>
ghcr.io/phamtanminhtien/patchpilot:latest
```

- Confirm each GHCR image tag includes both supported Linux platforms:

```txt
linux/amd64
linux/arm64
```

- Prefer a version tag instead of `latest` for reproducible Docker runs.
