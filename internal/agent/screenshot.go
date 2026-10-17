// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/jpeg"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// MaxShotWidth is the widest screenshot returned, in pixels.
const MaxShotWidth = 1280

// Screenshot returns a JPEG of what the tab shows. The browser takes it at full
// size and it is scaled down here: a clip or scale would have Chromium change
// the page's metrics, which the page can notice.
func (b *Browser) Screenshot(ctx context.Context, id string) ([]byte, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := b.activate(ctx, t); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var r struct {
		Data string `json:"data"`
	}
	if err := b.call(ctx, t.session, "Page.captureScreenshot", map[string]any{"format": "jpeg", "quality": 85}, &r); err != nil {
		if ctx.Err() != nil {
			return nil, apperr.New("agent_screenshot", "the window did not draw in 15 s; is it minimised?")
		}
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(r.Data)
	if err != nil {
		return nil, err
	}
	return shrink(raw, MaxShotWidth)
}

// shrink scales a JPEG down to width by averaging pixel boxes, and encodes it
// at quality 70.
func shrink(raw []byte, width int) ([]byte, error) {
	src, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	sb := src.Bounds()
	if sb.Dx() <= width {
		var out bytes.Buffer
		err := jpeg.Encode(&out, src, &jpeg.Options{Quality: 70})
		return out.Bytes(), err
	}
	height := sb.Dy() * width / sb.Dx()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		y0, y1 := sb.Min.Y+y*sb.Dy()/height, sb.Min.Y+(y+1)*sb.Dy()/height
		for x := 0; x < width; x++ {
			x0, x1 := sb.Min.X+x*sb.Dx()/width, sb.Min.X+(x+1)*sb.Dx()/width
			var r, g, bl, n uint32
			for yy := y0; yy < max(y1, y0+1); yy++ {
				for xx := x0; xx < max(x1, x0+1); xx++ {
					cr, cg, cb, _ := src.At(xx, yy).RGBA()
					r, g, bl, n = r+cr, g+cg, bl+cb, n+1
				}
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = uint8(r/n>>8), uint8(g/n>>8), uint8(bl/n>>8), 255
		}
	}
	var out bytes.Buffer
	err = jpeg.Encode(&out, dst, &jpeg.Options{Quality: 70})
	return out.Bytes(), err
}
