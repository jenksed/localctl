# Darwin build regression note

The infrastructure catalog originally used the filename `exercise_catalog_linux.go`.

In Go, a filename ending in `_linux.go` is compiled only when `GOOS=linux`. That meant the Ubuntu CI job included the Linux investigation catalog and its shared `infraExact`/`infraContains`/`infraManual` helpers, while macOS correctly excluded the file and failed to compile Docker/Kubernetes catalogs that referenced those helpers.

The catalog now lives in `exercise_catalog_linux_cases.go`, which is ordinary cross-platform Go source. CI also performs a `GOOS=darwin GOARCH=arm64 go build ./...` check so an Ubuntu-only pass cannot hide this class of regression again.
