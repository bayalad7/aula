package broadcast

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
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
	// Loopback also offers 127.0.0.1, for tests on one machine.
	Loopback bool
	Logger   *slog.Logger
}

// Media is the console's WebRTC stack: one API, one shared H.264 track and
// one UDP port for every receiver. Close it when the broadcast ends.
type Media struct {
	api   *webrtc.API
	track *webrtc.TrackLocalStaticSample
	conn  net.PacketConn
	mux   ice.UDPMux
	log   *slog.Logger
}

// NewMedia opens the UDP port and builds the stack. No STUN or TURN server
// is configured: only host candidates of the LAN.
func NewMedia(opt MediaOptions) (*Media, error) {
	log := opt.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	me := &webrtc.MediaEngine{}
	err := me.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000,
			SDPFmtpLine: H264FMTP, RTCPFeedback: []webrtc.RTCPFeedback{
				{Type: "nack"}, {Type: "nack", Parameter: "pli"}, {Type: "ccm", Parameter: "fir"},
			}},
		PayloadType: h264PayloadType,
	}, webrtc.RTPCodecTypeVideo)
	if err != nil {
		return nil, fmt.Errorf("broadcast media: %w", err)
	}
	ir := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(me, ir); err != nil {
		return nil, fmt.Errorf("broadcast media: %w", err)
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

	track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264,
		ClockRate: 90000, SDPFmtpLine: H264FMTP}, "pantalla", "aula-profesor")
	if err != nil {
		mux.Close()
		return nil, fmt.Errorf("broadcast media: %w", err)
	}
	return &Media{
		api:   webrtc.NewAPI(webrtc.WithMediaEngine(me), webrtc.WithInterceptorRegistry(ir), webrtc.WithSettingEngine(se)),
		track: track, conn: conn, mux: mux, log: log,
	}, nil
}

// Port is the UDP port in use.
func (m *Media) Port() int { return m.conn.LocalAddr().(*net.UDPAddr).Port }

// WriteSample sends one access unit to every connected receiver.
func (m *Media) WriteSample(au []byte, d time.Duration) error {
	return m.track.WriteSample(media.Sample{Data: au, Duration: d})
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
		if ev.OnState == nil {
			return
		}
		switch s {
		case webrtc.PeerConnectionStateConnected:
			ev.OnState(PeerConnected)
		case webrtc.PeerConnectionStateDisconnected:
			ev.OnState(PeerDisconnected)
		case webrtc.PeerConnectionStateFailed:
			ev.OnState(PeerFailed)
		case webrtc.PeerConnectionStateClosed:
			ev.OnState(PeerClosed)
		default:
			ev.OnState(PeerConnecting)
		}
	})
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
	return &pionPeer{pc: pc}, nil
}

type pionPeer struct {
	pc *webrtc.PeerConnection
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
	if err := checkSDP(sdp, "recvonly"); err != nil {
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
