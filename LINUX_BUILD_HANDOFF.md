# gotohp: home upload controls and Linux build handoff

> Superseded on 2026-09-11 by [LINUX_MINT_TECHNICAL_HANDOFF.md](LINUX_MINT_TECHNICAL_HANDOFF.md). The older behavior, versions, artifacts and test claims below are historical; use the new document for current work.

Prepared 2026-09-04. This is a handoff, not a claim that a Linux package has been built.

## Next objective

Build a Linux GUI `.deb` containing the current local changes so the user can transfer it to a separate Linux Mint PC and test it. The user does **not** want to sign into Codex on the Mint PC. No Codex login is required to install or run a built package there.

Current workspace: `K:\gotohp-main` on Windows. Upstream: `https://github.com/xob0t/gotohp`.
This directory is a source snapshot, **not a Git checkout** (`git status` fails). Use this modified snapshot, not a fresh upstream checkout alone. There may be other pre-existing local changes; do not overwrite them or assume every difference from upstream belongs to this task.

## User constraints and authorization

- Preserve the existing upload engine, authentication/device identity, and the behavior the user calls unlimited/non-storage-counting uploads. This UI change does not verify or guarantee Google's quota treatment; it leaves that implementation untouched.
- Do not replace the upload API with another Google Photos API, change account handling, or tune the transport as part of packaging.
- Fetching/listing existing Google Photos albums was explicitly shelved. Do not implement it.
- User authorized home-screen controls and requested a Linux package. WSL/Docker installation, Windows feature changes, reboots, external publication, and GitHub pushes/workflow dispatches have **not** been authorized.
- No further network/VPN changes. Windows upload-speed diagnosis is outside the current task.
- Preserve existing executables and user data. Do not bundle credentials, account databases, or `gotohp.config`.
- Repository `AGENTS.md`: focused PRs only; Conventional Commits titles with lowercase description if a PR is eventually requested.

## Why the UI was changed

On Linux Mint, the user dragged a folder and uploading started immediately without showing the three hover-only drop options visible on Windows. Inspection found that all platforms share `frontend/src/App.vue`; its overlay relied on browser drag events and an unspecified drop zone fell back to regular upload. The precise native Mint drag behavior was not reproduced here.

The user requested persistent home controls so dragging is no longer necessary.

## Implemented changes

### `frontend/src/App.vue`

- Home now displays three radio-card choices: **Upload Only**, **Upload to Album**, **Auto Album**.
- Added **Choose files** and **Choose folder** buttons using the pinned Wails runtime's `Dialogs.OpenFile` API.
- File picker allows multiple files. Folder picker selects one directory.
- Upload Only and Auto Album start after accepting the picker. Upload to Album shows the existing album-name/key confirmation screen first.
- Album confirmation rejects blank names and supports Enter. Buttons are disabled while preparing a selection.
- Added busy-state guards for account/settings controls and relevant selection actions.
- Existing drag-and-drop zones remain. Drops without a recognized zone now follow the home-screen selection; explicit drop zones override it.
- The home layout has compact spacing and vertical scrolling to accommodate the existing 400-by-600 desktop window.
- Added explanatory text: recursive folder scanning is still controlled by **Settings → Recursive Directory Upload**.
- Cleans up the `files-dropped` subscription on unmount.

### `frontend/src/utils/uploadSelection.ts` — new

Shared, dependency-injected selection controller used by both buttons and drag-and-drop:

1. Capture account and mode before awaiting a native picker.
2. Treat empty string, empty array, or null picker results as cancellation.
3. For named-album mode, retain selected paths/account until confirmation.
4. Await `SetSelected`, `SetAlbumName`, and `SetAlbumAutoMode`, in that order, before emitting the existing `startUpload` event with `{ files: paths }`.
5. Clear stale album settings when starting regular/automatic uploads.
6. Prevent overlapping selections; surface errors and retain pending album selection on failure for retry.

Directories are passed intact to the existing backend scanner. Recursion, file filtering, Live Photos, duplicate handling, upload concurrency, and quota-related behavior are not reimplemented here. Auto Album continues using each file's parent-folder name through the existing backend.

### Other current-task files

- `frontend/src/components/GoogleAccountSelect.vue`: added a `disabled` prop forwarded to the select component.
- `frontend/tests/uploadSelection.test.mjs`: 14 selection-flow tests, using Node's built-in test runner and the actual controller.
- `README.md`: documented home-screen upload modes and file/folder selection.

### Album-status correction added after the initial handoff

- `frontend/src/utils/UploadManager.ts`: clears the previous batch's album status and last-album label on every `uploadStart`. This prevents the upload screen from showing an old album while a new batch is running. On `albumComplete`, it records the completed album name for this batch only.
- `frontend/src/Upload.vue`: changes the completed in-progress label to **Album updated**. This avoids claiming a new album was created when the user selected an existing album key/name.
- `frontend/src/App.vue`: the post-upload results card shows a single `Last album: <name>` entry when this batch completed an album operation. Regular uploads show no album entry.

This correction is display-state only. It does not alter the backend upload sequence or Google Photos behavior.

### Windows test-build identification and icon

- `build/google-photos-icon.svg` is the user-supplied Google Photos icon source.
- `build/appicon.png` and `build/windows/icon.ico` were regenerated from it for the Windows resource.
- `main.go` now accepts an optional linker-set `buildLabel`. Normal builds remain `gotohp v<version>`; the local test executable was built with `home-upload-test`, so its title is `gotohp v0.10.0 — home-upload-test`.
- Current Windows artifact: `gotohp-home-upload-test.exe`. It is a local test build, not a signed upstream release.

No backend upload/authentication changes were made for this home-screen feature. No new production npm dependencies were added; the existing lockfile was used.

## Earlier authorized fix already present — preserve it

These changes predate the home-screen work:

- `backend/httpclient.go`: after cloning `http.DefaultTransport`, initialize `transport.TLSClientConfig = &tls.Config{}` if nil before accessing `InsecureSkipVerify`.
- `backend/httpclient_test.go`: regression coverage for nil/existing TLS configuration, proxy/direct clients, and local HTTP/1.1 and HTTP/2 negotiation.

Reason: the CLI crashed in `NewHTTPClientWithProxy` during a controlled `GODEBUG=http2client=0` test because `TLSClientConfig` was nil. The fix does **not** force HTTP/1.1 or disable HTTP/2. Existing proxy certificate-validation behavior was not changed. Do not bake diagnostic environment overrides into the Linux package.

Windows throughput remains unresolved. The user reported roughly 1–2 MB/s on Windows, versus full speed on a different Linux Mint PC on the same network. WSA also appeared fast but used a different account. These observations do not establish a specific transport cause.

## Verification completed

- `npm run build`: passed (Vue type checking and production Vite build).
- `npm run lint`: passed without warnings after formatting.
- From `frontend`: `node --experimental-strip-types --test tests/uploadSelection.test.mjs`: **14/14 passed**, using Node 24.14.1.
- `go test -mod=readonly -tags cli ./...`: passed on Windows using Go 1.26.8.
- Separate Windows GUI executable compiled successfully.
- Browser-based UI check in an isolated mocked preview at 400-by-570 content size: all home controls fit; all three modes sent the expected configuration and paths; named album selection waited for confirmation. **No actual Google uploads were performed.**

Not verified: native Linux file/folder dialogs, Linux compilation, `.deb` installation/upgrading, or a real upload using this new GUI build.

Artifacts already present:

- `K:\gotohp-main\gotohp-home-test.exe`: Windows GUI test build with home controls; not usable on Linux.
- `K:\gotohp-main\gotohp-cli-http-test.exe`: earlier transport-test CLI build.
- `K:\gotohp-main\gotohp-cli-x64.exe`: pre-existing CLI executable, left untouched by this work.
- `frontend/dist`: generated production frontend; main GUI embeds this directory.

Temporary tooling, not release contents:

- `.local-tools/go-transport-test`: portable Windows Go 1.26.8 plus module/build caches.
- `.local-tools/home-ui-preview`: alternate Vite configuration and mocked Wails runtime for UI verification. Preview server was stopped. **Never use this mock configuration for a release build.**

`npm ci` reported one high-severity dependency audit finding in the locked dependency tree. It was not investigated or automatically fixed as part of this narrow feature; do not silently upgrade dependencies while packaging.

## Linux build blocker and prerequisites

No Linux `.deb` has been produced. Docker was not found, and `wsl.exe` did not expose a usable installed Linux environment. Nothing was installed to enable WSL/Docker. User asked about disk space and was given a rough planning estimate of 20–30 GB free for WSL, Ubuntu, dependencies, and caches; this was not approval to install it.

Before choosing a build environment, obtain the target Mint version and CPU architecture. These remain unknown; do not assume the latest Mint or blindly produce an ARM package.

Project versions/configuration:

- Go: `go.mod` requires Go 1.26; previous local builds used 1.26.8.
- Node: `frontend/.node-version` says 24.
- Wails Go and frontend runtime: `v3.0.0-beta.12` / `3.0.0-beta.12`; keep them aligned.
- Default Linux GUI needs CGO, GTK4, and WebKitGTK 6.0.
- Existing workflow installs `build-essential gcc pkg-config libgtk-4-dev libwebkitgtk-6.0-dev` on its Linux builder.
- Default `.deb` runtime dependencies: `libgtk-4-1`, `libwebkitgtk-6.0-4`.
- The packaging file documents a legacy `gtk3` build option with matching GTK3/WebKit2GTK 4.1 dependencies. Choose it only if the target needs it, and match compiler tags and package dependencies consistently.

Relevant build sources:

- `Taskfile.yml`
- `build/Taskfile.yml`
- `build/linux/Taskfile.yml`
- `build/linux/nfpm/nfpm.yaml`
- `build/config.yml`
- `.github/workflows/build.yml` (reference only; do not publish/dispatch without permission)

## Suggested continuation on an authorized Linux-capable builder

1. Transfer the modified source snapshot privately. Exclude `.local-tools`, `frontend/node_modules`, Windows executables, generated caches, and all credentials/config/database files. Reinstall dependencies for Linux. Keep all actual application source, bindings, build files, assets, and lockfiles.
2. Use a builder compatible with the target Mint release; don't unknowingly link against a newer glibc/WebKit than the target provides.
3. Install the required toolchains/libraries only within the authorized build environment. Pin Wails to `v3.0.0-beta.12`.
4. Run frontend tests/build/lint and Go tests. Build the **GUI**, not the `cli` or `server` target.
5. Use the focused `.deb` task, rather than building all distribution formats unnecessarily.

The following are command outlines derived from the repository, **not Linux-tested commands from this session**. Run from the transferred project root after prerequisites are available:

```sh
# Use Node 24 and Go 1.26.x.
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.12
# Ensure the Go binary directory containing wails3 is on PATH.

cd frontend
npm ci
npm run build
npm run lint
node --experimental-strip-types --test tests/uploadSelection.test.mjs
cd ..

go test -mod=readonly -tags cli ./...
wails3 doctor
wails3 task linux:create:deb
```

`linux:create:deb` depends on the GUI build and desktop-file generation, then calls `wails3 tool package` with the nfpm configuration. Verify its packaging-tool requirements before running. The repository CI also installs `nfpm`; use a verified compatible version if needed.

Important: existing task dependencies run `go mod tidy`, frontend dependency installation, and binding generation. Preserve a baseline and inspect resulting changes. Do not accept unrelated dependency/binding changes without understanding them. Avoid wholesale build-asset regeneration that could overwrite target-specific packaging adjustments.

Set the correct `GOARCH` and valid maintainer metadata for nfpm; do not invent the user's name/email. Package version/release metadata should clearly distinguish this local test build without claiming an upstream release. Current metadata is `0.10.0` with release `1`.

## Package identity, replacement, and user data

Current `build/linux/nfpm/nfpm.yaml` specifies:

- Package name: `gotohp`.
- Executable: `/usr/local/bin/gotohp`.
- Desktop launcher: `/usr/share/applications/gotohp.desktop`.
- Icon: `/usr/share/icons/hicolor/128x128/apps/gotohp.png`.
- Post-install script currently contains only its Bash shebang; it has no data migration/deletion commands.

Installing a new package with the same package identity/architecture normally replaces or reinstalls the existing package rather than creating a second app. Verify the actual installed package name/version on Mint; it was not inspected here. An identical version may require explicit reinstallation depending on the installer. A different package name alone is **not** sufficient for side-by-side installation if file paths still conflict.

Application settings/accounts are normally stored under the OS user config directory in `gotohp/gotohp.config` (typically `~/.config/gotohp/gotohp.config` on Linux, respecting XDG configuration). A `gotohp.config` beside the executable takes precedence. Preserve this behavior and advise backing up the actual config before testing. Do not read/share credential contents or include them in the `.deb`.

For rollback, keep the previous `.deb` and configuration backup. Do not tell the user to purge the old package or delete settings before installing.

## Final verification and deliverables

- Inspect the `.deb` control metadata, architecture, dependency list, file contents, permissions, and maintainer scripts (`dpkg-deb --info` / `--contents`).
- Verify the embedded frontend is the real production build, not stale assets or preview mocks.
- Produce the `.deb`, its SHA-256 checksum, and concise transfer/install/rollback instructions with the actual filename.
- On Mint, test native multiple-file selection, folder selection, picker cancellation, album confirmation/cancellation, all three modes, empty/unsupported folders, recursive setting off/on, and drag-and-drop fallback.
- Have the user test a small authorized upload and confirm existing account/settings behavior. Don't initiate bulk uploads or claim unlimited quota behavior is proven by unit tests.
- Clearly state any checks that could not be performed. No Codex login on the target Mint PC is needed.
