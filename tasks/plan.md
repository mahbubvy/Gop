# Implementation Plan: Per-account custom avatars

## Overview

Let the user choose a local image for each connected Google account and show it beside that account in the account selector. Keep avatar data local to this app and independent of Google Photos uploads or account credentials.

## Decisions and assumptions

- Copy the chosen image into the app's existing per-user `gotohp` config directory, so moving or deleting the original image does not break the avatar.
- Associate the saved image with the account email, matching the app's existing account-scoped local settings.
- Display a circular thumbnail with `object-fit: cover`; no crop editor in the first version.
- Implemented formats: JPEG, PNG, and GIF, validated by Go's standard image decoders; WebP is not included.
- Maximum selected-image size: 5 MiB (confirmed).
- Removing an account deletes its app-managed avatar (confirmed).
- Never send avatar data to Google Photos or include it in debug logs.

## Task List

### Phase 1: Local storage

- [x] Task 1: Add safe per-account avatar storage and retrieval.
  - Validate format and size, copy atomically into the app config directory, and scope by account.
  - Acceptance: one account's avatar cannot replace or expose another's; invalid/oversized files leave the previous avatar intact; missing avatar returns an empty value.
  - Verification: focused Go tests for save, replace, account isolation, validation, and account removal.

### Phase 2: Account picker UX

- [x] Task 2: Add image selection and avatar display to the existing account picker.
  - Acceptance: user can set/replace/remove a custom image; selected account and dropdown entries show the correct avatar; accounts without one retain a readable fallback; changing accounts does not change the avatar association.
  - Verification: frontend tests and manual check with two accounts, including a default-avatar account.

### Checkpoint: Complete

- [x] Go and frontend tests pass; frontend production build succeeds.
- [ ] Windows executable build and manual multi-account UI test remain.
- [x] Images remain local; avatar data is not connected to upload events or debug export.

## Risks

| Risk | Mitigation |
|---|---|
| Original image path becomes invalid | Copy to app-managed config storage. |
| Large or malformed image affects UI/startup | Enforce a small size limit and validate actual image decoding before replacing the saved avatar. |
| Account switching shows the wrong image | Key avatar storage by account and test with multiple accounts. |
| Local personal image leaks into logs or upload flow | Keep avatar APIs separate from upload events and omit avatar values from diagnostics. |

## Open questions

- No open implementation questions. WebP can be added later if needed.
