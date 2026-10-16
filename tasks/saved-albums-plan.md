# Implementation Plan: Saved Albums

## Overview

Add a persistent, local Saved Albums list for existing Google Photos album IDs. The list is account-scoped and independent from the 14-entry Recent Albums list. Saved display names remain local; uploads continue using the album ID through the existing uploader.

## Decisions

- Use a distinct **Saved Albums** home button and page.
- Save explicitly from the existing-album ID form or a Recent Albums entry; do not auto-save every album.
- Saved entries have no automatic count limit or expiry. Recent Albums keeps its existing 14-item limit.
- Allow adding, renaming, choosing, and removing local shortcuts. Removing one never deletes the Google Photos album.
- Store entries locally by account email, consistent with the existing recent/history maps. Keep them if credentials are removed, as existing album history is retained.
- Never send a display name to Google Photos or request an album rename.

## Task List

### Slice 1: Local account-scoped storage

- [x] Add saved-album config model and selected-account list/upsert/remove methods.
- [x] Validate AF1Qip IDs and non-empty local display names; same ID updates its saved name.
- [x] Test persistence, account isolation, upsert, remove, and independence from the 14-item recent limit.

### Slice 2: Saved Albums UI and upload path

- [x] Add a Saved Albums page with add, rename, use, remove, and empty/error states.
- [x] Add explicit save actions to the existing-ID form and Recent Albums list.
- [x] Route selected saved albums through the existing upload selection flow, passing only the ID to the uploader and using the local name for UI display.

### Checkpoint: Feature

- [x] Focused backend and frontend tests pass; production frontend build passes.
- [x] Verify saved IDs/names are account-scoped and recent albums still cap at 14.
- [x] Build a separately named Windows EXE; do not publish or push.

## Risks

| Risk | Mitigation |
|---|---|
| Local label accidentally reaches Google Photos | Reuse the existing ID-based upload route; keep display names in local UI state/config only. |
| Saved list and Recent list become coupled | Store in a separate config map and preserve the existing recent trimming behavior. |
| Removing a shortcut is mistaken for deleting the remote album | Label the action as local removal and explain that the Google Photos album remains untouched. |
