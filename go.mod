module github.com/gogpu/gogpu

go 1.25

require (
	github.com/go-webgpu/webgpu v0.1.0
	github.com/gogpu/wgpu v0.1.0-alpha
	golang.org/x/sys v0.39.0
)

require github.com/go-webgpu/goffi v0.3.1 // indirect

// Local development: use local wgpu module
replace github.com/gogpu/wgpu => ../wgpu
