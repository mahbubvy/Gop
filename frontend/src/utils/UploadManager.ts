import { Clipboard, Events } from "@wailsio/runtime";
import { ConfigManager } from '../../bindings/app/backend';
import { reactive } from "vue";
import { recordAlbumProgress } from './uploadPresentation';
import { errorCategory, safeName } from './debugLog';
import {
  recordUploadResult,
  type UploadResults,
} from "./uploadResults";

export interface ThreadStatus {
  WorkerID: number;
  Status: string;
  FilePath: string;
  FileName: string;
  Message: string;
  BytesUploaded: number;
  BytesTotal: number;
  Attempt: number;
}

export interface FileUploadResult {
  Cancelled?: boolean;
  MediaKey: string;
  IsError: boolean;
  IsLivePhoto: boolean;
  Skipped: boolean;
  ErrorMessage: string;
  SkipCode: string;
  SkipReason: string;
  Path: string;
  Paths: string[];
}

export interface PreflightWarning {
  Paths: string[];
  Code: string;
  Message: string;
}

export interface UploadBatchStart {
  Total: number;
  TotalBytes: number;
  AlbumName: string;
  AlbumAutoMode: boolean;
  AutoAlbumFolders: string[];
  WorkPaths?: string[];
}

export interface AlbumStatus {
  AlbumName: string;
  ItemsAdded: number;
  TotalItems: number;
  AlbumKeys: string[];
  IsComplete: boolean;
}

export interface AlbumError {
  AlbumName: string;
  Error: string;
}

function isSkippedUploadWarning(code: string): boolean {
  return code === "incomplete-live-photo-skipped" || code === "ambiguous-filename-stem";
}

export interface UploadState {
  isUploading: boolean;
  totalFiles: number;
  uploadedFiles: number;
  threads: Map<number, ThreadStatus>;
  results: UploadResults;
  warnings: PreflightWarning[];
  // Byte tracking
  totalBytes: number;
  uploadedBytes: number;
  // Timing
  startTime: number;
  // Speed calculation (bytes per second)
  uploadSpeed: number;
  // Album creation
  albumStatus: AlbumStatus | null;
  isCreatingAlbum: boolean;
  // Destinations determined during preflight, shown while files are uploading.
  albumName: string;
  albumAutoMode: boolean;
  autoAlbumFolders: string[];
  // Every completed album operation in this batch, including existing albums.
  albumsUpdated: AlbumStatus[];
  albumErrors: AlbumError[];
  completionVisible: boolean;
  albumDisplayName: string;
  workPaths: string[];
  processedPaths: string[];
  cancelRequested: boolean;
  finishedAt: number;
  failures: { fileName: string; category: string }[];
}

class UploadManager {
  private static instance: UploadManager;

  // Reactive state that can be accessed by components
  public state = reactive<UploadState>({
    isUploading: false,
    totalFiles: 0,
    uploadedFiles: 0,
    threads: new Map<number, ThreadStatus>(),
    results: {
      success: [],
      fail: [],
      cancelled: [],
      skipped: [],
      warnings: [],
    },
    warnings: [],
    totalBytes: 0,
    uploadedBytes: 0,
    startTime: 0,
    uploadSpeed: 0,
    albumStatus: null,
    isCreatingAlbum: false,
    albumName: '',
    albumAutoMode: false,
    autoAlbumFolders: [],
    albumsUpdated: [],
    albumErrors: [],
    completionVisible: false,
    albumDisplayName: '',
    workPaths: [],
    processedPaths: [],
    cancelRequested: false,
    finishedAt: 0,
    failures: [],
  });

  // For speed calculation
  private lastSpeedUpdate: number = 0;
  private lastBytesUploaded: number = 0;
  private speedSamples: number[] = [];
  // Track bytes from completed files
  private completedBytes: number = 0;
  // Track the last known BytesTotal for each file
  private fileBytes: Map<string, number> = new Map();
  // Dedupe decisions can change the bytes that will actually be uploaded.
  private totalBytesAdjustment: number = 0;
  private pendingAlbumDisplayName = '';

  private constructor() {
    // Bind all methods to ensure 'this' context is preserved
    this.resetUploadResults = this.resetUploadResults.bind(this);
    this.cancelUpload = this.cancelUpload.bind(this);
    this.copyResultsAsJson = this.copyResultsAsJson.bind(this);
    this.dismissCompletion = this.dismissCompletion.bind(this);

    this.setupEventListeners();
  }

  public static getInstance(): UploadManager {
    if (!UploadManager.instance) {
      UploadManager.instance = new UploadManager();
    }
    return UploadManager.instance;
  }

  private setupEventListeners() {
    // Handle upload start
    Events.On("uploadStart", (event: { data: UploadBatchStart }) => {
      if (!this.state.isUploading) this.state.cancelRequested = false;
      this.state.workPaths = event.data.WorkPaths || [];
      this.state.processedPaths = [];
      this.state.finishedAt = 0;
      this.state.totalFiles = event.data.Total;
      this.state.totalBytes = event.data.TotalBytes; // May be 0 initially
      this.state.albumName = event.data.AlbumName || '';
      this.state.albumDisplayName = this.pendingAlbumDisplayName || friendlyAlbumName(event.data.AlbumName || '')
      this.state.albumAutoMode = !!event.data.AlbumAutoMode;
      this.state.autoAlbumFolders = event.data.AutoAlbumFolders || [];
      this.state.uploadedFiles = 0;
      this.state.uploadedBytes = 0;
      this.state.isUploading = true;
      this.state.threads.clear();
      this.state.startTime = Date.now();
      this.state.uploadSpeed = 0;
      this.lastSpeedUpdate = Date.now();
      this.lastBytesUploaded = 0;
      this.speedSamples = [];
      this.completedBytes = 0;
      this.fileBytes.clear();
      this.totalBytesAdjustment = 0;
      this.resetUploadResults();
      this.state.warnings = [];
      // Album events arrive after media uploads. Reset these first so a new
      // upload can never show the previous batch's album as its destination.
      this.state.albumStatus = null;
      this.state.isCreatingAlbum = false;
      this.state.albumsUpdated = [];
      this.state.albumErrors = [];
      this.state.completionVisible = false;
    });

    // Handle async total bytes update (calculated after uploadStart)
    Events.On("uploadTotalBytes", (event: { data: number }) => {
      this.state.totalBytes = Math.max(0, event.data + this.totalBytesAdjustment);
    });

    Events.On("uploadTotalBytesDelta", (event: { data: number }) => {
      this.totalBytesAdjustment += event.data;
      this.state.totalBytes = Math.max(0, this.state.totalBytes + event.data);
    });

    Events.On("uploadWarning", (event: { data: PreflightWarning }) => {
      this.state.warnings.push(event.data);
      if (!isSkippedUploadWarning(event.data.Code)) {
        this.state.results.warnings.push({
          paths: event.data.Paths,
          code: event.data.Code,
          reason: event.data.Message,
        });
      }
    });

    // Handle thread status updates
    Events.On("ThreadStatus", (event: { data: ThreadStatus }) => {
      const thread = event.data;
      const prevThread = this.state.threads.get(thread.WorkerID);

      if (thread.Status === 'uploading' && thread.FilePath && thread.BytesTotal > 0) {
        this.fileBytes.set(thread.FilePath, thread.BytesTotal);
      }

      if (thread.Status === 'error' && prevThread?.Message !== thread.Message && !(this.state.cancelRequested && errorCategory(thread.Message) === 'cancelled')) {
        window.dispatchEvent(new CustomEvent('uploadError', { detail: thread }));
      }
      
      this.state.threads.set(thread.WorkerID, thread);
      this.updateBytesAndSpeed();
    });

    // Handle file status updates
    Events.On("FileStatus", (event: { data: FileUploadResult }) => {
      const { Path, Paths } = event.data;
      if (event.data.IsError && !event.data.Cancelled) this.state.failures.push({ fileName: safeName(Path), category: errorCategory(event.data.ErrorMessage || '') });
      this.state.processedPaths.push(Path);

      this.state.uploadedFiles += recordUploadResult(this.state.results, event.data);
      if (!event.data.IsError) {
        const completedFileBytes = this.fileBytes.get(Path);
        if (completedFileBytes && completedFileBytes > 0) {
          this.completedBytes += completedFileBytes;
        }
      }
      for (const completedPath of Paths?.length ? Paths : [Path]) {
        this.fileBytes.delete(completedPath);
      }
      this.updateBytesAndSpeed();

      // Only uploadStop ends a batch; album assignment may still be running.
    });

    // Handle upload stop
    Events.On("uploadStop", () => {
      this.state.isUploading = false;
      this.state.completionVisible = true;
      this.state.finishedAt = Date.now();
      this.state.isCreatingAlbum = false;
      this.pendingAlbumDisplayName = '';
    });

    // Handle album creation progress
    Events.On("albumProgress", (event: { data: AlbumStatus }) => {
      this.state.albumStatus = { ...event.data, AlbumName: this.state.albumName.startsWith('AF1Qip') ? this.state.albumDisplayName : friendlyAlbumName(event.data.AlbumName) };
      recordAlbumProgress(this.state.albumsUpdated, this.state.albumStatus);
      this.state.isCreatingAlbum = true;
    });

    // Handle album creation complete
    Events.On("albumComplete", (event: { data: AlbumStatus }) => {
      this.state.albumStatus = { ...event.data, AlbumName: this.state.albumAutoMode ? event.data.AlbumName : this.state.albumDisplayName || friendlyAlbumName(event.data.AlbumName) };
      this.state.isCreatingAlbum = false;
      if (!event.data.AlbumKeys.some(key => this.state.albumsUpdated.some(album => album.AlbumKeys.includes(key)))) {
        recordAlbumProgress(this.state.albumsUpdated, this.state.albumStatus);
      }
      for (const album of this.state.albumsUpdated) {
        if (event.data.AlbumKeys.includes(album.AlbumKeys[0])) album.IsComplete = true;
      }

      // Every successful album operation is a useful shortcut. Auto Album
      // entries use their folder-derived album names, not opaque album keys.
      if (event.data.ItemsAdded > 0) {
        for (const albumID of event.data.AlbumKeys) {
          const displayName = this.state.albumsUpdated.find(album => album.AlbumKeys.includes(albumID))?.AlbumName || this.state.albumDisplayName || friendlyAlbumName(event.data.AlbumName)
          void ConfigManager.TouchRecentAlbum(albumID, displayName)
        }
      }
      const operation = this.state.albumAutoMode || !this.state.albumName.startsWith('AF1Qip') ? 'Created' : 'Updated'
      for (const albumID of event.data.AlbumKeys) {
        const album = this.state.albumsUpdated.find(album => album.AlbumKeys.includes(albumID))
        void ConfigManager.AddAlbumHistory(albumID, album?.AlbumName || this.state.albumDisplayName || friendlyAlbumName(event.data.AlbumName), album?.ItemsAdded ?? event.data.ItemsAdded, operation)
      }
    });

    // Handle album creation error
    Events.On("albumError", (event: { data: AlbumError }) => {
      this.state.isCreatingAlbum = false;
      this.state.albumErrors.push({ ...event.data, AlbumName: this.state.albumAutoMode ? event.data.AlbumName : this.state.albumDisplayName || friendlyAlbumName(event.data.AlbumName) });
      if (!this.state.albumAutoMode && this.state.albumName && event.data.Error.includes('404')) {
        void ConfigManager.RemoveRecentAlbum(this.state.albumName)
      }
      // Emit a custom event that App.vue can listen to for showing toast
      window.dispatchEvent(new CustomEvent('albumError', { detail: this.state.albumErrors[this.state.albumErrors.length - 1] }));
    });
  }

  private updateBytesAndSpeed() {
    // Calculate bytes from currently active uploads
    let activeUploadedBytes = 0;

    this.state.threads.forEach((thread) => {
      if (thread.Status === 'uploading' && thread.BytesTotal > 0) {
        activeUploadedBytes += thread.BytesUploaded;
      } else if (thread.Status === 'finalizing' && thread.FilePath) {
        activeUploadedBytes += this.fileBytes.get(thread.FilePath) ?? 0;
      }
    });

    // Total uploaded = completed files + current progress
    const totalUploaded = this.completedBytes + activeUploadedBytes;
    this.state.uploadedBytes = totalUploaded;

    // Calculate speed (using rolling average)
    const now = Date.now();
    const timeDelta = now - this.lastSpeedUpdate;

    if (timeDelta >= 500) { // Update speed every 500ms
      const bytesDelta = totalUploaded - this.lastBytesUploaded;
      const instantSpeed = (bytesDelta / timeDelta) * 1000; // bytes per second

      if (instantSpeed >= 0) {
        this.speedSamples.push(instantSpeed);
        // Keep last 5 samples for smoothing
        if (this.speedSamples.length > 5) {
          this.speedSamples.shift();
        }
        // Calculate average speed
        this.state.uploadSpeed = this.speedSamples.reduce((a, b) => a + b, 0) / this.speedSamples.length;
      }

      this.lastSpeedUpdate = now;
      this.lastBytesUploaded = totalUploaded;
    }
  }

  public resetUploadResults() {
    this.state.failures = [];
    this.state.results.success = [];
    this.state.results.fail = [];
    this.state.results.cancelled = [];
    this.state.results.skipped = [];
    this.state.results.warnings = [];
  }

  public cancelUpload() {
    this.state.cancelRequested = true;
    Events.Emit("uploadCancel");
  }

  public dismissCompletion() {
    this.state.completionVisible = false;
  }

  public setNextAlbumDisplayName(name: string) {
    this.pendingAlbumDisplayName = name.trim()
  }

  public async copyResultsAsJson() {
    const resultsJson = JSON.stringify(this.state.results, null, 2);
    try {
      await Clipboard.SetText(resultsJson);
      console.log("Upload results copied to clipboard");
      return true;
    } catch (error) {
      console.error("Failed to copy results:", error);
      return false;
    }
  }
}

function friendlyAlbumName(name: string): string {
  return name.startsWith('AF1Qip') || name.startsWith('Album (AF1Qip') ? 'Saved album' : name
}

// Create and export a single instance
export const uploadManager = UploadManager.getInstance();
