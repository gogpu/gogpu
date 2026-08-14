package golden

import (
	"image"
	"image/color"
	"testing"

	"github.com/gogpu/gogpu"
)

func TestCompareRGBA(t *testing.T) {
	got := image.NewRGBA(image.Rect(0, 0, 2, 1))
	want := image.NewRGBA(image.Rect(0, 0, 2, 1))
	got.SetRGBA(1, 0, color.RGBA{R: 4, A: 255})
	want.SetRGBA(1, 0, color.RGBA{R: 2, A: 255})

	pct, count := compareRGBA(got, want)
	if pct != 50 || count != 1 {
		t.Fatalf("compareRGBA() = %.1f%%, %d; want 50%%, 1", pct, count)
	}
}

func TestValidate(t *testing.T) {
	draw := func(*gogpu.Context) {}
	for _, test := range []struct {
		name string
		want bool
	}{
		{name: "ok", want: true},
		{name: "../escape", want: false},
		{name: "", want: false},
	} {
		err := validate(test.name, 1, 1, draw, Options{})
		if (err == nil) != test.want {
			t.Errorf("validate(%q) error=%v, want valid=%v", test.name, err, test.want)
		}
	}
}
