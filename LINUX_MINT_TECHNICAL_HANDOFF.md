# gotohp — Linux Mint technical handoff

Updated: 2026-10-07. Current source metadata version: **1.3.3** (local Windows builds; Linux packaging and native Mint acceptance remain pending).

This document supersedes `LINUX_BUILD_HANDOFF.md`. Use the current modified source snapshot, not upstream alone. This handoff does not claim that the latest build has run successfully on Linux Mint.

## Current source and release baseline — read first

- Private repository: https://github.com/mahbubvy/Gop (renamed from Gophoto-UI).
- Last identifiable published baseline remains `v1.3.1`; source commit `262a65f34f0c0b31ea4dff2de950e02e1c40d542`. The current workspace contains additional local changes after that tag; do not assume they are published.
- Current Windows test artifact: `bin/gotohp-v1.3.3-saved-album-sort-export.exe`, title suffix `Saved Albums Sort and Export`.
- Current artifact SHA256: `ADBE9224D94417FAF00A4326548A6420C673CCA786F46AAE2598FFB572FF9160`.
- This is a local labeled build with 1.3.3 file metadata, not a confirmed GitHub release.
- Main workspace is still not a Git checkout. `.publish-gophoto/` is a separate
  publishing clone, not the development source. Do not copy it into a Linux bundle.
- Obtain the private tagged source through an authorized checkout or transfer
  the current source folder. Do not substitute upstream: it lacks these changes.
- Do not push to GitHub or publish a release unless explicitly requested.
  Upstream build/release workflows now require manual dispatch.
- The original UI edge-case changes and all five local-record implementation/
  packaging phases are included in Windows releases. Native Linux acceptance
  remains outstanding; phase numbers from older discussions are historical.

### Changes after v1.3.1 — current implementation snapshot

These are present in the current source, but have **not** been verified on Mint.
Some v1.3.3 Windows binaries were local feature builds distinguished with a
`main.buildLabel`; this is not evidence of a published v1.3.3 release/tag.

- **Existing-album display names:** `AF1Qip...` remains the remote album ID.
  The user can provide a separate local display name. Saved-name lookup is
  account-scoped and takes precedence over Recent Albums and Album History.
  Names are UI labels only; the upload manager still passes the ID to the
  existing album API. No album rename or remote album listing is performed.
- **Saved Albums:** a persistent local list, separate from the 14-item Recent
  Albums list. Explicitly save from the existing-ID entry or Recent Albums;
  add, rename, select, and remove shortcuts. There is no Saved list cap or
  expiry. Removal deletes only the local shortcut, never the Google Photos
  album. Selection routes through the same existing album-ID upload flow.
- **Saved Albums sort/export/selection update (local build):** defaults to
  Recently added (newest first, reversing the locally persisted insertion
  order) or can sort alphabetically by display name, case-insensitively. Export
  opens the native Save File dialog and writes JSON with `schemaVersion: 1`,
  `exportedAt`, and an `albums` array of `{albumId, albumName}` pairs in the
  currently displayed order. The export does not include account/email data.
  The selected row's entire card receives green background/border styling and
  exposes `aria-pressed`; this is only a visual/accessible selection update.
  No backend schema, account storage, upload routing, or Google Photos behavior
  changed.
- **Importing that export on another PC:** the current app has no JSON import
  button, so the export is a transfer/reference file, not a directly loadable
  backup. Copy the JSON to Mint, open gotohp, select the account that owns the
  album IDs, then open Home → Saved Albums. For each object in `albums`, paste
  `albumId` into **Album identifier**, `albumName` into **Display name**, and
  choose **Save album**. Repeat for each entry. Saving through the UI validates
  IDs, updates an existing matching shortcut instead of duplicating it, and
  stores the list only under the selected account. Do not hand-edit `gotohp.config`
  for this workflow. Import is manual and one entry at a time; implementing a
  bulk JSON importer would be a separate feature.
- **Per-account avatars:** selected account uses a 72px avatar; dropdown rows
  use 24px thumbnails. Upload a local PNG/JPEG/GIF up to 5 MiB (decoded image
  limit 20 MP). Avatar bytes are copied to app data, not uploaded or logged;
  deleting an account removes its avatar.
- **Per-account app background:** Settings supports a local solid color or
  PNG/JPEG/GIF image up to 5 MiB, plus reset-to-default. Account switching
  selects that account's setting. The background is presentation only and is
  not sent to Google Photos.
- **Completion screen:** lists basenames of confirmed successful uploads in a
  bounded scroll area. Failed/cancelled/skipped rows remain in their separate
  review section; this list does not imply album assignment succeeded.
- **Upload Records order:** database query returns newest `recorded_at_ns`
  first, then path for a stable tie-break. Folder groups remain collapsed by
  default as described below.

Key implementation paths:

| Path | Responsibility / contract |
| --- | --- |
| `backend/configmanager.go` | `SavedAlbumsByAccount`, recents/history, per-account background map, and account removal cleanup. Saved-album Wails methods (`GetSavedAlbums`, `SaveSavedAlbum`, `RemoveSavedAlbum`) operate on `AppConfig.Selected`. |
| `backend/accountavatar.go` | Avatar validation, local storage, data URL retrieval and safe removal. Files are under `filepath.Dir(ConfigPath)/avatars`; SHA-256 of normalized email is the filename. |
| `backend/appbackground.go` | Account-scoped colors/image hashes; files under `filepath.Dir(ConfigPath)/backgrounds`. Image bytes are content-addressed with SHA-256. |
| `frontend/src/components/GoogleAccountSelect.vue` | 72px selected avatar and 24px dropdown avatar; image controls are local account-picker actions. |
| `frontend/src/SettingsPanel.vue` | Background selection/reset controls and settings persistence. |
| `frontend/src/SavedAlbums.vue` | Saved Album create/rename/select/remove UX, newest/alphabetical sort, JSON export, and full-card selected styling; remove copy explicitly says Google Photos album is untouched. |
| `frontend/src/utils/savedAlbums.ts` | Pure list sorting and JSON export-shape helpers. Added in the Saved Albums sort/export change. |
| `frontend/tests/savedAlbums.test.mjs` | Covers sort order, export fields, and Saved Albums controls/selected-card styling. |
| `frontend/src/utils/albumDisplayName.ts` | For an exact ID match, resolve a readable name in this order: Saved Albums, Recent Albums, then Album History; reject opaque IDs and `Saved album` placeholders. |
| `frontend/src/UploadComplete.vue` | Successful filename list, album additions, assignment errors and completion controls. |
| `frontend/src/utils/uploadPresentation.ts` | `uploadedFileName` returns a basename from Windows or Linux paths; tests are in `frontend/tests/uploadPresentation.test.mjs`. |
| `backend/configmanager_saved_album_test.go` | Persistence/account isolation/upsert/remove and unbounded Saved vs capped Recent behavior. |

### Current verification (Windows development host only)

- `cd frontend && node --test tests/*.test.mjs`: **45 passed**. These are explicit unit/behavior
  tests; they are not embedded in the app and do not run uploads automatically.
- `cd frontend && npm run build`: passed Vue type checking and production Vite
  bundle for the current Saved Albums sort/export UI.
- Windows GUI executable compiled with Go 1.26.8, `production` build tag,
  embedded frontend, and local linker label `Saved Albums Sort and Export`.
  File version metadata remains 1.3.3. Its SHA256 is listed above. This was a
  compile/metadata check only; it was not launched for a native UI click-through.
- Focused Go tests for Saved Albums and Recent Albums passed. The most recent
  full `go test ./backend` run had two failures in
  `TestRemovingAccountDeletesItsAvatarOnly` and
  `TestClearAccountAvatarRemovesOnlyThatAccountImage`, both reporting
  `could not verify local avatar storage`. Investigate on Linux rather than
  treating the full backend suite as green; all other tests in that run passed.
- Targeted ESLint on the changed completion files reported no errors. The full
  project lint has existing formatting warnings; it exits successfully.
- Latest local EXE build succeeded. No `.deb`, Linux binary, Mint launch, native
  GTK chooser, or live Mint upload has been verified.

### Local records — new in 1.3.0

The database foundation, worker integration and UI are implemented, not proposals.
See `LOCAL_UPLOAD_RECORDS_PLAN.md` for contracts and `WINDOWS_V1.3.0_TEST_CHECKLIST.md`
for tests adaptable to Mint (its version/window descriptions are historical).

- Store: `backend/uploadrecords.go`, using the existing pure-Go
  `modernc.org/sqlite` driver. No SQLite CLI/server installation is required.
- Location: `os.UserConfigDir()/gotohp/upload-records.sqlite`; normally
  `$XDG_CONFIG_HOME/gotohp/upload-records.sqlite` or `~/.config/gotohp/upload-records.sqlite`
  on Linux. It remains per-user even when credentials use a portable config.
- Schema version 1. Account + cleaned absolute path is the primary key;
  matching additionally requires size and modification time (nanoseconds).
  Preserve case; no content fingerprint or symlink-identity matching is added.
- Renaming/moving makes a new local candidate. Metadata-preserving replacements
  may be missed; timestamp-only changes can cause an unnecessary normal check.
- Records mean historical success, not proof of current remote existence.
  Prior uploads are not imported automatically. Do not transfer the Windows
  database to Mint as though its Windows paths were valid Linux identities.
- `record_uploads` defaults true; `skip_recorded_uploads` defaults false. Both
  controls are on Upload Records. Turning recording off retains old entries.
- `backend/uploadrecord_integration.go` wraps ordinary-file workers before
  hashing. Confirmed success is saved synchronously, with a bounded independent
  context so cancellation does not strand already completed records.
- Upload Only skips local matches. Album destinations reuse saved media keys
  without retransferring. Album errors do not erase transfer success or cause
  silent reupload. Destination keys are deduplicated before album calls.
- Force Upload, `startUpload.bypassLocalRecords`, and Delete From Host bypass
  local skips. Local records must NEVER authorize source deletion. Live Photo
  work items bypass this layer and retain existing paired-item handling.
- Account/options/destination are captured per batch; normal upload/auth/device
  identity behavior is retained. Missing media keys fail explicitly; unavailable
  databases warn and use the normal upload path. Never delete a corrupt DB.
- Indexed lookup/write calls are bounded to 256 records. Workers currently
  commit one success at a time: durable and simple, but benchmark small-file
  overhead on Mint before claiming maximum throughput.
- `backend/uploadrecords_ui.go`: GetUploadRecords(account, search, offset),
  ForgetUploadRecords(account, paths), ExportUploadRecords(account, path),
  PreviewUploadRecords(paths, id), CancelUploadRecordPreview(id). Regenerate bindings.
- List pages have 50 entries grouped by folder. Explicit-submit substring search
  and deep OFFSET pages can be slower than indexed exact matching. Folder counts
  describe this page, not all historical entries in the folder.
- CSV exports all records for the selected account, with paths/timestamps but
  without remote media keys or credentials. Forget requires confirmation and
  only removes local rows, not media/files/Album History.
- All source entry points share a cancelable match preview. Zero matches
  continue automatically; matches offer Cancel, normal local skipping, or Ignore
  local records this time. Album reuse intent is explicit. Worker checks run
  again, so file edits can change counts after preview. Request IDs prevent stale
  preview responses from starting an abandoned selection.
- This does not make the earlier candidate-count scan cancelable; that separate
  preparatory scan still precedes the record preview.

### UI refinements — 1.3.1

- `frontend/src/UploadRecords.vue` has native `<details>/<summary>` folder groups,
  collapsed by default, keyboard operable, with visible per-page counts. Opening
  reveals selectable file rows. Search/page changes create fresh collapsed groups.
- `maxRecentAlbumsPerAccount` is now **14**, not 8. It fills as albums are used;
  older entries discarded under the previous limit are not automatically restored.
- Initial window is **800 × 900**; minimum 400 × 600, maximum 800 × 900.
  Test work-area fit on Mint, including panels/scaling/small displays.
- Shared debug button uses z-index 60; notification toaster overrides its library
  default with z-index 40 and 64px desktop/mobile bottom offsets. Sheets use 50.
  Keep modal button instances inside Settings/account-setup focus traps. Test
  warnings with the button on every screen, not just Home. Native OS dialogs
  necessarily appear above in-app content.
- Debug schema remains **3**. Screen names include upload-records/record-review;
  local skips retain `local-record` in skippedDetails. Completion separates local
  records from other skips and actual new upload success.

## Historical edge-case update — included since 1.3.0

The following fixes were originally unbuilt changes after 1.2.1. They are now included in 1.3.0 and 1.3.1.

- Backend `FileUploadResult.Cancelled` identifies errors that satisfy `errors.Is(err, context.Canceled)`. The frontend records these separately in `results.cancelled`, not `results.fail`; unrelated errors remain failures. Transfer/authentication algorithms are unchanged.
- Completion shows Cancelled and No final result counts. The latter is `max(0, Total - processed)` and does not assert that those items never began. Debug export is now schema **3**, with cancelled file basenames and the same counts.
- `ConfigManager.CountUploadCandidates(paths)` uses the existing local scanner. Selection blocks empty/unsupported-only inputs before changing backend upload settings. Auto Album also rejects an empty classified folder preview. Existing filtering/recursion settings apply.
- `uploadSelection.starting` blocks duplicate starts between event dispatch and backend uploadStart. A reactive watch clears it once backend uploading state arrives; dispatch failure releases it. Home displays Starting upload during this interval.
- Local validation adds a scan before upload preflight; large source trees may take longer to prepare, and this preparatory scan does not currently expose cancellation. Do not describe it as instantaneous or cancelable.
- Regenerate Wails bindings for the added method/model field. New tests include `frontend/tests/uploadResults.test.mjs` and backend empty/missing-candidate checks. All 27 frontend tests passed together; Go CLI-tag tests passed.

New user evidence from `log/3 auto album.json` (Windows 1.2.1): 30 uploaded, 0 failed, three completed albums with 8/13/9 additions. `log/upload cancel.json`: cancel requested, 8 cancellation-category errors and 2 items without a final result, 0 successful uploads. This supports working stop behavior but does not verify preservation of earlier successes in that particular batch. These user logs are private, not release contents.

The current baseline above takes precedence over historical evidence and older test counts. Native Mint testing remains pending.

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
| 1.3.0 | Local records, optional skip/reuse, history UI, cancellation safeguards and toast layering | Automated checks and Windows packaging; live acceptance pending |
| 1.3.1 | Collapsed folders, 14 recent albums, larger initial window | 33 frontend tests and backend production tests pass; Windows build/version verified |
| 1.3.2 | Local display names for existing album IDs; album-name handoff correction | Windows executable exists; Mint not tested |
| 1.3.3 | Account-specific backgrounds; subsequent local builds also add avatars, Saved Albums, uploaded filenames, and Saved Albums sort/export controls | Version metadata is 1.3.3; no later published tag confirmed. Current labeled Windows EXE listed above |

Latest Windows artifact: `bin/gotohp-v1.3.3-saved-album-sort-export.exe`, title
`gotohp v1.3.3 — Saved Albums Sort and Export`. Its build label is a local
artifact identifier, not a semver change. No current Linux binary or `.deb`
has been built or verified.

Implementation is complete through local-record phase 5 Windows packaging. Native/platform acceptance testing remains pending.

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
- **Saved Albums** is also a separate Home destination. It contains explicit
  local bookmarks for existing album IDs and is not a remote album browser.
- Account avatars and background selection are entirely local UI state. Do not
  couple them to upload/auth payloads, album APIs or debug logs.

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
- The **Uploaded files** section derives only from `state.results.success` and
  displays filenames, not full paths. It is capped at 12rem high with its own
  scroll area for large batches. Do not mix album-add failures with file-upload
  failures or list skipped/failed files as successful.

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
| `frontend/src/SavedAlbums.vue` | Local per-account album-ID bookmarks and select/add/rename/remove UI |
| `frontend/src/utils/debugLog.ts` | Filename redaction and inferred error categories; deliberately avoids exporting raw server errors |
| `frontend/src/components/DebugLogButton.vue` | Shared floating export button |
| `frontend/src/components/GoogleAuthSetup.vue` | Existing auth UI; added slot/spacer so log button is inside modal focus boundary; no auth algorithm changes |
| `frontend/src/index.css` | Theme, visible keyboard focus, layout minimums |
| `backend/configmanager.go` | Persistent per-account recents/history, diagnostic file export |
| `backend/accountavatar.go`, `backend/appbackground.go` | Local account image and background persistence/validation; no Google Photos calls |
| `backend/album.go` | Existing key-vs-title behavior, batching and album splitting; do not replace API calls |
| `frontend/bindings/` | Generated Wails TypeScript; regenerate, do not hand-edit |

Selection order: await `SetSelected(account)` -> `SetAlbumName(nameOrKey or empty)` -> `SetAlbumAutoMode(boolean)` -> optional record review -> `Events.Emit('startUpload', {files, bypassLocalRecords})`. Cancelling review must not dispatch or leave selection locked.

Backend starts with a preflight `uploadStart` and emits another after classification. `UploadBatchStart` fields include `Total`, `TotalBytes`, `AlbumName`, `AlbumAutoMode`, `AutoAlbumFolders`, `WorkPaths`. WorkPaths contains primary work-item paths; Live Photos can represent paired sources, so counts are work items rather than necessarily physical filesystem files.

Other events: `uploadTotalBytes`, `uploadTotalBytesDelta`, `uploadWarning`, `ThreadStatus`, `FileStatus`, `albumProgress`, `albumComplete`, `albumError`, `uploadStop`. UI emits `uploadCancel`.

The pending readable Recent Album name must survive both startup events. It is assigned at actual dispatch and cleared on stop. Do not let a cancelled picker leak a prior name into another mode.

`recordAlbumProgress` tracks by album key and subtracts earlier split albums from cumulative ItemsAdded. Completion marks existing rows instead of appending duplicates.

## 5. Persistence, history and logs

- `RecentAlbumsByAccount`: max 14 per account, `albumId`, `albumName`, `lastUsedAt` (Unix seconds). Successful automatic-folder albums are included.
- `AlbumHistoryByAccount`: max 40 per account, `albumId`, `albumName`, `itemsAdded`, `operation`, `createdAt` (Unix seconds).
- `SavedAlbumsByAccount`: no automatic cap, each row has `albumId` and `albumName`. The YAML/Koanf key is `saved_albums_by_account`; the config map is hidden from generic JSON serialization. Save/rename/remove methods operate on `AppConfig.Selected`; call `SetSelected` before cross-account operations.
- `AF1Qip...` IDs remain authoritative. A local alias never renames or creates the remote album. For display, a saved alias overrides Recent and History labels. Saved-album shortcut deletion does not delete remote data and does not delete the History/Recent row.
- Entering an existing album ID in the upload flow requires a separate Display name. The label may be prefetched from matching Saved/Recent/History data. Only the ID is sent through `SetAlbumName` to the existing upload path. Share URLs are not parsed by this feature.
- An ID associated with the wrong Google account can return a 404 while the files have already uploaded to that account's library. The latest user log confirmed this was an account mismatch. Do not auto-retry the files or create an album on 404; verify the intended account/ID first.
- A plain album name still means “create a new album,” even if a matching title exists. No remote title matching or account-wide album listing exists.
- If no readable local name is known, IDs show as `Saved album`. Legacy entries cannot recover a human name without the user adding an alias; no remote lookup exists.
- History and recent writes currently originate from frontend album-completion events. Partial album additions appear in results, but partial/error paths do not currently guarantee a persisted history entry. Review this before claiming complete audit coverage.
- Normal Linux configuration is under the OS config directory, usually `~/.config/gotohp/gotohp.config` (XDG applies). An executable-adjacent `gotohp.config` takes precedence. Check `backend/configmanager.go` for exact resolution; never log credential contents.
- Config writes use existing persistence; do not reset data for a UI upgrade.
- Avatar files are stored in sibling directory `avatars/`, named by SHA-256 of
  lower-cased trimmed email; background images are in `backgrounds/`, named by
  content SHA-256. Both are under `filepath.Dir(ConfigPath)`, with directories
  mode 0700 and files mode 0600. Avatar/background associations normalize the
  account email; Saved/Recent/History maps use the selected account key. Account
  removal clears that account's background entry and avatar, but intentionally
  retains album history/shortcuts. If transferring local UI state between
  machines, config and image directories are separate pieces of private app
  data; never copy or share credentials/debug data casually.

Debug export schema v3 snapshots state **before** opening the save dialog. It includes application version, screen, viewport dimensions, selection mode, cancellation request flag, timestamps, byte/work counts, per-folder counts, per-album additions, worker status/attempt, and inferred error categories.

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

**Metadata mismatch:** `build/windows/info.json` is now 1.3.3, but
`build/linux/nfpm/nfpm.yaml` still has `version: "0.10.0"`, release 1, and
`README.md` still headlines v1.3.1. Before making a Linux package, decide and
align the Linux package version with the intended artifact (current code
metadata is 1.3.3); do this before rebuilding frontend because debug logs import
the embedded info file. CLI packaging has separate config; inspect it if making
a CLI artifact. Asset regeneration can overwrite custom metadata/icons using
`build/config.yml`: inspect and align that source before running it. Do not
blindly change package identity to Gop just because the GitHub repository was
renamed.

`build/windows/info.json` currently describes account backgrounds. Local
Windows test builds set a display-only linker label such as `Saved-Albums` or
`Uploaded-Filenames`; the normal Linux task does not set this label. Either
pass a clear local test label via `-X main.buildLabel=...` or use the plain
version title. Do not represent the test label as an upstream release.

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

- Current frontend `npm run build` passed (Vue type checking + production Vite build).
- Current `node --test tests/*.test.mjs` run: 45/45 passed, including Saved Album sorting/export fields/UI checks, naming helpers, successful-filename basename handling, upload selection, local record review, and startup-state checks. Tests run only when invoked; they are not embedded or auto-triggered by the app.
- Focused Go Saved/Recent Album tests passed. A full backend run did not pass cleanly: the two avatar cleanup tests listed above failed on the Windows development host. These need investigation on the Linux builder.
- Targeted ESLint for `SavedAlbums.vue` and `utils/savedAlbums.ts` reported zero errors; the existing component has 79 formatting warnings. Full project lint was not rerun for this build.
- Windows artifact compiled; latest artifact and SHA256 are listed above. Native Mint rendering, Save File dialog, GTK dialogs, and `.deb` upgrade remain unverified.

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
7. Cancel media transfer and album assignment separately; export schema v3 logs and compare confirmed files/albums in Google Photos. Avoid blindly retrying files already uploaded.
8. Account switching does not expose another account's recent/history entries.
9. History loading/empty/error/full states, long names, local timestamps, Created/Updated and count accuracy.
10. Resize between compact and maximum dimensions; check scrolling, no hidden controls/overlap, Tab/Shift+Tab/Enter, modal focus and export button in every screen.
11. Logs identify settings/account/history/review/upload/result screens and the correct version. Inspect privately for accidental sensitive content before sharing.
12. Install over the old package without losing settings; verify rollback with the retained package.

13. Upload Records: enable recording, upload a few disposable files, reopen and verify persistence. Enable skipping and repeat; all-recorded Upload Only must complete with no transfer. Mix new and recorded files, then use New/Recent/three-folder Auto albums and confirm membership without retransferring local matches.
14. Rename/move/replace a file; test account isolation, Force Upload and explicit bypass. Keep Delete From Host off for routine testing. Confirm local records never cause local deletion. Verify a success remains recorded if a later file or album operation fails/cancels.
15. Test collapsed folders, keyboard expansion, page/search reset, visible per-page counts, CSV output and confirmed forget. Forget must not delete media or files. Test unwritable/corrupt test databases without touching real user history. Test cancelling a preview then immediately starting another selection.
16. Verify 800×900 default fits the Mint work area, smaller resizing, and debug button access above warnings inside every screen/modal. Confirm recent capacity reaches 14 as new albums are used.
17. Saved Albums: save, rename, select and remove a local shortcut; verify data survives restart and does not cross accounts. Test Recently added (newest first) and case-insensitive Alphabetical sorting. Export JSON and verify each `albumId` stays paired with its `albumName`, export order follows the selected sort, and no account/email is included. Transfer the export to Mint and manually re-enter each pair through Saved Albums; verify the selected account receives the shortcuts and duplicate IDs update their local name. There is no bulk JSON import control in this build. Check removal leaves the remote album untouched. Test same ID under another account and confirm a 404 is displayed as an album error without reporting transfer failure. Confirm the selected whole card is visibly green and keyboard/screen-reader state exposes `aria-pressed`.
18. Existing-ID display alias: enter an AF1Qip key and local label, upload to the intended account, and verify progress/results/recent/history use that label while the Google Photos title stays unchanged. Confirm plain text title still creates a new album.
19. Account visuals: set different 72px avatars and background color/image on two test accounts, switch between them, restart, reset one background, and remove one account. Confirm image files persist locally with the app config and no image/avatar content is exported in debug logs.
20. Completion file list: upload a mix of successful, failed, skipped and cancelled files; only successful basenames should appear in the scrollable list. Confirm same-named source files still appear as separate rows; test long names and a large batch without hiding Go back home.

Remaining gaps: native visual/accessibility and live-upload validation, native verification of empty/duplicate/cancellation safeguards, persistence of partial album updates in Album History, and old Saved album entries without a known name. Local-record implementation is present but still needs Mint acceptance and small-file performance testing. Full backend tests currently include two failing avatar cleanup tests on the Windows host. Do not silently expand into remote album fetching or transport changes.

## 11. Earlier transport fix and speed observations

`backend/httpclient.go` initializes nil TLSClientConfig after cloning the default transport. Preserve the regression fix/tests. It prevented a panic during an HTTP/1 diagnostic (`GODEBUG=http2client=0`); it does not force HTTP/1 or disable HTTP/2. Never ship those diagnostic environment overrides by default.

User observed roughly 1–2 MB/s on Windows and full speed on another Mint computer using the same network. WSA looked faster with a different account. Cause remains unresolved; these observations do not prove a Windows transport defect. Packaging/UI work should not alter this subsystem.

## 12. Expected next-agent deliverable

Use this snapshot, determine target compatibility, fix only required Linux integration issues, launch/test the real GUI, and deliver a clearly versioned `.deb`, SHA-256, install/rollback steps and honest test results. Record any unverified behavior. A successful compile is not proof of a working rendered UI.
