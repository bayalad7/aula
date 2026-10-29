//go:build windows && (amd64 || arm64)

package platform

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// H.264 encoder on Media Foundation, called through COM vtables without cgo.
// It uses the synchronous software MFT that ships with Windows ("H264 Encoder
// MFT"); Windows "N" editions without the Media Feature Pack lack it, and
// then NewH264Encoder fails and the console cannot broadcast (D-014).
//
// COM objects are only touched from one locked OS thread, initialized in the
// multithreaded apartment; Encode and Close hand their work to that thread.

var (
	mfplat                = windows.NewLazySystemDLL("mfplat.dll")
	procMFStartup         = mfplat.NewProc("MFStartup")
	procMFShutdown        = mfplat.NewProc("MFShutdown")
	procMFTEnumEx         = mfplat.NewProc("MFTEnumEx")
	procMFCreateMediaType = mfplat.NewProc("MFCreateMediaType")
	procMFCreateSample    = mfplat.NewProc("MFCreateSample")
	procMFCreateMemBuffer = mfplat.NewProc("MFCreateMemoryBuffer")
	ole32                 = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx    = ole32.NewProc("CoInitializeEx")
	procCoUninitialize    = ole32.NewProc("CoUninitialize")
	procCoTaskMemFree     = ole32.NewProc("CoTaskMemFree")
)

const (
	mfVersion       = 0x00020070 // MF_SDK_VERSION << 16 | MF_API_VERSION
	mfStartupLite   = 1          // no sockets
	coinitMulti     = 0x0
	mftEnumSync     = 0x01
	mftEnumLocal    = 0x10
	mftEnumSorted   = 0x40
	mftProvidesSmp  = 0x100 // MFT_OUTPUT_STREAM_PROVIDES_SAMPLES
	mftMsgBeginStrm = 0x10000000
	mftMsgStartStrm = 0x10000003
	mftMsgEndStrm   = 0x10000002
	interlaceProg   = 2
	h264ProfileBase = 66
	rateCtrlPeakVBR = 1 // eAVEncCommonRateControlMode_PeakConstrainedVBR
	vtUI4           = 19
	vtBool          = 11
	hundredNSPerSec = 10_000_000

	mfENeedMoreInput = 0xC00D6D72
	mfEStreamChange  = 0xC00D6D61
)

var (
	categoryVideoEncoder = guid("f79eac7d-e545-4387-bdee-d647d7bde42a")
	mediaTypeVideo       = guid("73646976-0000-0010-8000-00aa00389b71")
	formatNV12           = guid("3231564e-0000-0010-8000-00aa00389b71")
	formatH264           = guid("34363248-0000-0010-8000-00aa00389b71")
	mtMajorType          = guid("48eba18e-f8c9-4687-bf11-0a74c9f96a8f")
	mtSubtype            = guid("f7e34c9a-42e8-4714-b74b-cb29d72c35e5")
	mtAvgBitrate         = guid("20332624-fb0d-4d9e-bd0d-cbf6786c102e")
	mtInterlace          = guid("e2724bb8-e676-4806-b4b2-a8d6efb44ccd")
	mtFrameSize          = guid("1652c33d-d6b2-4012-b834-72030849a37d")
	mtFrameRate          = guid("c459a2e8-3d2c-4e44-b132-fee5156c7bb0")
	mtPixelAspect        = guid("c6376a1e-8d0a-4027-be45-6d9a0ad39bb6")
	mtProfile            = guid("ad76a80b-2d5c-4e0b-b375-64e520137036")
	mtSequenceHeader     = guid("3c036de7-3ad0-4c9e-9216-ee6d6ac21cb3")
	iidIMFTransform      = guid("bf94c121-5b05-4e6f-8000-ba598961414d")
	iidICodecAPI         = guid("901db4c7-31ce-41a2-85dc-8fa0bf41b8da")
	apiLowLatency        = guid("9c27891a-ed7a-40e1-88e8-b22727a024ee")
	apiRateControl       = guid("1c0608e9-370c-4710-8a58-cb6181c42423")
	apiMeanBitrate       = guid("f7222374-2144-4815-b550-a37f8e12ee52")
	apiMaxBitrate        = guid("9651eae4-39b9-4ebf-85ef-d7f444ec7465")
	apiGOPSize           = guid("95f31b26-95a4-41aa-9303-246a7fc6eef1")
	apiBFrames           = guid("8d390aac-dc5c-4200-b57f-814d04babab2")
	apiForceKeyFrame     = guid("398c1b98-8353-475a-9ef2-8f265d260345")
)

// vtable slots (IUnknown, IMFAttributes, IMFTransform, IMFSample,
// IMFMediaBuffer, IMFActivate, ICodecAPI).
const (
	slotRelease = 2

	slotGetBlobSize = 14
	slotGetBlob     = 15
	slotSetUINT32   = 21
	slotSetUINT64   = 22
	slotSetGUID     = 24

	slotGetOutputStreamInfo    = 7
	slotGetAttributes          = 8
	slotGetOutputAvailableType = 14
	slotSetInputType           = 15
	slotSetOutputType          = 16
	slotGetOutputCurrentType   = 18
	slotProcessMessage         = 23
	slotProcessInput           = 24
	slotProcessOutput          = 25

	slotSetSampleTime     = 36
	slotSetSampleDuration = 38
	slotConvertToContig   = 41
	slotAddBuffer         = 42

	slotLock             = 3
	slotUnlock           = 4
	slotSetCurrentLength = 6

	slotActivateObject = 33

	slotCodecSetValue = 9
)

// comObj is a COM interface pointer; its first word points to the vtable.
type comObj = *comIface

type comIface struct{ vtbl *[64]uintptr }

// call invokes vtable slot n of the object and turns a failed HRESULT into an
// error.
func (o *comIface) call(n int, args ...uintptr) error {
	if hr := o.callHR(n, args...); int32(hr) < 0 {
		return hresult(hr)
	}
	return nil
}

func (o *comIface) callHR(n int, args ...uintptr) uint32 {
	r, _, _ := syscall.SyscallN(o.vtbl[n], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return uint32(r)
}

func (o *comIface) release() {
	if o != nil {
		o.callHR(slotRelease)
	}
}

type hresult uint32

func (h hresult) Error() string { return fmt.Sprintf("HRESULT 0x%08X", uint32(h)) }

func hrErr(r uintptr) error {
	if int32(r) < 0 {
		return hresult(uint32(r))
	}
	return nil
}

func guid(s string) windows.GUID {
	g, err := windows.GUIDFromString("{" + s + "}")
	if err != nil {
		panic(err) // constants above; caught by any test that loads the package
	}
	return g
}

type mftOutputStreamInfo struct {
	Flags, Size, Alignment uint32
}

type mftOutputDataBuffer struct {
	StreamID uint32
	Sample   comObj
	Status   uint32
	Events   comObj
}

type mftRegisterTypeInfo struct {
	Major, Sub windows.GUID
}

type variant struct {
	VT  uint16
	_   [3]uint16
	Val uint64
	_   uint64
}

// mfEncoder serializes every COM call onto its own thread. Like every
// VideoEncoder, it is used from one goroutine at a time.
type mfEncoder struct {
	cfg       VideoEncoderConfig
	work      chan func()
	done      chan struct{}
	closeOnce sync.Once

	// Only touched on the COM thread.
	mft      comObj
	codec    comObj
	nv12     []byte
	seqHdr   []byte // SPS and PPS, from the output type
	frame    int64
	provides bool
	outSize  uint32
}

func newH264Encoder(cfg VideoEncoderConfig) (VideoEncoder, error) {
	e := &mfEncoder{cfg: cfg, work: make(chan func()), done: make(chan struct{}),
		nv12: make([]byte, cfg.Width*cfg.Height*3/2)}
	ready := make(chan error, 1)
	go e.thread(ready)
	if err := <-ready; err != nil {
		return nil, fmt.Errorf("h264 encoder: %w", err)
	}
	return e, nil
}

func (e *mfEncoder) thread(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(e.done)
	if r, _, _ := procCoInitializeEx.Call(0, coinitMulti); int32(r) < 0 {
		ready <- fmt.Errorf("CoInitializeEx: %w", hresult(uint32(r)))
		return
	}
	defer procCoUninitialize.Call() //nolint:errcheck // void
	if err := procMFStartup.Find(); err != nil {
		ready <- fmt.Errorf("media foundation: %w: %w", ErrNotSupported, err)
		return
	}
	if r, _, _ := procMFStartup.Call(mfVersion, mfStartupLite); int32(r) < 0 {
		ready <- fmt.Errorf("MFStartup: %w", hresult(uint32(r)))
		return
	}
	defer procMFShutdown.Call() //nolint:errcheck // best effort
	if err := e.open(); err != nil {
		e.closeCOM()
		ready <- err
		return
	}
	ready <- nil
	for f := range e.work {
		f()
	}
	e.closeCOM()
}

func (e *mfEncoder) do(f func()) error {
	select {
	case <-e.done:
		return errors.New("h264 encoder closed")
	default:
	}
	finished := make(chan struct{})
	e.work <- func() { f(); close(finished) }
	<-finished
	return nil
}

// open finds the software encoder, configures it and starts streaming.
func (e *mfEncoder) open() error {
	out := mftRegisterTypeInfo{Major: mediaTypeVideo, Sub: formatH264}
	var list *comObj
	var count uint32
	r, _, _ := procMFTEnumEx.Call(uintptr(unsafe.Pointer(&categoryVideoEncoder)), mftEnumSync|mftEnumLocal|mftEnumSorted,
		0, uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&list)), uintptr(unsafe.Pointer(&count)))
	if err := hrErr(r); err != nil {
		return fmt.Errorf("MFTEnumEx: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("no H.264 encoder on this system (Windows N without the Media Feature Pack?): %w", ErrNotSupported)
	}
	activates := unsafe.Slice(list, count)
	err := activates[0].call(slotActivateObject, uintptr(unsafe.Pointer(&iidIMFTransform)), uintptr(unsafe.Pointer(&e.mft)))
	for _, a := range activates {
		a.release()
	}
	procCoTaskMemFree.Call(uintptr(unsafe.Pointer(list))) //nolint:errcheck // void
	if err != nil {
		return fmt.Errorf("activate encoder: %w", err)
	}

	if err := e.mft.call(0, uintptr(unsafe.Pointer(&iidICodecAPI)), uintptr(unsafe.Pointer(&e.codec))); err != nil {
		e.codec = nil // keyframes on demand and rate control then fall back to defaults
	}
	var attrs comObj
	if e.mft.call(slotGetAttributes, uintptr(unsafe.Pointer(&attrs))) == nil && attrs != nil {
		attrs.call(slotSetUINT32, uintptr(unsafe.Pointer(&apiLowLatency)), 1) //nolint:errcheck // optional
		attrs.release()
	}
	peak := uint64(e.cfg.BitrateKbps) * 1000
	e.setCodec(apiLowLatency, vtBool, 0xFFFF) // VARIANT_TRUE
	e.setCodec(apiRateControl, vtUI4, rateCtrlPeakVBR)
	e.setCodec(apiMeanBitrate, vtUI4, peak*6/10)
	e.setCodec(apiMaxBitrate, vtUI4, peak)
	e.setCodec(apiGOPSize, vtUI4, uint64(e.cfg.KeyframeEvery))
	e.setCodec(apiBFrames, vtUI4, 0)

	outType, err := e.mediaType(formatH264, func(t comObj) error {
		if err := t.call(slotSetUINT32, uintptr(unsafe.Pointer(&mtAvgBitrate)), uintptr(peak*6/10)); err != nil {
			return err
		}
		return t.call(slotSetUINT32, uintptr(unsafe.Pointer(&mtProfile)), h264ProfileBase)
	})
	if err != nil {
		return err
	}
	err = e.mft.call(slotSetOutputType, 0, uintptr(unsafe.Pointer(outType)), 0)
	outType.release()
	if err != nil {
		return fmt.Errorf("set output type: %w", err)
	}
	inType, err := e.mediaType(formatNV12, nil)
	if err != nil {
		return err
	}
	err = e.mft.call(slotSetInputType, 0, uintptr(unsafe.Pointer(inType)), 0)
	inType.release()
	if err != nil {
		return fmt.Errorf("set input type (NV12 %dx%d): %w", e.cfg.Width, e.cfg.Height, err)
	}
	e.readOutputInfo()
	if err := e.mft.call(slotProcessMessage, mftMsgBeginStrm, 0); err != nil {
		return fmt.Errorf("begin streaming: %w", err)
	}
	if err := e.mft.call(slotProcessMessage, mftMsgStartStrm, 0); err != nil {
		return fmt.Errorf("start of stream: %w", err)
	}
	return nil
}

// mediaType builds a progressive video type of the configured size and rate.
func (e *mfEncoder) mediaType(sub windows.GUID, extra func(comObj) error) (comObj, error) {
	var t comObj
	if r, _, _ := procMFCreateMediaType.Call(uintptr(unsafe.Pointer(&t))); int32(r) < 0 {
		return nil, fmt.Errorf("MFCreateMediaType: %w", hresult(uint32(r)))
	}
	size := uint64(e.cfg.Width)<<32 | uint64(e.cfg.Height)
	rate := uint64(e.cfg.FPS)<<32 | 1
	steps := []func() error{
		func() error {
			return t.call(slotSetGUID, uintptr(unsafe.Pointer(&mtMajorType)), uintptr(unsafe.Pointer(&mediaTypeVideo)))
		},
		func() error {
			return t.call(slotSetGUID, uintptr(unsafe.Pointer(&mtSubtype)), uintptr(unsafe.Pointer(&sub)))
		},
		func() error { return t.call(slotSetUINT64, uintptr(unsafe.Pointer(&mtFrameSize)), uintptr(size)) },
		func() error { return t.call(slotSetUINT64, uintptr(unsafe.Pointer(&mtFrameRate)), uintptr(rate)) },
		func() error {
			return t.call(slotSetUINT64, uintptr(unsafe.Pointer(&mtPixelAspect)), uintptr(uint64(1)<<32|1))
		},
		func() error { return t.call(slotSetUINT32, uintptr(unsafe.Pointer(&mtInterlace)), interlaceProg) },
	}
	if extra != nil {
		steps = append(steps, func() error { return extra(t) })
	}
	for _, s := range steps {
		if err := s(); err != nil {
			t.release()
			return nil, fmt.Errorf("media type: %w", err)
		}
	}
	return t, nil
}

func (e *mfEncoder) setCodec(api windows.GUID, vt uint16, val uint64) {
	if e.codec == nil {
		return
	}
	v := variant{VT: vt, Val: val}
	e.codec.call(slotCodecSetValue, uintptr(unsafe.Pointer(&api)), uintptr(unsafe.Pointer(&v))) //nolint:errcheck // optional knob
}

// readOutputInfo learns how output samples are allocated and caches the
// sequence header (SPS and PPS) of the current output type.
func (e *mfEncoder) readOutputInfo() {
	var info mftOutputStreamInfo
	if e.mft.call(slotGetOutputStreamInfo, 0, uintptr(unsafe.Pointer(&info))) == nil {
		e.provides = info.Flags&mftProvidesSmp != 0
		e.outSize = info.Size
	}
	if e.outSize == 0 {
		e.outSize = uint32(e.cfg.Width * e.cfg.Height * 3 / 2)
	}
	var t comObj
	if e.mft.call(slotGetOutputCurrentType, 0, uintptr(unsafe.Pointer(&t))) != nil || t == nil {
		return
	}
	defer t.release()
	var n uint32
	if t.call(slotGetBlobSize, uintptr(unsafe.Pointer(&mtSequenceHeader)), uintptr(unsafe.Pointer(&n))) != nil || n == 0 {
		return
	}
	hdr := make([]byte, n)
	if t.call(slotGetBlob, uintptr(unsafe.Pointer(&mtSequenceHeader)), uintptr(unsafe.Pointer(&hdr[0])), uintptr(n), 0) == nil {
		e.seqHdr = hdr
	}
}

func (e *mfEncoder) Encode(img *image.RGBA, forceKey bool) ([]byte, error) {
	if b := img.Bounds(); b.Dx() != e.cfg.Width || b.Dy() != e.cfg.Height {
		return nil, fmt.Errorf("h264 encode: frame %dx%d, encoder %dx%d", b.Dx(), b.Dy(), e.cfg.Width, e.cfg.Height)
	}
	if err := RGBAToNV12(img, e.nv12); err != nil {
		return nil, err
	}
	var au []byte
	var encErr error
	if err := e.do(func() { au, encErr = e.encode(forceKey) }); err != nil {
		return nil, err
	}
	return au, encErr
}

// encode runs on the COM thread.
func (e *mfEncoder) encode(forceKey bool) ([]byte, error) {
	if forceKey {
		e.setCodec(apiForceKeyFrame, vtUI4, 1)
	}
	sample, err := e.newSample(uint32(len(e.nv12)), e.nv12)
	if err != nil {
		return nil, err
	}
	dur := int64(hundredNSPerSec / e.cfg.FPS)
	sample.call(slotSetSampleTime, uintptr(e.frame*dur)) //nolint:errcheck // cannot fail on a fresh sample
	sample.call(slotSetSampleDuration, uintptr(dur))     //nolint:errcheck
	e.frame++
	err = e.mft.call(slotProcessInput, 0, uintptr(unsafe.Pointer(sample)), 0)
	sample.release()
	if err != nil {
		return nil, fmt.Errorf("process input: %w", err)
	}
	var au []byte
	for {
		chunk, more, err := e.output()
		if err != nil {
			return nil, err
		}
		au = append(au, chunk...)
		if !more {
			break
		}
	}
	if len(au) == 0 {
		return nil, nil
	}
	return e.withParameterSets(au), nil
}

// output pulls one sample; more is false when the encoder needs input.
func (e *mfEncoder) output() (data []byte, more bool, err error) {
	buf := mftOutputDataBuffer{}
	if !e.provides {
		s, err := e.newSample(e.outSize, nil)
		if err != nil {
			return nil, false, err
		}
		buf.Sample = s
	}
	var status uint32
	hr := e.mft.callHR(slotProcessOutput, 0, 1, uintptr(unsafe.Pointer(&buf)), uintptr(unsafe.Pointer(&status)))
	defer func() {
		buf.Sample.release()
		buf.Events.release()
	}()
	switch {
	case hr == mfENeedMoreInput:
		return nil, false, nil
	case hr == mfEStreamChange:
		// Renegotiate with the type the encoder proposes, then try again.
		var t comObj
		if err := e.mft.call(slotGetOutputAvailableType, 0, 0, uintptr(unsafe.Pointer(&t))); err != nil {
			return nil, false, fmt.Errorf("stream change: %w", err)
		}
		err := e.mft.call(slotSetOutputType, 0, uintptr(unsafe.Pointer(t)), 0)
		t.release()
		if err != nil {
			return nil, false, fmt.Errorf("stream change: %w", err)
		}
		e.readOutputInfo()
		return nil, true, nil
	case int32(hr) < 0:
		return nil, false, fmt.Errorf("process output: %w", hresult(hr))
	}
	data, err = sampleBytes(buf.Sample)
	return data, true, err
}

func (e *mfEncoder) newSample(size uint32, fill []byte) (comObj, error) {
	var s, b comObj
	if r, _, _ := procMFCreateSample.Call(uintptr(unsafe.Pointer(&s))); int32(r) < 0 {
		return nil, fmt.Errorf("MFCreateSample: %w", hresult(uint32(r)))
	}
	if r, _, _ := procMFCreateMemBuffer.Call(uintptr(size), uintptr(unsafe.Pointer(&b))); int32(r) < 0 {
		s.release()
		return nil, fmt.Errorf("MFCreateMemoryBuffer: %w", hresult(uint32(r)))
	}
	defer b.release()
	if fill != nil {
		var p *byte
		if err := b.call(slotLock, uintptr(unsafe.Pointer(&p)), 0, 0); err != nil {
			s.release()
			return nil, fmt.Errorf("lock buffer: %w", err)
		}
		copy(unsafe.Slice(p, size), fill)
		b.call(slotUnlock)                          //nolint:errcheck
		b.call(slotSetCurrentLength, uintptr(size)) //nolint:errcheck
	}
	if err := s.call(slotAddBuffer, uintptr(unsafe.Pointer(b))); err != nil {
		s.release()
		return nil, fmt.Errorf("add buffer: %w", err)
	}
	return s, nil
}

func sampleBytes(s comObj) ([]byte, error) {
	if s == nil {
		return nil, nil
	}
	var b comObj
	if err := s.call(slotConvertToContig, uintptr(unsafe.Pointer(&b))); err != nil {
		return nil, fmt.Errorf("output buffer: %w", err)
	}
	defer b.release()
	var p *byte
	var n uint32
	if err := b.call(slotLock, uintptr(unsafe.Pointer(&p)), 0, uintptr(unsafe.Pointer(&n))); err != nil {
		return nil, fmt.Errorf("lock output: %w", err)
	}
	out := bytes.Clone(unsafe.Slice(p, n))
	b.call(slotUnlock) //nolint:errcheck
	return out, nil
}

// withParameterSets prepends SPS and PPS to an IDR that lacks them, so a
// receiver that joins late can always start decoding at the next keyframe.
func (e *mfEncoder) withParameterSets(au []byte) []byte {
	if len(e.seqHdr) == 0 {
		return au
	}
	hasIDR, hasSPS := false, false
	for _, t := range nalTypes(au) {
		switch t {
		case 5:
			hasIDR = true
		case 7:
			hasSPS = true
		}
	}
	if !hasIDR || hasSPS {
		return au
	}
	return append(bytes.Clone(e.seqHdr), au...)
}

// nalTypes lists the NAL unit types of an Annex-B stream.
func nalTypes(b []byte) []byte {
	var out []byte
	for i := 0; i+3 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
			out = append(out, b[i+3]&0x1f)
			i += 3
		}
	}
	return out
}

func (e *mfEncoder) Close() error {
	e.closeOnce.Do(func() { close(e.work) })
	<-e.done
	return nil
}

// closeCOM runs on the COM thread.
func (e *mfEncoder) closeCOM() {
	if e.mft != nil {
		e.mft.callHR(slotProcessMessage, mftMsgEndStrm, 0)
	}
	e.codec.release()
	e.mft.release()
	e.codec, e.mft = nil, nil
}
