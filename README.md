# Photo Backup

Wails desktop app for copying photos from a flash drive or phone, cataloguing them
in SQLite, and searching the collection by description or tag.

## Backend

- Go + Wails v2 provide the desktop API and folder picker.
- GORM with SQLite stores backup records and many-to-many tags in
  `backup_history.db` under the current user's configuration folder
  (`%AppData%\PhotoBackup` on Windows and `~/Library/Application Support/PhotoBackup`
  on macOS).
- Photo files are copied into the selected destination, retaining their relative
  folder structure. JPEG, PNG, and GIF files are analyzed for dimensions, hashed
  with SHA-256, and skipped when their content has already been backed up.
- The backup screen supports choosing a whole source folder or selecting one or
  more image files; selected files are copied directly into the destination.
- The library supports listing, tag/description search, metadata editing, and
  deletion of both the catalog record and its backed-up file.
- Image data is served through the Wails asset server at `/image/{id}`; add
  `?thumb=1` for a 400-by-300 maximum JPEG thumbnail.

## Wails API

`SelectFolder`, `SelectImages`, `StartBackup`, `StartBackupFiles`, `ListImages`, `SearchByTag`,
`SearchByDescription`, `TagCloud`, `UpdateImage`, and `DeleteImage` are exposed
to the frontend. Backup progress is emitted as the `backup:progress` event.

The browser-only Nuxt mode uses mock data; use `wails dev` to exercise the Go
backend and native folder picker.

## Live Development

Run `wails dev` in the project directory. This starts the Vite server with hot
reload and exposes the Wails Go bindings to the frontend.

## Building

Run `wails build` to create a redistributable desktop application.
