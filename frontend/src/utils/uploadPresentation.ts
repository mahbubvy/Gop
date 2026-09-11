export function mediaLabel(paths: string[]): string {
  if (!paths.length) return 'files'
  const videos = /\.(mp4|mov|m4v|avi|mkv|webm|mpg|mpeg|mts|m2ts|3gp)$/i
  const photos = /\.(jpg|jpeg|png|gif|webp|heic|heif|avif|bmp|tif|tiff|dng|cr2|nef|arw)$/i
  if (paths.every(path => videos.test(path))) return 'videos'
  if (paths.every(path => photos.test(path))) return 'photos'
  return 'files'
}

export function resultHeading(cancelled: boolean, uploaded: number, failed: number, albumErrors: number, skipped: number): string {
  if (cancelled) return 'Upload cancelled'
  if (failed && !uploaded && !skipped) return 'Upload failed'
  if (failed || albumErrors) return 'Upload finished with issues'
  if (!uploaded) return skipped ? 'Files skipped' : 'No files uploaded'
  return 'Upload complete'
}

export function folderCounts(folder: string, workPaths: string[], processedPaths: string[]) {
  const normalize = (path: string) => path.replace(/\\/g, '/').replace(/\/$/, '')
  const paths = workPaths.filter(path => normalize(path).slice(0, normalize(path).lastIndexOf('/')) === normalize(folder))
  const completed = new Set(processedPaths)
  return { total: paths.length, processed: paths.filter(path => completed.has(path)).length }
}

// Album progress is cumulative across split albums. Keep each actual album's
// confirmed additions, including partial additions before an error/cancel.
export function recordAlbumProgress<T extends { AlbumKeys: string[]; AlbumName: string; ItemsAdded: number; TotalItems: number; IsComplete: boolean }>(albums: T[], event: T) {
  const key = event.AlbumKeys.at(-1)
  if (!key || event.ItemsAdded <= 0) return
  const previous = albums.filter(album => event.AlbumKeys.slice(0, -1).includes(album.AlbumKeys[0]))
  const added = event.ItemsAdded - previous.reduce((sum, album) => sum + album.ItemsAdded, 0)
  const index = albums.findIndex(album => album.AlbumKeys[0] === key)
  const next = { ...event, AlbumKeys: [key], ItemsAdded: Math.max(0, added) }
  if (index < 0) albums.push(next)
  else albums[index] = next
}
