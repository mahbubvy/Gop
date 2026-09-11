# gotohp v1.3.0 — Local upload records

Build: `gotohp-v1.3.0.exe` (Windows x64). Earlier EXEs remain unchanged.
Close the earlier version before launching this one. This portable EXE uses the
existing account/settings configuration; it is not an installer.

## Completed automated checks

- Backend tests under both `cli` and `production` tags pass.
- All 32 frontend tests pass, including reactive startup and debug-layer checks.
- TypeScript/Vite production build and ESLint error checks pass.
- Release executable CLI version/help smoke checks pass.
- Windows FileVersion/ProductVersion: 1.3.0; description: Local upload records.
- SHA256: DC33E87CCCA4EF652B9507400BE3DC56DBC0684277C38EECD9DE99D7F68A1F99

These are not native GUI or live Google Photos acceptance tests. The checks below
still require hands-on testing; no real uploads or account mutations were made
during release verification.

## Start with a small test folder

Use a few disposable copies of photos/videos. Leave Delete From Host OFF.
For testing skips, also leave Force Upload OFF.

1. Open Upload Records on Home. Recording should be on and skipping off by
   default, unless previously configured. Upload two test files using Upload Only.
   Completion stays open; no album section. Go home and confirm records appear.
2. Enable Skip files recorded as uploaded. Select those two files again. The
   review should show both recorded. Finish without uploading; completion should
   show two locally recorded items, zero new uploads and zero transferred bytes.
3. Add one new file. Select all three. Expect two recorded and one normal-upload
   candidate. Only the new file needs the normal upload path.
4. Select recorded files for a New album and a Recent album. Expect no transfer
   for recorded files, but confirmed album updates. Verify membership in Photos.
   Recent album names must remain readable after completion.
5. Use Auto Album on three folders, including recorded files. Confirm all three
   folders/albums appear separately and receive the correct unique media items.
6. Rename/move one test file, or replace it with a different-sized file. It should
   stop matching locally. The ordinary uploader may still detect it remotely.
7. Choose Ignore local records this time. This bypasses local matching only;
   ordinary remote duplicate detection still applies. Force Upload also overrides
   local skips. Do not enable Delete From Host just to test bypass behavior.
8. Cancel a record scan, then immediately start another selection. No abandoned
   scan should trigger an upload. Cancel an upload after at least one confirmed
   success; that success should remain recorded after reopening the app.
9. Search records by folder/file name, change pages, export CSV and inspect it.
   Export includes the whole selected account, not just the current page/search.
   Forget one selected record: only its local entry disappears. Media and Album
   History stay unchanged. Another account must not match/display these records.
10. Test compact and enlarged windows. With a warning toast visible, click the
    bottom-right debug button on Home, Upload Records, review, uploading/results,
    Settings and account setup. Toasts sit above a 64px bottom gap and below the
    button. Export and cancel the native save dialog; the app should remain usable.

Records live in the OS user config directory at `gotohp/upload-records.sqlite`.
They begin with confirmed activity in this version; old uploads are not imported.
They record past success only, not whether media still exists remotely. Renamed
files are new candidates; metadata-preserving replacements cannot be detected.
Never use local records as proof that deleting a source file is safe.

If anything fails, export the screen's debug log and note the test number. Native
save dialogs are OS windows; the floating button only stays above in-app content.
