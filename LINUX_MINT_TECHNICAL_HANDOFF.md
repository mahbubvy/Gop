# gotohp — Linux Mint technical handoff

Updated: 2026-09-11. Current application version: **1.2.1** (custom local build).

This document supersedes `LINUX_BUILD_HANDOFF.md`. Use the current modified source snapshot, not upstream alone. This handoff does not claim that the latest build has run successfully on Linux Mint.

## Latest source update — phase 4, not yet packaged

The source now includes additional edge-case fixes beyond the existing `gotohp-v1.2.1.exe`. Version metadata is still 1.2.1 until the next packaging step; do not assume that executable includes these changes.

- Backend `FileUploadResult.Cancelled` identifies errors that satisfy `errors.Is(err, context.Canceled)`. The frontend records these separately in `results.cancelled`, not `results.fail`; unrelated errors remain failures. Transfer/authentication algorithms are unchanged.
- Completion shows Cancelled and No final result counts. The latter is `max(0, Total - processed)` and does not assert that those items never began. Debug export is now schema **3**, with cancelled file basenames and the same counts.
- `ConfigManager.CountUploadCandidates(paths)` uses the existing local scanner. Selection blocks empty/unsupported-only inputs before changing backend upload settings. Auto Album also rejects an empty classified folder preview. Existing filtering/recursion settings apply.
- `uploadSelection.starting` blocks duplicate starts between event dispatch and backend uploadStart. A reactive watch clears it once backend uploading state arrives; dispatch failure releases it. Home displays Starting upload during this interval.
- Local validation adds a scan before upload preflight; large source trees may take longer to prepare, and this preparatory scan does not currently expose cancellation. Do not describe it as instantaneous or cancelable.
- Regenerate Wails bindings for the added method/model field. New tests include `frontend/tests/uploadResults.test.mjs` and backend empty/missing-candidate checks. All 27 frontend tests passed together; Go CLI-tag tests passed.

New user evidence from `log/3 auto album.json` (Windows 1.2.1): 30 uploaded, 0 failed, three completed albums with 8/13/9 additions. `log/upload cancel.json`: cancel requested, 8 cancellation-category errors and 2 items without a final result, 0 successful uploads. This supports working stop behavior but does not verify preservation of earlier successes in that particular batch. These user logs are private, not release contents.

Older sections below describe the packaged 1.2.1 baseline; this update takes precedence for the current source's cancellation classification, validation and schema version. Native tests of these new source changes remain pending.

## 1. Objective and boundaries

Continue development or produce a Linux Mint GUI `.deb` containing the current UI changes. The user runs Mint on another PC and does not want to sign into Codex there; the packaged app needs no Codex login.

- Source workspace at handoff: `K:\gotohp-main` on Windows. Upstream reference: https://github.com/xob0t/gotohp.
- This directory is not a Git checkout. Preserve the entire modified source and compare against a private baseline before changes.
- **Preserve the upload engine, auth/device identity, transport defaults, and the behavior the user describes as unlimited/non-storage-counting Google Photos uploads.** Do not swap APIs or authentication flows while porting the UI. Google quota treatment has not been independently verified or guaranteed.
- Remote fetching of all account albums was explicitly shelved. Recent Album is a local per-account shortcut list, not a Google album browser.
- Do not introduce VPN/network changes, dependencies upgrades, WSL/Docker installation, publication, or credential migration just to package this snapshot without appropriate authorization.
- Preserve existing app data and executables. Never include credentials, user config, databases, or exported user logs in a release/source bundle for public distribution.
- Repository PR guidance: focused changes, Conventional Commits titles, lowercase description. No PR/push is currently requested.

## 2. Current status and version history

| Version | Change | Verification limit |
| --- | --- | --- |
| 1.1.0 | Upload flow phases 1–3 | User tested primary flows on Windows |
| 1.1.1 | Restored all four drag destinations, including Recent Album | Built for Windows; dedicated retest not recorded |
| 1.2.0 | Phase 5 history/log/accessibility changes | **Known broken startup: blank window. Do not distribute** |
| 1.2.1 | Fixed initialization-order crash | Startup regression test and production builds pass; user confirmation of native launch not yet recorded |

Latest Windows artifact: `gotohp-v1.2.1.exe`, title `gotohp v1.2.1 — Startup fix`.
No current Linux `.deb` has been built or verified in this task.

Phases 1–3 and phase 5 are implemented. Phase 4 was not completed as a dedicated pass. Phase 6 native/platform acceptance testing remains pending.

## 3. Product behavior to preserve

### Home and source selection

- Connected-account selector; three radio modes: Upload Only, New album, Auto Album; separate Recent Album entry.
- Choose files allows multiple files. Choose folder currently selects one directory; drag can supply multiple folders.
- Regular mode starts after selection and explicitly targets `Google Photos library · No album`.
- New album asks for a nonblank name after file selection. Enter confirms. Existing `AF1Qip...` keys remain accepted for compatibility.
- A typed title creates a **new** album even if the title already exists. It is not a title-based lookup. Recent Album reuses the stored album key.
- Auto Album scans and previews actual containing-folder names before confirmation. It follows current recursion/filter/Live Photo classification settings. A new album is created only for folders with eligible results. Identically named folders can create distinct identically titled albums.
- Settings controls recursive scanning; UI must not implement separate recursion semantics.
- Album History is directly below Settings on Home, not inside Settings.

### Drag and drop — regression-sensitive

- While dragging over Home, show four destinations: Upload Only, New album, Auto Album, Recent Album.
- Explicit `data-drop-zone` wins over the selected radio mode. An unrecognized/missing zone falls back to the selected mode.
- Drop on Recent Album -> retain dropped paths -> choose a saved album -> start upload.
- On the Recent Album screen, dropping after selecting an album targets it; dropping before selection stores sources until an album is selected.
- A prior redesign replaced the four destinations with one selected-mode area, breaking the user's preferred workflow. **Do not reintroduce that behavior.**
- Mint originally failed to show the hover overlay and started uploading immediately. Native GTK/WebKit drag events were not reproduced here; persistent picker controls are the fallback.

### Progress and completion

- Current account, photo/video/file wording, processed count, byte progress, speed, current workers, Cancel.
- Upload Only has no album-progress section.
- Auto mode shows every source folder with processed/total work counts, distinguished by full path when basenames collide.
- Folder progress means media processing, **not successful album assignment**. Files upload first, album operations follow.
- Recent Album uses the readable saved name through progress and results.
- Only backend `uploadStop` ends a batch. Do not end at the last FileStatus; album assignment may still be running.
- Persistent completion page until Go back home, with success/failure/skipped counts, timestamp, and appropriate cancellation/empty/error heading.
- Albums updated contains confirmed additions, including partial additions before album failure/cancellation. Split albums have separate counts rather than the cumulative count repeated for each.
- On assignment failure, explain that successfully uploaded files are already in the library. Do not imply the media transfer failed or that no album was selected.

## 4. Code map and contracts

| Path | Responsibility / caution |
| --- | --- |
| `main.go` | Wails GUI, event wiring, optional `buildLabel`, resizable 400×600 to 800×900 window; inspect native Mint decoration/content dimensions |
| `cli_shared.go`, `backend/version.go` | Version read from embedded `build/windows/info.json`, even on Linux |
| `frontend/src/App.vue` | Accounts, source/destination screens, drag routing, Settings/History, debug export, screen focus |
| `frontend/src/utils/uploadSelection.ts` | Dependency-injected picker/drop controller; captures account/mode, configures backend then emits start; optional Auto Album review enabled by App |
| `backend/upload.go` | Existing scanner/workers/album orchestration; added read-only `ConfigManager.PreviewAutoAlbums` and `UploadBatchStart.WorkPaths` for presentation |
| `frontend/src/utils/UploadManager.ts` | Reactive upload state, Wails event listeners, byte tracking, friendly names, history/recents, cancellation request and diagnostic failure categories |
| `frontend/src/utils/uploadPresentation.ts` | Media labels, result headings, per-folder counts, cumulative-to-per-album progress conversion |
| `frontend/src/Upload.vue` | Active transfer, folder and assignment progress |
| `frontend/src/UploadComplete.vue` | Persistent result screen and issue details |
| `frontend/src/AlbumHistory.vue` | Account context, full names, operation/count/time, loading/empty/error/retry states |
| `frontend/src/utils/debugLog.ts` | Filename redaction and inferred error categories; deliberately avoids exporting raw server errors |
| `frontend/src/components/DebugLogButton.vue` | Shared floating export button |
| `frontend/src/components/GoogleAuthSetup.vue` | Existing auth UI; added slot/spacer so log button is inside modal focus boundary; no auth algorithm changes |
| `frontend/src/index.css` | Theme, visible keyboard focus, layout minimums |
| `backend/configmanager.go` | Persistent per-account recents/history, diagnostic file export |
| `backend/album.go` | Existing key-vs-title behavior, batching and album splitting; do not replace API calls |
| `frontend/bindings/` | Generated Wails TypeScript; regenerate, do not hand-edit |

Selection order: await `SetSelected(account)` -> `SetAlbumName(nameOrKey or empty)` -> `SetAlbumAutoMode(boolean)` -> `Events.Emit('startUpload', {files})`.

Backend starts with a preflight `uploadStart` and emits another after classification. `UploadBatchStart` fields include `Total`, `TotalBytes`, `AlbumName`, `AlbumAutoMode`, `AutoAlbumFolders`, `WorkPaths`. WorkPaths contains primary work-item paths; Live Photos can represent paired sources, so counts are work items rather than necessarily physical filesystem files.

Other events: `uploadTotalBytes`, `uploadTotalBytesDelta`, `uploadWarning`, `ThreadStatus`, `FileStatus`, `albumProgress`, `albumComplete`, `albumError`, `uploadStop`. UI emits `uploadCancel`.

The pending readable Recent Album name must survive both startup events. It is assigned at actual dispatch and cleared on stop. Do not let a cancelled picker leak a prior name into another mode.

`recordAlbumProgress` tracks by album key and subtracts earlier split albums from cumulative ItemsAdded. Completion marks existing rows instead of appending duplicates.

## 5. Persistence, history and logs

- `RecentAlbumsByAccount`: max 8 per account, `albumId`, `albumName`, `lastUsedAt` (Unix seconds). Successful automatic-folder albums are included.
- `AlbumHistoryByAccount`: max 40 per account, `albumId`, `albumName`, `itemsAdded`, `operation`, `createdAt` (Unix seconds).
- Raw keys display as Saved album. Legacy entries lacking a readable name cannot magically recover it; no remote lookup exists.
- History and recent writes currently originate from frontend album-completion events. Partial album additions appear in results, but partial/error paths do not currently guarantee a persisted history entry. Review this before claiming complete audit coverage.
- Normal Linux configuration is under the OS config directory, usually `~/.config/gotohp/gotohp.config` (XDG applies). An executable-adjacent `gotohp.config` takes precedence. Check `backend/configmanager.go` for exact resolution; never log credential contents.
- Config writes use existing persistence; do not reset data for a UI upgrade.

Debug export schema v2 snapshots state **before** opening the save dialog. It includes application version, screen, viewport dimensions, selection mode, cancellation request flag, timestamps, byte/work counts, per-folder counts, per-album additions, worker status/attempt, and inferred error categories.

It excludes account addresses, full local paths, album keys and raw server errors. File/album names remain potentially personal. Categories include cancelled, timeout, authentication-or-permission, not-found, rate-limited, network, file-or-metadata, server-error, other-error; these are inferred summaries, not exact backend codes. This is a state snapshot, not a full chronological network trace.

`cancelRequested` is a frontend request flag, not a separately acknowledged backend cancellation result. Cancellation may still appear in failed-file counts. Do not claim cancelled files are separately classified today.

The floating button is instantiated inside Settings/account setup dialogs to respect modal focus traps; the outside button is hidden while those dialogs are open. Keep bottom spacing so it does not obscure controls.

## 6. Critical startup regression (fixed in 1.2.1)

Phase 5 initially registered `watch(currentScreen, ...)` before initializing `showAlbumInput`. Vue watch evaluates the source immediately to collect dependencies, even without `immediate: true`. Home evaluation threw:

```text
ReferenceError: Cannot access 'showAlbumInput' before initialization
```

This produced a completely blank native window despite successful TypeScript/Vite compilation. The fix registers the watcher after all selection-derived refs/computeds exist.

`frontend/tests/appStartup.test.mjs` evaluates the actual reactive setup slice with Vue and stubbed selection to catch this failure. It is **not** a full mounted DOM/native test. Keep it, but also launch the actual Mint GUI and confirm Home renders before shipping.

## 7. Build environment and Linux metadata mismatch

Pinned/current source:

- Go directive: 1.26; Windows builds used portable Go 1.26.8.
- Node: `.node-version` specifies 24; use a compatible Node 24 build with type stripping for tests.
- Wails Go module and frontend runtime: v3.0.0-beta.12 / 3.0.0-beta.12.
- Vue 3, Vite 8, TypeScript 6, Tailwind 4; use package-lock, do not silently upgrade.
- Linux GUI requires CGO and native libraries. Windows `CGO_ENABLED=0` build instructions do **not** apply to Linux GUI.
- Repository default GTK stack: GTK4/WebKitGTK 6.0. Build dependency names recorded by this repo: `build-essential`, `gcc`, `pkg-config`, `libgtk-4-dev`, `libwebkitgtk-6.0-dev`. Default runtime dependencies: `libgtk-4-1`, `libwebkitgtk-6.0-4`.
- `nfpm.yaml` documents a `gtk3` legacy option with GTK3/WebKit2GTK 4.1. Verify availability against the pinned Wails code and actual target. Change build tags and package dependencies together if needed.

**Unknown:** target Mint release, architecture, desktop/session type. Determine these before picking a builder. Build against libraries/glibc compatible with the target; do not assume a newer builder binary runs on an older Mint installation. WebView2 is Windows-only and is not the Mint solution.

**Metadata mismatch:** `build/windows/info.json` is 1.2.1, but `build/linux/nfpm/nfpm.yaml` still has `version: "0.10.0"`, release 1. Align the GUI package version before producing a `.deb`. The frontend also imports info.json for log version; update metadata **before rebuilding frontend**. CLI packaging has separate config; inspect it if building CLI.

`main.buildLabel` adds a human-readable title suffix. The standard Linux task does not currently pass a custom label. Either deliberately add it to Linux build flags or accept a plain `gotohp v1.2.1` title; do not call it an upstream release.

## 8. Transfer and native build procedure

Transfer the modified source privately. Include Go source/modules, frontend source/package-lock/tests/bindings, build scripts/assets, generated source, and this handoff. Exclude `.local-tools`, `frontend/node_modules`, Windows `.exe`/`.syso`, caches, user config/databases and user debug logs. Reinstall platform-specific dependencies on Linux. Existing `frontend/dist` is generated: rebuild it.

The following commands are derived from the repository; **not Linux-executed in this session**. Run only on an authorized Linux builder with the correct prerequisites and architecture:

```sh
# First inspect the target/builder, no credential data needed.
cat /etc/os-release
uname -m
dpkg --print-architecture
go version
node --version
pkg-config --modversion gtk4 webkitgtk-6.0

go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.12
export PATH="$(go env GOPATH)/bin:$PATH"
wails3 doctor

cd frontend
npm ci
node --experimental-strip-types --test tests/*.test.mjs
npm run build
npm run lint
cd ..

go test -mod=readonly -tags cli ./...
# After aligning nfpm version/maintainer and selecting correct architecture:
wails3 task linux:create:deb
```

Set `GOARCH` to the actual Go architecture and real maintainer variables required by `nfpm.yaml`; do not fabricate user identity. For an amd64 target it is `amd64`, not `x86_64`. Inspect `wails3 task --help` for task variable passing and set `ARCH` where appropriate.

The packaging task builds GUI, generates desktop entry, then calls `wails3 tool package`. Do not invoke the general Linux package task unless AppImage/RPM/AUR are also needed.

Build task side effects: runs `go mod tidy`, npm install, binding generation, icon generation. Inspect changes to lockfiles/generated assets. Preserve the user icon (`build/google-photos-icon.svg`, `build/appicon.png`, `build/windows/icon.ico`). Never use `.local-tools/home-ui-preview` mock runtime/config for a release.

Manual native diagnostic build alternative after prerequisites/frontend:

```sh
wails3 generate bindings -f '-tags production' -clean -ts
cd frontend
npm run build
cd ..
mkdir -p bin
CGO_ENABLED=1 go build -mod=readonly -tags production -trimpath -buildvcs=false \
  -ldflags '-w -s -X "main.buildLabel=Mint test - history and logs"' \
  -o bin/gotohp .
./bin/gotohp version
./bin/gotohp
```

If building gtk3, use consistent tags during both binding generation and Go build. The manual build alone is not a `.deb`; the default packaging task can rebuild and replace its binary/title flags.

## 9. Installation, replacement and rollback

Current GUI package identity: `gotohp`; executable `/usr/local/bin/gotohp`; desktop `/usr/share/applications/gotohp.desktop`; icon `/usr/share/icons/hicolor/128x128/apps/gotohp.png`.

Inspect the existing Mint package (`dpkg-query -W gotohp`) and new package metadata before installation. Same package name/architecture normally upgrades/replaces the old app; verify version ordering. Different names do not guarantee side-by-side installation if file paths collide.

Inspect with `dpkg-deb --info ACTUAL.deb`, `dpkg-deb --contents ACTUAL.deb`, and `sha256sum ACTUAL.deb`. Replace ACTUAL with the produced filename. Confirm dependencies, architecture, version, paths and maintainer scripts. Keep the old `.deb` and a private config backup. Install via `sudo apt install ./ACTUAL.deb` on the authorized target. Do not purge or delete settings. Provide actual package filename and checksum in delivery.

## 10. Test evidence and remaining acceptance work

Verified during Windows development:

- Frontend typecheck/production build; latest 1.2.1 build passed.
- 21 selection/presentation/debug tests passed at phase 5; startup regression test added and passed in 1.2.1 (22 tests across the current four test files when run together).
- ESLint quiet check passed before the startup fix; existing style warnings are not a clean all-warnings result.
- Go CLI-tag tests passed after preview changes, including `backend/upload_preview_test.go`.
- Windows artifacts compiled and reported their version. Native Mint rendering, GTK dialogs and `.deb` upgrade remain unverified.

User-provided Windows logs:

- New album: 5 uploads, 5 album additions, readable title.
- Recent album via picker: 5 uploads/additions, readable title retained.
- Auto Album log: 8 uploads into **one** album; does not demonstrate three separate folders.
- Cancel log: 0 successes, 5 failures; old schema lacked cancellation/errors. New schema needs a fresh test.
- User reported the main flows successful, then identified drag regression and later the 1.2.0 blank window. Distinguish those reports from independently verified native testing.

Mint acceptance checklist:

1. Fresh launch renders Home; no blank screen. Restart and verify version/account persistence.
2. Native file picker supports multiple files; folder picker cancellation causes no upload/config reset.
3. Upload Only photo/video/mixed batches show no album section; completion stays open.
4. New title creates an album with correct count. Repeated title makes another album; Recent Album reuses one.
5. Three folders -> preview three destinations -> progress three rows -> results correct additions; test recursive nested folders and equal basenames.
6. Drag onto each destination, especially Recent Album. Also test a zone-less Linux drop and direct drops on the recent screen.
7. Cancel media transfer and album assignment separately; export schema v2 logs and compare confirmed files/albums in Google Photos. Avoid blindly retrying files already uploaded.
8. Account switching does not expose another account's recent/history entries.
9. History loading/empty/error/full states, long names, local timestamps, Created/Updated and count accuracy.
10. Resize between compact and maximum dimensions; check scrolling, no hidden controls/overlap, Tab/Shift+Tab/Enter, modal focus and export button in every screen.
11. Logs identify settings/account/history/review/upload/result screens and the correct version. Inspect privately for accidental sensitive content before sharing.
12. Install over the old package without losing settings; verify rollback with the retained package.

Remaining gaps: native visual/accessibility validation, dedicated phase-4 empty/unsupported/duplicate-submission testing, cancellation vs failure classification, persistence of partial album updates, and old Saved album entries without a known name. Resolve within explicit scope; do not silently expand into remote album fetching or transport changes.

## 11. Earlier transport fix and speed observations

`backend/httpclient.go` initializes nil TLSClientConfig after cloning the default transport. Preserve the regression fix/tests. It prevented a panic during an HTTP/1 diagnostic (`GODEBUG=http2client=0`); it does not force HTTP/1 or disable HTTP/2. Never ship those diagnostic environment overrides by default.

User observed roughly 1–2 MB/s on Windows and full speed on another Mint computer using the same network. WSA looked faster with a different account. Cause remains unresolved; these observations do not prove a Windows transport defect. Packaging/UI work should not alter this subsystem.

## 12. Expected next-agent deliverable

Use this snapshot, determine target compatibility, fix only required Linux integration issues, launch/test the real GUI, and deliver a clearly versioned `.deb`, SHA-256, install/rollback steps and honest test results. Record any unverified behavior. A successful compile is not proof of a working rendered UI.
