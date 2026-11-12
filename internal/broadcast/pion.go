package broadcast

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/transport/v5/stdnet"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

// H264FMTP is the only codec the console offers (D-014): Constrained
// Baseline, level 4.0, which is what platform.VideoEncoder produces.
const H264FMTP = "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e028"

const h264PayloadType = 102

// MediaOptions set up the WebRTC side of the console.
type MediaOptions struct {
	// UDPPort is the single port every connection uses (ICE-lite over a UDP
	// mux): ws_port + 1, 47831 by default. 0 picks a free one (tests).
	UDPPort int
	// Loopback also offers 127.0.0.1: for tests on one machine, and for the
	// console's own view of a student's presentation (its WebView connects
	// from this machine).
	Loopback bool
	// Relay makes the shared track forward the RTP packets of a student's
	// presentation (WriteRTP, NewSourcePeer) instead of encoding samples
	// (docs/protocol.md §5.24).
	Relay  bool
	Logger *slog.Logger
}

// Media is the console's WebRTC stack: one API, one shared H.264 track and
// one UDP port for every receiver. Close it when the broadcast ends.
type Media struct {
	api    *webrtc.API
	track  webrtc.TrackLocal
	sample *webrtc.TrackLocalStaticSample // the teacher's screen
	relay  *webrtc.TrackLocalStaticRTP    // a student's screen, forwarded
	conn   net.PacketConn
	mux    ice.UDPMux
	log    *slog.Logger
}

// NewMedia opens the UDP port and builds the stack. No STUN or TURN server
// is configured: only host candidates of the LAN.
func NewMedia(opt MediaOptions) (*Media, error) {
	log := opt.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	me, ir, err := h264Engine()
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenPacket("udp4", ":"+strconv.Itoa(opt.UDPPort))
	if err != nil {
		return nil, fmt.Errorf("el puerto UDP %d está ocupado: %w", opt.UDPPort, err)
	}
	nw, err := stdnet.NewNet()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("broadcast media: %w", err)
	}
	mux := ice.NewUDPMuxDefault(ice.UDPMuxParams{UDPConn: conn, Net: nw})

	var se webrtc.SettingEngine
	se.SetLite(true)
	se.SetICEUDPMux(mux)
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetIncludeLoopbackCandidate(opt.Loopback)

	m := &Media{
		api:  webrtc.NewAPI(webrtc.WithMediaEngine(me), webrtc.WithInterceptorRegistry(ir), webrtc.WithSettingEngine(se)),
		conn: conn, mux: mux, log: log,
	}
	if opt.Relay {
		m.relay, err = webrtc.NewTrackLocalStaticRTP(h264Capability, "pantalla", "aula-alumno")
		m.track = m.relay
	} else {
		m.sample, err = webrtc.NewTrackLocalStaticSample(h264Capability, "pantalla", "aula-profesor")
		m.track = m.sample
	}
	if err != nil {
		mux.Close()
		return nil, fmt.Errorf("broadcast media: %w", err)
	}
	return m, nil
}

var h264Capability = webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: H264FMTP}

// h264Engine registers the only codec of Aula (D-014) with NACK, PLI and FIR
// feedback, and pion's default interceptors.
func h264Engine() (*webrtc.MediaEngine, *interceptor.Registry, error) {
	me := &webrtc.MediaEngine{}
	err := me.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000,
			SDPFmtpLine: H264FMTP, RTCPFeedback: []webrtc.RTCPFeedback{
				{Type: "nack"}, {Type: "nack", Parameter: "pli"}, {Type: "ccm", Parameter: "fir"},
			}},
		PayloadType: h264PayloadType,
	}, webrtc.RTPCodecTypeVideo)
	if err != nil {
		return nil, nil, fmt.Errorf("broadcast media: %w", err)
	}
	ir := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(me, ir); err != nil {
		return nil, nil, fmt.Errorf("broadcast media: %w", err)
	}
	return me, ir, nil
}

// Port is the UDP port in use.
func (m *Media) Port() int { return m.conn.LocalAddr().(*net.UDPAddr).Port }

// WriteSample sends one access unit to every connected receiver.
func (m *Media) WriteSample(au []byte, d time.Duration) error {
	if m.sample == nil {
		return errors.New("broadcast media: a relay takes RTP packets, not samples")
	}
	return m.sample.WriteSample(media.Sample{Data: au, Duration: d})
}

// WriteRTP forwards one packet of the student's screen to every connected
// receiver, untouched but for its header extensions: their IDs were agreed
// with the source, not with the receivers. pion sets the SSRC and payload
// type of each receiver.
func (m *Media) WriteRTP(pkt *rtp.Packet) error {
	if m.relay == nil {
		return errors.New("broadcast media: not a relay")
	}
	pkt.Header.Extension = false
	pkt.Header.Extensions = nil
	return m.relay.WriteRTP(pkt)
}

// Close releases the UDP port. Close every peer first.
func (m *Media) Close() error {
	return m.mux.Close()
}

// NewPeer implements PeerFactory.
func (m *Media) NewPeer(ev PeerEvents) (Peer, error) {
	pc, err := m.api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, fmt.Errorf("peer connection: %w", err)
	}
	tr, err := pc.AddTransceiverFromTrack(m.track, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly})
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("add track: %w", err)
	}
	wireEvents(pc, ev)
	// RTCP from the receiver: keyframe requests. Reading also lets the
	// interceptors (NACK, reports) see the packets. It ends on Close.
	go func() {
		for {
			pkts, _, err := tr.Sender().ReadRTCP()
			if err != nil {
				return
			}
			for _, p := range pkts {
				switch p.(type) {
				case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
					if ev.OnKeyframe != nil {
						ev.OnKeyframe()
					}
				}
			}
		}
	}()
	return &pionPeer{pc: pc, answer: "recvonly"}, nil
}

// SourcePeer is the console's connection with the student who presents
// (docs/protocol.md §5.24): it offers to receive one video and forwards
// every packet to the shared relay track.
type SourcePeer interface {
	Peer
	// RequestKeyframe sends a PLI to the source.
	RequestKeyframe() error
}

// SourcePeerFactory opens a new SourcePeer wired to ev; ev.OnKeyframe is not
// used (the source is the one that makes keyframes).
type SourcePeerFactory func(ev PeerEvents) (SourcePeer, error)

// NewSourcePeer implements SourcePeerFactory; the media must be a relay.
func (m *Media) NewSourcePeer(ev PeerEvents) (SourcePeer, error) {
	if m.relay == nil {
		return nil, errors.New("broadcast media: not a relay")
	}
	pc, err := m.api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, fmt.Errorf("peer connection: %w", err)
	}
	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		pc.Close()
		return nil, fmt.Errorf("add transceiver: %w", err)
	}
	p := &pionPeer{pc: pc, answer: "sendonly"}
	pc.OnTrack(func(tr *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		p.setRemoteSSRC(uint32(tr.SSRC()))
		_ = p.RequestKeyframe()
		// The loop ends when the connection closes (a new attempt or the
		// end of the presentation).
		for {
			pkt, _, err := tr.ReadRTP()
			if err != nil {
				return
			}
			if err := m.WriteRTP(pkt); err != nil {
				m.log.Debug("presentation packet not forwarded", "err", err)
			}
		}
	})
	wireEvents(pc, ev)
	return p, nil
}

// wireEvents hands the connection's local candidates and states to ev.
func wireEvents(pc *webrtc.PeerConnection, ev PeerEvents) {
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if ev.OnCandidate == nil {
			return
		}
		if c == nil {
			ev.OnCandidate(Candidate{})
			return
		}
		init := c.ToJSON()
		cand := Candidate{Candidate: init.Candidate}
		if init.SDPMid != nil {
			cand.SDPMid = *init.SDPMid
		}
		if init.SDPMLineIndex != nil {
			cand.SDPMLineIndex = int(*init.SDPMLineIndex)
		}
		ev.OnCandidate(cand)
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		if ev.OnState != nil {
			ev.OnState(peerState(s))
		}
	})
}

func peerState(s webrtc.PeerConnectionState) PeerState {
	switch s {
	case webrtc.PeerConnectionStateConnected:
		return PeerConnected
	case webrtc.PeerConnectionStateDisconnected:
		return PeerDisconnected
	case webrtc.PeerConnectionStateFailed:
		return PeerFailed
	case webrtc.PeerConnectionStateClosed:
		return PeerClosed
	}
	return PeerConnecting
}

type pionPeer struct {
	pc     *webrtc.PeerConnection
	answer string // the direction the answer must have

	mu   sync.Mutex
	ssrc uint32 // of the source's video, once it arrives
}

func (p *pionPeer) setRemoteSSRC(ssrc uint32) {
	p.mu.Lock()
	p.ssrc = ssrc
	p.mu.Unlock()
}

// RequestKeyframe implements SourcePeer.
func (p *pionPeer) RequestKeyframe() error {
	p.mu.Lock()
	ssrc := p.ssrc
	p.mu.Unlock()
	if ssrc == 0 {
		return nil // no video yet: OnTrack asks when it arrives
	}
	return p.pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: ssrc}})
}

func (p *pionPeer) Offer() (string, error) {
	offer, err := p.pc.CreateOffer(nil)
	if err != nil {
		return "", fmt.Errorf("create offer: %w", err)
	}
	if err := p.pc.SetLocalDescription(offer); err != nil {
		return "", fmt.Errorf("set local description: %w", err)
	}
	return p.pc.LocalDescription().SDP, nil
}

func (p *pionPeer) SetAnswer(sdp string) error {
	if err := checkSDP(sdp, p.answer); err != nil {
		return err
	}
	if err := p.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: sdp}); err != nil {
		return fmt.Errorf("set remote description: %w", err)
	}
	return nil
}

func (p *pionPeer) AddCandidate(c Candidate) error {
	idx := uint16(c.SDPMLineIndex)
	init := webrtc.ICECandidateInit{Candidate: c.Candidate, SDPMid: &c.SDPMid, SDPMLineIndex: &idx}
	if err := p.pc.AddICECandidate(init); err != nil {
		return fmt.Errorf("add candidate: %w", err)
	}
	return nil
}

func (p *pionPeer) Close() error {
	if err := p.pc.Close(); err != nil && !errors.Is(err, webrtc.ErrConnectionClosed) {
		return fmt.Errorf("close peer: %w", err)
	}
	return nil
}
