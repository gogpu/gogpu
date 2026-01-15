module github.com/gogpu/gogpu

go 1.25

require (
	github.com/go-webgpu/webgpu v0.1.4
	github.com/gogpu/wgpu v0.9.3
	golang.org/x/sys v0.39.0
)

require github.com/go-webgpu/goffi v0.3.7

require (
	github.com/gogpu/gg v0.17.1 // indirect
	github.com/gogpu/naga v0.8.4 // indirect
	golang.org/x/image v0.34.0 // indirect
	golang.org/x/text v0.32.0 // indirect
)

// Use local version for development
replace github.com/gogpu/gg => ../gg
