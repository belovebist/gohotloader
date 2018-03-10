# Go Hot Loader

This is a hot-loader application built for go. While developing web applications or any other application that needs frequent changes and testing there's a need of auto-reloading when the code changes. This application helps with that.

## Installation:
go get github.com/belovebist/gohotloader

## Usage:
`gohotloader`

This shows basic help and a list of available parameters.

### Example usage:
`gohotloader -watch=app -build=app -stderrthreshold=INFO`

This example assumes the source code for your application exists in `$GOPATH/src/app` or `/go/src/app`. It watches the application directory "app". Whenever there is any change in that directory, the source specified by `-build=app` is rebuilt and application is started again. The executable is placed at `/tmp/hl_build` by default. You can override this by providing path to executable through parameter `-exec=[path_to_executable]`. The last option `-stderrthreshold=INFO` specifies the log level to be info. Other options for loglevel are `DEBUG` and `ERROR`

### General usage:
`gohotloader -watch=app,lib,tools -build=app -exec=/go/bin/myapp -stderrthreshold=DEBUG`

The argument `-watch` can take multiple directory names separated by comma. Watching multiple directories is useful when the application depends on other packages that may change while the application is running.

## Note:
The default executable is /tmp/hl_build. This should work fine on mac and linux. But for windows `-exec` parameter is required.

Also note that I've used this only on mac. So it may need further tests on linux and windows.