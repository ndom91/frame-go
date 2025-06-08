module photo-frame

go 1.21

require (
	fyne.io/fyne/v2 v2.6.1
	fyne.io/systray v1.11.0
	github.com/BurntSushi/toml v1.4.0
	github.com/davecgh/go-spew v1.1.1
	github.com/fredbi/uri v1.1.0
	github.com/fsnotify/fsnotify v1.7.0
	github.com/fyne-io/gl-js v0.1.0
	github.com/fyne-io/glfw-js v0.2.0
	github.com/fyne-io/image v0.1.1
	github.com/fyne-io/oksvg v0.1.0
	github.com/go-gl/gl v0.0.0-20231021071112-07e5d0ea2e71
	github.com/go-gl/glfw/v3.3/glfw v0.0.0-20240506104042-037f3cc74f2a
	github.com/go-text/render v0.2.0
	github.com/go-text/typesetting v0.2.1
	github.com/godbus/dbus/v5 v5.1.0
	github.com/hack-pad/go-indexeddb v0.3.2
	github.com/hack-pad/safejs v0.1.0
	github.com/jeandeaual/go-locale v0.0.0-20241217141322-fcc2cadd6f08
	github.com/jsummers/gobmp v0.0.0-20230614200233-a9de23ed2e25
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/nicksnyder/go-i18n/v2 v2.5.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/rymdport/portal v0.4.1
	github.com/srwiley/oksvg v0.0.0-20221011165216-be6e8873101c
	github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef
	github.com/stretchr/testify v1.10.0
	github.com/yuin/goldmark v1.7.8
	golang.org/x/image v0.24.0
	golang.org/x/net v0.35.0
	golang.org/x/sys v0.30.0
	golang.org/x/text v0.22.0
	gopkg.in/yaml.v3 v3.0.1
)

require github.com/kr/text v0.2.0 // indirect

// Additional dependencies will be added when you implement S3 sync:
// require (
//     github.com/aws/aws-sdk-go-v2 v1.21.0
//     github.com/aws/aws-sdk-go-v2/service/s3 v1.38.5
// )
