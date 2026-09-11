# Local upload records: phased implementation

## Status

Phases 1–4 implemented. Phase 5 automated checks and Windows packaging completed
as `gotohp-v1.3.0.exe`. Native GUI/live upload acceptance remains pending; use
`WINDOWS_V1.3.0_TEST_CHECKLIST.md`. No live uploads were performed for verification.
Notifications now use layer 40 below sheets (50) and debug export (60), with a
64px bottom offset. Windows resource strings use English (0409), making version
and description visible in Explorer as well as in the application.

### Phase 4 UI and service notes

- Home has an Upload Records button below Album History/Settings. Its screen
  has independent recording and skipping controls with save-error reporting.
- Account-specific search covers file/folder paths; pages contain at most 50
  records grouped by folder. Folder counts explicitly refer to this page, not
  lifetime totals. Filename, size, recorded timestamp and origin are displayed.
- Search is explicit-submit, not a full-table search on every keystroke. Listing
  uses indexed account/path ordering with bounded pages; substring search and
  deep OFFSET pages can cost more than exact matching in very large databases.
- Forgetting selected records requires inline confirmation and an account-bound
  delete. It does not touch media, files or Album History. Selection resets on
  search/page changes. CSV exports all records for the account, not just the
  current search, streams output to a temporary file, excludes remote media keys,
  and neutralizes formula-like text cells. The output includes full local paths.
- The shared picker/drop/recent selection pipeline runs a cancelable local
  preview when skipping is enabled and not overridden by Force Upload or Delete
  From Host. With zero matches it continues automatically. Matches show recorded
  vs normal-check counts, album intent, Cancel, and a one-batch bypass. All-matched
  Upload Only offers Finish without uploading. Actual workers recheck metadata.
- Preview failures do not silently bypass records: the user chooses Cancel or
  normal uploading. Request IDs isolate preview cancellation/stale responses.
- The debug button stays available; currentScreen distinguishes upload-records
  and record-review. History rows themselves are not copied into debug exports.
- Coverage adds page boundaries, search, per-account forget, CSV escaping/key
  omission, preview count/cancellation and selection review/bypass tests. Native
  layout, real save dialogs and Google album updates still need phase 5 testing.

### Phase 3 integration notes

- `record_uploads` defaults true; `skip_recorded_uploads` defaults false. Backend
  setters, generated bindings and phase 4 user-facing controls exist.
- `startUpload.bypassLocalRecords` is a per-batch bypass. Existing Force Upload
  also bypasses local skips. Delete From Host bypasses them unconditionally.
- Account credentials, destination and upload options are captured per batch.
  API construction/commit uses the captured options, retaining the same auth,
  device identity rules and HTTP implementation.
- Each ordinary-file worker checks one indexed candidate before hashing and
  commits each confirmed success synchronously. There is no pending write queue.
  This intentionally favors immediate persistence over batching disk commits;
  measure small-file overhead before introducing a buffered writer.
- Confirmed writes use a bounded independent context, preserving successes when
  the upload context is cancelled. Unverified/cancelled failures aren't recorded.
- Live Photo work bypasses the record layer. Database unavailability warns and
  falls back to normal upload; a matching entry missing its media key fails
  explicitly, requesting a bypass retry, rather than silently retransferring.
- Local skips carry `SkipCode: local-record` and a saved MediaKey through the
  existing result/album collection. Album calls deduplicate keys per destination.
  An album error never rolls back transfer records or starts a reupload.
- The initial byte total is published before workers; local skips subtract their
  sizes and contribute no transferred bytes. Completion separates locally
  recorded items from other skips; debug skippedDetails retains their code.
- Tests cover all-recorded/mixed three-folder candidate flow, bypasses, account
  isolation, cancellation, confirmed cleanup warnings, mutation, database failure,
  missing keys and frontend skip counting. Actual Google album assignment and
  native completion-screen acceptance remain phase 5 tests.

Phase 2 implementation: `backend/uploadrecords.go` and its tests. The default
location is the OS user config directory under `gotohp/upload-records.sqlite`,
independent of portable credential configuration. Schema version 1 uses a
composite account/path primary key. Callers supply a stable account identifier.
Paths retain their original case and are cleaned absolute paths.

SaveConfirmed and Match accept at most 256 items per call. Writes commit
synchronously and atomically; the store has no hidden pending buffer. Phase 3
must bound any caller-side queue, flush it on finish/cancellation using an
appropriate non-cancelled flush context, and surface database errors as warnings.
Only that unflushed caller queue could be lost on a crash.
The phase 3 implementation currently has no such queue: it commits each success.

CaptureUploadRecordSnapshot and ConfirmUploadRecord support before/after metadata
checks and explicitly confirmed post-success local deletion. Phase 3 must call
them at the actual upload success boundaries; the database cannot independently
prove that a supplied media key represents a successful upload. Album success
is not a condition for storing media success. Missing media keys are rejected.

Verification: `go test -mod=readonly -tags cli ./...` passes, including persistence,
account isolation, metadata mismatch, upsert, cancellation, atomic rollback,
schema initialization/reopen, future-schema refusal, corrupt-file preservation,
source mutation/deletion checks and 5,000-record batched lookup. These tests are
local synthetic checks, not an end-to-end upload or native GUI test.

## Phase 1 — matching and destination rules

Use the recommended rules from the planning discussion:

- Records belong to the current PC and Google account, not all accounts together.
- A match requires normalized absolute path, byte size, and last-modified time.
  This uses filesystem metadata, not content fingerprints.
- Renaming or moving a file makes it a new local candidate. The same basename in
  another folder does not match. A replaced file with changed size or timestamp
  is a new candidate. Replacements preserving both can be missed; timestamp-only
  changes can cause unnecessary uploads. Provide an explicit bypass.
- Clean absolute paths using platform rules. Preserve original display paths;
  do not blindly lowercase paths (case-sensitive directories exist on Windows).
  Do not resolve symlinks or attempt cross-drive identity matching initially.
- Check each file, never skip a whole folder because its name is recorded.
- The record means confirmed success in the past, not current remote existence.
  There is no server lookup to validate a local match.
- Upload Only: a match skips media transfer.
- New, Recent, and Auto albums: a match skips media transfer but its saved media
  key is included in the selected destination's normal album assignment.
- Missing saved media keys or failed album assignments must be visible. Offer
  an explicit normal-upload retry; do not silently reupload recorded files.
- Keep transfer success and album assignment success separate. An album error
  must not erase a confirmed media success record.
- Preserve current title/key semantics: a new album title creates an album;
  selecting a recent album key targets that existing album. No name-based lookup.
- Live Photo work items bypass local skipping in the initial implementation;
  preserve their existing paired-item behavior.

Safety and settings:

- Separate record keeping from skipping recorded files. Start with record
  keeping enabled and local skipping opt-in; disclose the historical-only check.
- Force Upload and an explicit per-batch bypass take precedence over local skips.
- Local records must never authorize Delete From Host. With that setting active,
  bypass local skipping and retain the current verified upload/remote-match path.
- Never change authentication, device identity, HTTP transport, commit behavior,
  or existing remote duplicate detection on the normal upload path. This feature
  makes no guarantee about Google's storage accounting.

## Phase 2 — database foundation

- Use the existing modernc.org/sqlite dependency; a separate per-user local
  database, not the credential config, and not inside the source/media folder.
- Version schema migrations. Index account + normalized path. Store display path,
  size, modification time, successful-record timestamp, and returned media key.
- Distinguish a new transfer from an existing remote match where available;
  record neither cancellation nor failures nor unverified generic skips.
- Capture metadata before transfer and validate it before caching success when
  the source still exists. Account for current post-upload local deletion.
- Use bounded batched lookups and short batched write transactions, flushing on
  normal finish/cancellation. A crash can lose only the pending unflushed records;
  document this rather than claiming exactly-once behavior.
- Database errors must not silently skip files. Show a warning and allow the
  normal upload path; do not delete or silently recreate a corrupt database.
- Tests: account isolation, changed metadata, upsert, persistence, migrations,
  failed operations and thousands-of-record lookup behavior.

## Phase 3 — upload integration

- Capture account and relevant settings once per batch.
- Make record checks cancelable and bounded-memory, before transfer hashing.
- Keep locally skipped, newly uploaded, cancelled, failed, and album-added counts
  distinct. Local matches contribute no transferred bytes or artificial speed.
- All-recorded album batches still run album assignment; all-recorded Upload Only
  batches still reach the persistent completion screen.
- Reuse existing destination grouping. Deduplicate media keys within an album
  request and report only confirmed album additions.
- Persist confirmed successes even if a later item or album operation fails.
- Add tests for mixed/all-matched batches, missing keys, cancellation, force
  bypass, Delete From Host, and unchanged Live Photo handling.

## Phase 4 — records UI

- Home Upload Records button; account-specific, searchable, paginated records
  grouped by folder, with filenames and timestamps.
- Clear recording and local-skip controls plus a per-batch bypass.
- Pre-upload matched/new summary when local matches exist; no redundant dialog
  when none exist. Show the album-add intent for recorded media explicitly.
- Export CSV/text and forget selected records, explaining that forgetting is
  local only and does not delete local files or Google Photos media.
- Completion and debug export distinguish local matches from transfer success.

## Phase 5 — Windows build and verification

- Run backend/frontend tests and build checks; create a separately versioned EXE.
- Verify large libraries, renamed/replaced files, same names in different folders,
  different accounts, repeated runs, cancellation/restart and database failures.
- Test Upload Only, New, Recent and three-folder Auto albums, including batches
  with no new transfers and album assignment errors.
- Include the previously implemented but unbuilt cancellation/empty-selection
  safeguards in regression testing. Do not claim native GUI tests from unit tests.

## Existing integration points checked

- backend/upload.go: workers return FileUploadResult.MediaKey; successful path/key
  mappings are collected before album assignment. Cleanup errors with a returned
  key already preserve successful transfer results separately from warnings.
- backend/upload.go: ordinary transfers calculate SHA1, optionally check remote
  duplicates, commit media and can delete the source afterward. Leave this path
  unchanged for nonmatches and bypass cases.
- backend/album.go: AddToAlbum accepts media keys; title versus key determines
  new versus existing albums. Normal album operations still require network.
- backend/configmanager.go and go.mod: SQLite is already used/dependent; reuse the
  driver but keep the new record store separate from existing credential work.
