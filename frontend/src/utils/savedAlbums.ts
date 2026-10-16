export interface SavedAlbum {
  albumId: string
  albumName: string
}

export type SavedAlbumSort = 'added' | 'alphabetical'

export function sortSavedAlbums(albums: SavedAlbum[], sortBy: SavedAlbumSort): SavedAlbum[] {
  if (sortBy === 'alphabetical') {
    return [...albums].sort((a, b) => a.albumName.localeCompare(b.albumName, undefined, { sensitivity: 'base' }))
  }
  return [...albums].reverse()
}

export function savedAlbumsExport(albums: SavedAlbum[]) {
  return {
    schemaVersion: 1,
    albums: albums.map(({ albumId, albumName }) => ({ albumId, albumName })),
  }
}
