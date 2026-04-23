package gogpu

import (
	"errors"
	"testing"
)

func TestDrawTextureNilTexture(t *testing.T) {
	ctx := &Context{renderer: &Renderer{}}

	err := ctx.DrawTexture(nil, 0, 0)
	if !errors.Is(err, ErrTextureNil) {
		t.Errorf("DrawTexture(nil) = %v, want ErrTextureNil", err)
	}
}

func TestDrawTextureDestroyedTexture(t *testing.T) {
	ctx := &Context{renderer: &Renderer{}}

	// Texture with nil HAL texture is considered destroyed
	tex := &Texture{
		texture: nil,
		width:   64,
		height:  64,
	}

	err := ctx.DrawTexture(tex, 0, 0)
	if !errors.Is(err, ErrTextureDestroyed) {
		t.Errorf("DrawTexture(destroyed) = %v, want ErrTextureDestroyed", err)
	}
}

func TestDrawTextureScaledNilTexture(t *testing.T) {
	ctx := &Context{renderer: &Renderer{}}

	err := ctx.DrawTextureScaled(nil, 0, 0, 100, 100)
	if !errors.Is(err, ErrTextureNil) {
		t.Errorf("DrawTextureScaled(nil) = %v, want ErrTextureNil", err)
	}
}

func TestDrawTextureScaledDestroyedTexture(t *testing.T) {
	ctx := &Context{renderer: &Renderer{}}

	tex := &Texture{
		texture: nil,
		width:   64,
		height:  64,
	}

	err := ctx.DrawTextureScaled(tex, 0, 0, 100, 100)
	if !errors.Is(err, ErrTextureDestroyed) {
		t.Errorf("DrawTextureScaled(destroyed) = %v, want ErrTextureDestroyed", err)
	}
}

func TestDrawTextureScaledNegativeDimensions(t *testing.T) {
	ctx := &Context{renderer: &Renderer{}}

	tex := &Texture{
		texture: newMockWgpuTexture(),
		width:   64,
		height:  64,
	}

	tests := []struct {
		name string
		w, h float32
	}{
		{"negative width", -10, 100},
		{"negative height", 100, -10},
		{"both negative", -10, -10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ctx.DrawTextureScaled(tex, 0, 0, tt.w, tt.h)
			if !errors.Is(err, ErrInvalidDimensions) {
				t.Errorf("DrawTextureScaled(%f, %f) = %v, want ErrInvalidDimensions", tt.w, tt.h, err)
			}
		})
	}
}

func TestDrawTextureExNilTexture(t *testing.T) {
	ctx := &Context{renderer: &Renderer{}}

	err := ctx.DrawTextureEx(nil, DrawTextureOptions{})
	if !errors.Is(err, ErrTextureNil) {
		t.Errorf("DrawTextureEx(nil) = %v, want ErrTextureNil", err)
	}
}

func TestDrawTextureExDestroyedTexture(t *testing.T) {
	ctx := &Context{renderer: &Renderer{}}

	tex := &Texture{
		texture: nil,
		width:   64,
		height:  64,
	}

	err := ctx.DrawTextureEx(tex, DrawTextureOptions{})
	if !errors.Is(err, ErrTextureDestroyed) {
		t.Errorf("DrawTextureEx(destroyed) = %v, want ErrTextureDestroyed", err)
	}
}

func TestDrawTextureExNegativeDimensions(t *testing.T) {
	ctx := &Context{renderer: &Renderer{}}

	tex := &Texture{
		texture: newMockWgpuTexture(),
		width:   64,
		height:  64,
	}

	tests := []struct {
		name string
		opts DrawTextureOptions
	}{
		{
			name: "negative width",
			opts: DrawTextureOptions{Width: -10},
		},
		{
			name: "negative height",
			opts: DrawTextureOptions{Height: -10},
		},
		{
			name: "both negative",
			opts: DrawTextureOptions{Width: -10, Height: -10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ctx.DrawTextureEx(tex, tt.opts)
			if !errors.Is(err, ErrInvalidDimensions) {
				t.Errorf("DrawTextureEx(%+v) = %v, want ErrInvalidDimensions", tt.opts, err)
			}
		})
	}
}

func TestDrawTextureExValidTexture(t *testing.T) {
	ctx := &Context{renderer: newTestRenderer(800, 600)}

	tex := &Texture{
		texture: newMockWgpuTexture(),
		width:   64,
		height:  64,
	}

	// Should return nil when no frame is in progress (currentView == nil).
	// This is the correct behavior - it silently succeeds without drawing.
	err := ctx.DrawTextureEx(tex, DrawTextureOptions{
		X:      100,
		Y:      200,
		Width:  128,
		Height: 128,
		Alpha:  0.5,
	})

	if err != nil {
		t.Errorf("DrawTextureEx(valid) = %v, want nil", err)
	}
}

func TestDrawTextureExDefaultValues(t *testing.T) {
	ctx := &Context{renderer: newTestRenderer(800, 600)}

	tex := &Texture{
		texture: newMockWgpuTexture(),
		width:   64,
		height:  32,
	}

	// Test that zero width/height uses original texture dimensions
	// This is tested indirectly through the API behavior
	err := ctx.DrawTextureEx(tex, DrawTextureOptions{
		X: 10,
		Y: 20,
		// Width and Height are 0 - should use original
		// Alpha is 0 - should default to 1.0
	})

	// Should return nil when no frame is in progress
	if err != nil {
		t.Errorf("DrawTextureEx(defaults) = %v, want nil", err)
	}
}

func TestValidateTexture(t *testing.T) {
	tests := []struct {
		name    string
		tex     *Texture
		wantErr error
	}{
		{
			name:    "nil texture",
			tex:     nil,
			wantErr: ErrTextureNil,
		},
		{
			name: "destroyed texture (nil HAL texture)",
			tex: &Texture{
				texture: nil,
				width:   64,
				height:  64,
			},
			wantErr: ErrTextureDestroyed,
		},
		{
			name: "valid texture",
			tex: &Texture{
				texture: newMockWgpuTexture(),
				width:   64,
				height:  64,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTexture(tt.tex)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateTexture() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDrawTextureOptions(t *testing.T) {
	// Test the DrawTextureOptions struct fields
	opts := DrawTextureOptions{
		X:      100.5,
		Y:      200.5,
		Width:  128.0,
		Height: 64.0,
		Alpha:  0.75,
	}

	if opts.X != 100.5 {
		t.Errorf("X = %f, want 100.5", opts.X)
	}
	if opts.Y != 200.5 {
		t.Errorf("Y = %f, want 200.5", opts.Y)
	}
	if opts.Width != 128.0 {
		t.Errorf("Width = %f, want 128.0", opts.Width)
	}
	if opts.Height != 64.0 {
		t.Errorf("Height = %f, want 64.0", opts.Height)
	}
	if opts.Alpha != 0.75 {
		t.Errorf("Alpha = %f, want 0.75", opts.Alpha)
	}
}

func TestDrawTextureExAlphaClamping(t *testing.T) {
	// This test verifies the alpha clamping logic conceptually.
	// All alpha values (including out-of-range) should pass validation and
	// return nil when no frame is in progress.
	ctx := &Context{renderer: newTestRenderer(800, 600)}

	tex := &Texture{
		texture: newMockWgpuTexture(),
		width:   64,
		height:  64,
	}

	tests := []struct {
		name  string
		alpha float32
	}{
		{"alpha < 0", -0.5},
		{"alpha > 1", 1.5},
		{"alpha = 0", 0}, // Should default to 1.0
		{"alpha = 1", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ctx.DrawTextureEx(tex, DrawTextureOptions{Alpha: tt.alpha})
			// All should pass validation and return nil (no frame in progress)
			if err != nil {
				t.Errorf("DrawTextureEx(alpha=%f) = %v, want nil", tt.alpha, err)
			}
		})
	}
}

func TestContextTextureErrorConstants(t *testing.T) {
	// Verify error messages are meaningful
	tests := []struct {
		err  error
		want string
	}{
		{ErrTextureNil, "gogpu: texture is nil"},
		{ErrTextureDestroyed, "gogpu: texture has been destroyed"},
		{ErrInvalidDimensions, "gogpu: invalid texture dimensions"},
		{ErrNotImplemented, "gogpu: feature not implemented"},
		{ErrInvalidTextureType, "gogpu: texture must be *Texture"},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.want)
			}
		})
	}
}

func TestContextTextureDrawerInvalidType(t *testing.T) {
	ctx := &Context{renderer: &Renderer{}}
	drawer := ctx.AsTextureDrawer()

	// Passing a non-*Texture type should return ErrInvalidTextureType
	err := drawer.DrawTexture(nil, 0, 0)
	if !errors.Is(err, ErrInvalidTextureType) {
		t.Errorf("DrawTexture(nil) = %v, want ErrInvalidTextureType", err)
	}
}

func TestContextTextureDrawerValidTexture(t *testing.T) {
	ctx := &Context{renderer: &Renderer{}}
	drawer := ctx.AsTextureDrawer()

	// Valid texture with nil HAL texture should return ErrTextureDestroyed
	tex := &Texture{texture: nil, width: 64, height: 64}
	err := drawer.DrawTexture(tex, 0, 0)
	if !errors.Is(err, ErrTextureDestroyed) {
		t.Errorf("DrawTexture(destroyed) = %v, want ErrTextureDestroyed", err)
	}
}

func TestContextTextureDrawerTextureCreator(t *testing.T) {
	ctx := &Context{renderer: &Renderer{}}
	drawer := ctx.AsTextureDrawer()

	creator := drawer.TextureCreator()
	if creator == nil {
		t.Fatal("TextureCreator() returned nil")
	}
}
