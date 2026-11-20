//go:build linux && cgo

package platform

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>
#include <wels/codec_api.h>

// openh264 is loaded at run time (dlopen): without it Aula still starts, and
// only the broadcast is not offered. Only the ABI of these headers (2.4,
// libopenh264.so.7) is loaded.
typedef int (*aula_create_fn)(ISVCEncoder**);
typedef void (*aula_destroy_fn)(ISVCEncoder*);

static void* aula_lib;
static aula_create_fn aula_create;
static aula_destroy_fn aula_destroy;

static int aula_h264_load(void) {
	if (aula_lib) return 0;
	void* lib = dlopen("libopenh264.so.7", RTLD_NOW | RTLD_LOCAL);
	if (!lib) return -1;
	aula_create = (aula_create_fn)dlsym(lib, "WelsCreateSVCEncoder");
	aula_destroy = (aula_destroy_fn)dlsym(lib, "WelsDestroySVCEncoder");
	if (!aula_create || !aula_destroy) { dlclose(lib); return -2; }
	aula_lib = lib;
	return 0;
}

// aula_h264_open creates an encoder for screen content: Constrained
// Baseline (CAVLC), one layer, bitrate control, a keyframe every gop frames.
static int aula_h264_open(ISVCEncoder** out, int w, int h, int fps, int kbps, int gop) {
	ISVCEncoder* enc = NULL;
	if (aula_create(&enc) != 0 || !enc) return -1;
	SEncParamExt p;
	memset(&p, 0, sizeof p);
	(*enc)->GetDefaultParams(enc, &p);
	p.iUsageType = SCREEN_CONTENT_REAL_TIME;
	p.iPicWidth = w;
	p.iPicHeight = h;
	p.iTargetBitrate = kbps * 600; // the mean is about 60 % of the peak
	p.iMaxBitrate = kbps * 1000;
	p.iRCMode = RC_BITRATE_MODE;
	p.fMaxFrameRate = (float)fps;
	p.uiIntraPeriod = gop;
	p.iEntropyCodingModeFlag = 0; // CAVLC: Baseline
	p.bEnableFrameSkip = 1;
	p.iMultipleThreadIdc = 0;     // as many threads as CPUs
	p.iSpatialLayerNum = 1;
	p.iTemporalLayerNum = 1;
	p.sSpatialLayers[0].iVideoWidth = w;
	p.sSpatialLayers[0].iVideoHeight = h;
	p.sSpatialLayers[0].fFrameRate = (float)fps;
	p.sSpatialLayers[0].iSpatialBitrate = p.iTargetBitrate;
	p.sSpatialLayers[0].iMaxSpatialBitrate = p.iMaxBitrate;
	p.sSpatialLayers[0].uiProfileIdc = PRO_BASELINE;
	p.sSpatialLayers[0].sSliceArgument.uiSliceMode = SM_SINGLE_SLICE;
	if ((*enc)->InitializeExt(enc, &p) != 0) {
		aula_destroy(enc);
		return -2;
	}
	*out = enc;
	return 0;
}

// aula_h264_encode encodes one I420 frame into out (Annex-B: openh264's
// NALs already carry start codes; SPS and PPS come with every IDR).
static int aula_h264_encode(ISVCEncoder* enc, unsigned char* y, unsigned char* u, unsigned char* v,
		int w, int h, long long ts, int force_key, unsigned char* out, int cap, int* out_len) {
	if (force_key) (*enc)->ForceIntraFrame(enc, 1);
	SSourcePicture pic;
	memset(&pic, 0, sizeof pic);
	pic.iColorFormat = videoFormatI420;
	pic.iPicWidth = w;
	pic.iPicHeight = h;
	pic.iStride[0] = w;
	pic.iStride[1] = w / 2;
	pic.iStride[2] = w / 2;
	pic.pData[0] = y;
	pic.pData[1] = u;
	pic.pData[2] = v;
	pic.uiTimeStamp = ts;
	SFrameBSInfo info;
	memset(&info, 0, sizeof info);
	int r = (*enc)->EncodeFrame(enc, &pic, &info);
	if (r != 0) return r;
	*out_len = 0;
	if (info.eFrameType == videoFrameTypeSkip) return 0;
	int n = 0;
	for (int l = 0; l < info.iLayerNum; l++) {
		SLayerBSInfo* layer = &info.sLayerInfo[l];
		int size = 0;
		for (int i = 0; i < layer->iNalCount; i++) size += layer->pNalLengthInByte[i];
		if (n + size > cap) return -100;
		memcpy(out + n, layer->pBsBuf, size);
		n += size;
	}
	*out_len = n;
	return 0;
}

static void aula_h264_close(ISVCEncoder* enc) {
	(*enc)->Uninitialize(enc);
	aula_destroy(enc);
}
*/
import "C"

import (
	"fmt"
	"image"
	"sync"
	"time"
	"unsafe"
)

// The H.264 encoder on Linux (docs/privacidad.md rule 106): openh264, loaded
// with dlopen. Without libopenh264.so.7 it fails with ErrNotSupported, and
// neither the console broadcasts nor the agent declares screen.present.

var h264Load struct {
	once sync.Once
	err  error
}

func loadOpenH264() error {
	h264Load.once.Do(func() {
		if C.aula_h264_load() != 0 {
			h264Load.err = fmt.Errorf("h264 encoder: libopenh264.so.7 not found: %w", ErrNotSupported)
		}
	})
	return h264Load.err
}

func newH264Encoder(cfg VideoEncoderConfig) (VideoEncoder, error) {
	if err := loadOpenH264(); err != nil {
		return nil, err
	}
	var enc *C.ISVCEncoder
	if r := C.aula_h264_open(&enc, C.int(cfg.Width), C.int(cfg.Height), C.int(cfg.FPS),
		C.int(cfg.BitrateKbps), C.int(cfg.KeyframeEvery)); r != 0 {
		return nil, fmt.Errorf("h264 encoder: openh264 setup failed (%d)", int(r))
	}
	w, h := cfg.Width, cfg.Height
	return &openH264{enc: enc, w: w, h: h, nv12: make([]byte, w*h*3/2), i420: make([]byte, w*h*3/2),
		out: make([]byte, w*h*2+1<<16), start: time.Now()}, nil
}

type openH264 struct {
	mu         sync.Mutex
	enc        *C.ISVCEncoder
	w, h       int
	nv12, i420 []byte
	out        []byte
	start      time.Time
}

func (e *openH264) Encode(img *image.RGBA, forceKey bool) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.enc == nil {
		return nil, fmt.Errorf("h264 encoder: closed")
	}
	if b := img.Bounds(); b.Dx() != e.w || b.Dy() != e.h {
		return nil, fmt.Errorf("h264 encoder: frame %dx%d, want %dx%d", b.Dx(), b.Dy(), e.w, e.h)
	}
	if err := RGBAToNV12(img, e.nv12); err != nil {
		return nil, err
	}
	nv12ToI420(e.nv12, e.i420, e.w, e.h)
	ySize, cSize := e.w*e.h, e.w*e.h/4
	key := C.int(0)
	if forceKey {
		key = 1
	}
	var n C.int
	r := C.aula_h264_encode(e.enc,
		(*C.uchar)(unsafe.Pointer(&e.i420[0])),
		(*C.uchar)(unsafe.Pointer(&e.i420[ySize])),
		(*C.uchar)(unsafe.Pointer(&e.i420[ySize+cSize])),
		C.int(e.w), C.int(e.h), C.longlong(time.Since(e.start).Milliseconds()), key,
		(*C.uchar)(unsafe.Pointer(&e.out[0])), C.int(len(e.out)), &n)
	if r != 0 {
		return nil, fmt.Errorf("h264 encoder: encode failed (%d)", int(r))
	}
	if n == 0 {
		return nil, nil // skipped by the rate control
	}
	return append([]byte(nil), e.out[:n]...), nil
}

func (e *openH264) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.enc != nil {
		C.aula_h264_close(e.enc)
		e.enc = nil
	}
	return nil
}

// nv12ToI420 splits NV12's interleaved chroma into I420's U and V planes.
func nv12ToI420(nv12, i420 []byte, w, h int) {
	ySize := w * h
	copy(i420[:ySize], nv12[:ySize])
	uv := nv12[ySize : ySize+ySize/2]
	u := i420[ySize : ySize+ySize/4]
	v := i420[ySize+ySize/4 : ySize+ySize/2]
	for i := range ySize / 4 {
		u[i], v[i] = uv[2*i], uv[2*i+1]
	}
}
