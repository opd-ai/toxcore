//go:build cgo && libvpx
// +build cgo,libvpx

// Package video provides video processing capabilities for ToxAV.
//
// This file provides the CGo-based libvpx VP8 encoder factory when built
// with the 'libvpx' build tag. This enables full VP8 encoding with P-frames.
//
// Build with:
//
//	go build -tags libvpx ./...
//
// Prerequisites:
//   - libvpx-dev (Debian/Ubuntu) or libvpx (Homebrew/macOS)
//   - CGo enabled (default, unless CGO_ENABLED=0)
package video

/*
#cgo pkg-config: vpx
#include <vpx/vp8cx.h>
#include <vpx/vpx_encoder.h>

// pktBuf returns a pointer to the compressed frame data in the packet.
static void* pktBuf(vpx_codec_cx_pkt_t* p) { return p->data.frame.buf; }

// pktSz returns the byte size of the compressed frame in the packet.
static size_t pktSz(vpx_codec_cx_pkt_t* p) { return p->data.frame.sz; }

// pktIsKey returns 1 when the packet contains a VP8 key frame (I-frame).
static int pktIsKey(vpx_codec_cx_pkt_t* p) {
    return (p->data.frame.flags & VPX_FRAME_IS_KEY) ? 1 : 0;
}
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/sirupsen/logrus"
	vpx "github.com/xlab/libvpx-go/vpx"
)

// libvpxBitsPerKilobit converts bits-per-second to kbps for libvpx.
const libvpxBitsPerKilobit = 1000

// libvpxTimebaseNum is the numerator for the 1/30 timebase (30fps).
const libvpxTimebaseNum = 1

// libvpxTimebaseDen is the denominator for the 1/30 timebase (30fps).
const libvpxTimebaseDen = 30

// libvpxDefaultKFMaxDist is the default maximum distance between keyframes.
const libvpxDefaultKFMaxDist = 30

// LibVPXEncoder wraps the libvpx VP8 encoder for full VP8 support with P-frames.
//
// This encoder requires CGo and libvpx to be installed. It produces
// RFC 6386 compliant VP8 bitstreams with both I-frames and P-frames.
type LibVPXEncoder struct {
	encoder       *vpx.CodecCtx
	cfg           *vpx.CodecEncCfg
	img           *vpx.Image
	pts           vpx.CodecPts
	bitRate       uint32
	width         uint16
	height        uint16
	keyFrame      bool
	kfMaxDist     uint32
}

// NewLibVPXEncoder creates a new VP8 encoder using libvpx.
//
// This encoder supports:
//   - I-frames (key frames) and P-frames (predicted frames)
//   - Configurable keyframe interval
//   - Rate control for bandwidth targeting
//   - Real-time encoding mode optimized for video calls
//
// Parameters:
//   - width, height: Frame dimensions (must be positive)
//   - bitRate: Target encoding bit rate in bits per second
//
// Returns an error if libvpx initialization fails.
func NewLibVPXEncoder(width, height uint16, bitRate uint32) (*LibVPXEncoder, error) {
	cfg := &vpx.CodecEncCfg{}
	if err := vpx.Error(vpx.CodecEncConfigDefault(vpx.EncoderIfaceVP8(), cfg, 0)); err != nil {
		return nil, fmt.Errorf("libvpx config default: %w", err)
	}
	cfg.GW = uint32(width)
	cfg.GH = uint32(height)
	cfg.RcTargetBitrate = bitRate / libvpxBitsPerKilobit
	cfg.GTimebase.Num = libvpxTimebaseNum
	cfg.GTimebase.Den = libvpxTimebaseDen
	cfg.RcEndUsage = vpx.Vbr
	cfg.KfMode = vpx.KfAuto
	cfg.KfMaxDist = libvpxDefaultKFMaxDist

	ctx := &vpx.CodecCtx{}
	if err := vpx.Error(vpx.CodecEncInitVer(ctx, vpx.EncoderIfaceVP8(), cfg, 0, vpx.EncoderABIVersion)); err != nil {
		return nil, fmt.Errorf("libvpx init: %w", err)
	}

	img := &vpx.Image{}
	allocImg := vpx.ImageAlloc(img, vpx.ImageFormatI420, uint32(width), uint32(height), 1)
	if allocImg == nil {
		vpx.CodecDestroy(ctx)
		return nil, fmt.Errorf("libvpx image alloc failed for %dx%d", width, height)
	}
	// Populate Go-side Planes and Stride fields from the C struct.
	img.Deref()

	logrus.WithFields(logrus.Fields{
		"function": "NewLibVPXEncoder",
		"width":    width,
		"height":   height,
		"bit_rate": bitRate,
	}).Info("Created libvpx VP8 encoder with P-frame support")

	return &LibVPXEncoder{
		encoder:   ctx,
		cfg:       cfg,
		img:       img,
		bitRate:   bitRate,
		width:     width,
		height:    height,
		keyFrame:  true,
		kfMaxDist: libvpxDefaultKFMaxDist,
	}, nil
}

// Encode encodes a YUV420 video frame into a VP8 bitstream.
//
// Unlike the pure-Go encoder, this can produce both I-frames and P-frames.
// Keyframes are inserted periodically based on the kfMaxDist configuration.
func (e *LibVPXEncoder) Encode(frame *VideoFrame) ([]byte, error) {
	if frame == nil {
		return nil, fmt.Errorf("frame cannot be nil")
	}
	if frame.Width != e.width || frame.Height != e.height {
		return nil, fmt.Errorf("frame size mismatch: expected %dx%d, got %dx%d",
			e.width, e.height, frame.Width, frame.Height)
	}

	if err := e.fillImage(frame); err != nil {
		return nil, err
	}

	flags := vpx.EncFrameFlags(0)
	if e.keyFrame {
		flags = vpx.EflagForceKf
		e.keyFrame = false
	}

	if err := vpx.Error(vpx.CodecEncode(e.encoder, e.img, e.pts, 1, flags, vpx.DlRealtime)); err != nil {
		return nil, fmt.Errorf("libvpx encode: %w", err)
	}
	e.pts++

	return e.drainPackets(), nil
}

// fillImage copies YUV planes from a VideoFrame into the pre-allocated vpx.Image.
func (e *LibVPXEncoder) fillImage(frame *VideoFrame) error {
	w := int(e.width)
	h := int(e.height)
	uvW := w / 2
	uvH := h / 2
	if err := copyPlane(e.img.Planes[vpx.PlaneY], int(e.img.Stride[vpx.PlaneY]),
		frame.Y, frame.YStride, w, h); err != nil {
		return fmt.Errorf("libvpx fill Y plane: %w", err)
	}
	if err := copyPlane(e.img.Planes[vpx.PlaneU], int(e.img.Stride[vpx.PlaneU]),
		frame.U, frame.UStride, uvW, uvH); err != nil {
		return fmt.Errorf("libvpx fill U plane: %w", err)
	}
	if err := copyPlane(e.img.Planes[vpx.PlaneV], int(e.img.Stride[vpx.PlaneV]),
		frame.V, frame.VStride, uvW, uvH); err != nil {
		return fmt.Errorf("libvpx fill V plane: %w", err)
	}
	return nil
}

// copyPlane copies pixel rows from a source Go slice into a C plane pointer,
// respecting independent source and destination strides.
func copyPlane(dst *byte, dstStride int, src []byte, srcStride, width, height int) error {
	if src == nil {
		return fmt.Errorf("nil source plane")
	}
	dstSlice := (*[1 << 30]byte)(unsafe.Pointer(dst))
	effectiveSrcStride := srcStride
	if effectiveSrcStride == 0 {
		effectiveSrcStride = width
	}
	for row := range height {
		srcOff := row * effectiveSrcStride
		dstOff := row * dstStride
		if srcOff+width > len(src) {
			return fmt.Errorf("source plane too small at row %d", row)
		}
		copy(dstSlice[dstOff:dstOff+width], src[srcOff:srcOff+width])
	}
	return nil
}

// drainPackets collects all compressed output packets from the encoder and
// returns their concatenated frame data.
func (e *LibVPXEncoder) drainPackets() []byte {
	var out []byte
	var iter vpx.CodecIter
	for {
		pkt := vpx.CodecGetCxData(e.encoder, &iter)
		if pkt == nil {
			break
		}
		if pkt.Kind != vpx.CodecCxFramePkt {
			continue
		}
		cPkt := (*C.vpx_codec_cx_pkt_t)(unsafe.Pointer(pkt.Ref()))
		sz := int(C.pktSz(cPkt))
		if sz == 0 {
			continue
		}
		buf := (*[1 << 30]byte)(C.pktBuf(cPkt))[:sz:sz]
		out = append(out, buf...)
	}
	return out
}

// SetBitRate updates the target encoding bit rate and reconfigures the encoder.
func (e *LibVPXEncoder) SetBitRate(bitRate uint32) error {
	e.bitRate = bitRate
	e.cfg.RcTargetBitrate = bitRate / libvpxBitsPerKilobit
	if err := vpx.Error(vpx.CodecEncConfigSet(e.encoder, e.cfg)); err != nil {
		return fmt.Errorf("libvpx set bitrate: %w", err)
	}
	return nil
}

// SupportsInterframe returns true because libvpx supports P-frames.
func (e *LibVPXEncoder) SupportsInterframe() bool {
	return true
}

// SetKeyFrameInterval configures the maximum number of inter frames between
// key frames. A value of 0 means every frame is a key frame.
func (e *LibVPXEncoder) SetKeyFrameInterval(interval int) {
	if interval <= 0 {
		e.kfMaxDist = 1
	} else {
		e.kfMaxDist = uint32(interval)
	}
	e.cfg.KfMaxDist = e.kfMaxDist
	vpx.CodecEncConfigSet(e.encoder, e.cfg) //nolint:errcheck — best-effort config update
}

// ForceKeyFrame causes the next Encode call to produce a key frame.
func (e *LibVPXEncoder) ForceKeyFrame() {
	e.keyFrame = true
}

// Close releases encoder resources.
func (e *LibVPXEncoder) Close() error {
	if e.img != nil {
		vpx.ImageFree(e.img)
		e.img = nil
	}
	if e.encoder != nil {
		if err := vpx.Error(vpx.CodecDestroy(e.encoder)); err != nil {
			return fmt.Errorf("libvpx destroy: %w", err)
		}
		e.encoder = nil
	}
	return nil
}

// NewDefaultEncoder creates a VP8 encoder using libvpx for full VP8 support.
//
// This encoder produces both I-frames and P-frames for efficient bandwidth usage.
// This implementation is used when built with '-tags libvpx'.
func NewDefaultEncoder(width, height uint16, bitRate uint32) (Encoder, error) {
	return NewLibVPXEncoder(width, height, bitRate)
}

// DefaultEncoderSupportsInterframe returns whether the default encoder
// supports inter-frame prediction (P-frames).
//
// In CGo+libvpx builds, this returns true.
func DefaultEncoderSupportsInterframe() bool {
	return true
}

// DefaultEncoderName returns a human-readable name for the default encoder.
func DefaultEncoderName() string {
	return "libvpx vp8 (full VP8 with P-frames)"
}
