export function isAlbumIdentifier(value: string): boolean {
  return value.length > 6 && value.startsWith('AF1Qip')
}

function readableName(value: string): string {
  const name = value.trim()
  return name && name !== 'Saved album' && !isAlbumIdentifier(name) ? name : ''
}

export function findAlbumDisplayName(
  albumID: string,
  recent: { albumId: string; albumName: string }[],
  history: { albumId: string; albumName: string }[],
  saved: { albumId: string; albumName: string }[] = [],
): string {
  return readableName(saved.find(album => album.albumId === albumID)?.albumName || '')
    || readableName(recent.find(album => album.albumId === albumID)?.albumName || '')
    || readableName(history.find(album => album.albumId === albumID)?.albumName || '')
}
