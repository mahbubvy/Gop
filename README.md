# Gophoto-UI

Personal UI-enhanced version of [xob0t/gotohp](https://github.com/xob0t/gotohp).
Original MIT license and attribution are retained.

Windows release: **v1.3.0 — Local upload records**. Adds local upload history,
optional local repeat-upload skipping, album media reuse, CSV export and UI
improvements. Native acceptance checks: [test checklist](WINDOWS_V1.3.0_TEST_CHECKLIST.md).
Download the EXE from this repository's Releases page. Release workflows are
manual; the v1.3.0 asset is the locally tested Windows x64 build.

## Original project overview

![demo](readme_assets/app_demo.webp)

Unofficial Google Photos Desktop GUI Client

- Unlimited uploads (can be disabled)
- Home-screen file/folder selection and drag-and-drop uploads
- Credential management
- Real-time upload progress tracking
- Configurable upload threads
- Individual files or directories uploads, with optional recursive scanning
- Skips files already present in your account
- CLI mode
- Configurable, presistent upload settings (stored in "%system config path%/gotohp/gotohp.config")  
   You can force local config by creating empty gotohp.config next to executable.

## [Download](https://github.com/xob0t/gotohp/releases/latest)

## Upload from the home screen

Select your account and an upload mode:

- **Upload Only** uploads without adding files to an album.
- **Upload to Album** asks for an album name or key after you select files or a folder.
- **Auto Album** uses each file's parent folder name for its album.

Click **Choose files** (multiple selection supported) or **Choose folder**.
Upload Only and Auto Album start uploading after you accept the picker;
Upload to Album waits for the album confirmation. Cancelling does not start an upload.
Subfolders are included only when **Settings → Recursive Directory Upload** is enabled.
Drag-and-drop remains available; a drop without a specific drop zone uses the selected home-screen mode.

## CLI usage

Download the `gotohp-cli` release artifact for your platform. It runs without the GUI.

```shell
gotohp-cli upload /path/to/photos --recursive --threads 5
gotohp-cli creds add "androidId=..."
gotohp-cli creds set user@gmail.com
```

Run `gotohp-cli help` for all commands and options.

## Sign in

### Option 1 - Embedded setup

1. Open gotohp and click **Add Google account**.
2. Click **Open Google sign-in** and sign in with the account you want to add.
3. Sign in on the Google Embedded Setup page and click **I agree**. The page may keep loading forever; this is expected.
4. Open the browser developer tools. Under Application or Storage, open Cookies for `accounts.google.com`.
5. Copy only the value of the `oauth_token` cookie and paste it into gotohp.
6. Click **Connect account**.

### Option 2 - ReVanced. No root required

<details>
  <summary><strong>Click to expand</strong></summary>

1. Install Google Photos ReVanced on your android device/emulator.
   - Install GmsCore [https://github.com/ReVanced/GmsCore/releases](https://github.com/ReVanced/GmsCore/releases)
   - Install patched apk [https://github.com/RookieEnough/Morphe-AutoBuilds/releases](https://github.com/RookieEnough/Morphe-AutoBuilds/releases) or patch it yourself
2. Connect the device to your PC via ADB.
3. Open the terminal on your PC and execute

   Windows

   ```cmd
   adb logcat | FINDSTR "auth%2Fphotos.native"
   ```

   Linux/Mac

   ```shell
   adb logcat | grep "auth%2Fphotos.native"
   ```

4. If you are already using ReVanced - remove Google Account from GmsCore.
5. Open Google Photos ReVanced on your device and log into your account.
6. One or more identical GmsCore logs should appear in the terminal.
7. Copy text from `androidId=` to the end of the line from any log.
8. That's it! 🎉

</details>

### Option 3 - Official apk. Root required

<details>
  <summary><strong>Click to expand</strong></summary>

1. Get a rooted android device or an emulator.
2. Connect the device to your PC via ADB.
3. Install [HTTP Toolkit](https://httptoolkit.com)
4. In HTTP Toolkit, select Intercept - `Android Device via ADB`. Filter traffic with

   ```text
   contains(https://www.googleapis.com/auth/photos.native)
   ```

   Or if you have an older version of Google Photos, try

   ```text
   contains(www.googleapis.com%2Fauth%2Fplus.photos.readwrite)
   ```

5. Open Google Photos app and login with your account.
6. A single request should appear.  
   Copy request body as text.
7. Add that credential string in gotohp.
8. If gotohp asks for a token binding key, keep the rooted device connected and click `Read from ADB`. gotohp will read the account's `lstBindingKeyAlias` from Android AccountManager and save it into the credential.

#### Troubleshooting

- **No Auth Request Intercepted**
  1. Log out of your Google account.
  2. Log in again.
  3. Try `Android App via Frida` interception method in HTTP Toolkit.
- **Token binding key not found**
  1. Make sure the same Google account is present on the connected device.
  2. Make sure root is available to ADB.

</details>

## Build

Follow official wails3 guide
[https://v3.wails.io/getting-started/installation/](https://v3.wails.io/getting-started/installation/)
