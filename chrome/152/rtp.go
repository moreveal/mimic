package chrome152

import (
	"fmt"
	"github.com/moreveal/mimic/internal/state"
	"strings"
)

// Frozen Chrome 152 Windows codec policy. These are codec/profile metadata and
// preferred payload assignments, not complete captured SDP descriptions.
func rtpCatalog() state.RTPCatalog {
	audio := state.RTPMediaCatalog{Codecs: map[string]state.RTPCodec{}, ReceiverRTT: true}
	audio.SenderOrder = strings.Fields("opus red g722 pcmu pcma cn dtmf48 dtmf8")
	audio.ReceiverOrder = append([]string(nil), audio.SenderOrder...)
	audio.OfferOrder = append([]string(nil), audio.SenderOrder...)
	for _, v := range []struct {
		key, name          string
		rate, channels, pt int
	}{{"opus", "opus", 48000, 2, 111}, {"red", "red", 48000, 2, 63}, {"g722", "G722", 8000, 1, 9}, {"pcmu", "PCMU", 8000, 1, 0}, {"pcma", "PCMA", 8000, 1, 8}, {"cn", "CN", 8000, 1, 13}, {"dtmf48", "telephone-event", 48000, 1, 110}, {"dtmf8", "telephone-event", 8000, 1, 126}} {
		audio.Codecs[v.key] = state.RTPCodec{Name: v.name, ClockRate: v.rate, Channels: v.channels, Payload: v.pt, RTXPayload: -1}
	}
	opus := audio.Codecs["opus"]
	opus.Parameters = "minptime=10;useinbandfec=1"
	opus.Feedback = []string{"transport-cc"}
	audio.Codecs["opus"] = opus
	red := audio.Codecs["red"]
	red.Redundant = []string{"opus", "opus"}
	audio.Codecs["red"] = red
	video := state.RTPMediaCatalog{Codecs: map[string]state.RTPCodec{}, ReceiverRTT: true}
	video.SenderOrder = strings.Fields("vp8 rtx h264-base-p1 h264-base-p0 h264-constrained-p1 h264-constrained-p0 h264-main-p1 h264-main-p0 av1-0 vp9-0 vp9-2 h264-high-p1 h265-1 h265-2 red ulpfec")
	video.ReceiverOrder = strings.Fields("vp8 rtx vp9-0 vp9-2 vp9-1 vp9-3 h264-base-p1 h264-base-p0 h264-constrained-p1 h264-constrained-p0 h264-main-p1 h264-main-p0 h264-predictive-p1 h264-predictive-p0 av1-0 av1-1 h264-high-p1 h264-high-p0 h265-1 h265-2 red ulpfec flexfec")
	for _, key := range video.SenderOrder {
		if key != "rtx" {
			video.OfferOrder = append(video.OfferOrder, key)
		}
	}
	feedback := strings.Fields("goog-remb transport-cc")
	feedback = append(feedback, "ccm fir", "nack", "nack pli")
	add := func(key, name, parameters string, pt, rtx int, media bool) {
		c := state.RTPCodec{Name: name, ClockRate: 90000, Parameters: parameters, Payload: pt, RTXPayload: rtx}
		if media {
			c.Feedback = append([]string(nil), feedback...)
		}
		video.Codecs[key] = c
	}
	add("vp8", "VP8", "", 96, 97, true)
	add("rtx", "rtx", "", -1, -1, false)
	for _, v := range []struct {
		key, profile   string
		p1, r1, p0, r0 int
	}{{"base", "42001f", 102, 103, 104, 107}, {"constrained", "42e01f", 108, 109, 114, 115}, {"main", "4d001f", 116, 117, 39, 40}, {"predictive", "f4001f", -1, -1, -1, -1}, {"high", "64001f", 118, 119, -1, -1}} {
		for _, mode := range []int{1, 0} {
			pt, rtx := v.p1, v.r1
			if mode == 0 {
				pt, rtx = v.p0, v.r0
			}
			add(fmt.Sprintf("h264-%s-p%d", v.key, mode), "H264", fmt.Sprintf("level-asymmetry-allowed=1;packetization-mode=%d;profile-level-id=%s", mode, v.profile), pt, rtx, true)
		}
	}
	high := video.Codecs["h264-high-p1"]
	high.SenderParameters = "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640033"
	video.Codecs["h264-high-p1"] = high
	for _, v := range []struct{ profile, pt, rtx int }{{0, 98, 99}, {2, 100, 101}, {1, -1, -1}, {3, -1, -1}} {
		add(fmt.Sprintf("vp9-%d", v.profile), "VP9", fmt.Sprintf("profile-id=%d", v.profile), v.pt, v.rtx, true)
	}
	add("av1-0", "AV1", "level-idx=5;profile=0;tier=0", 45, 46, true)
	add("av1-1", "AV1", "level-idx=5;profile=1;tier=0", -1, -1, true)
	for _, v := range []struct{ profile, pt, rtx int }{{1, 49, 50}, {2, 51, 52}} {
		add(fmt.Sprintf("h265-%d", v.profile), "H265", fmt.Sprintf("level-id=180;profile-id=%d;tier-flag=0;tx-mode=SRST", v.profile), v.pt, v.rtx, true)
	}
	add("red", "red", "", 122, 123, false)
	add("ulpfec", "ulpfec", "", 124, -1, false)
	add("flexfec", "flexfec-03", "repair-window=10000000", -1, -1, false)
	abs := "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time"
	transport := "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01"
	mid := "urn:ietf:params:rtp-hdrext:sdes:mid"
	audio.HeaderExtensions = []string{"urn:ietf:params:rtp-hdrext:ssrc-audio-level", abs, transport, mid}
	video.HeaderExtensions = []string{"urn:ietf:params:rtp-hdrext:toffset", abs, "urn:3gpp:video-orientation", transport, "http://www.webrtc.org/experiments/rtp-hdrext/playout-delay", "http://www.webrtc.org/experiments/rtp-hdrext/video-content-type", "http://www.webrtc.org/experiments/rtp-hdrext/video-timing", "http://www.webrtc.org/experiments/rtp-hdrext/color-space", mid, "urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id", "urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id"}
	return state.RTPCatalog{"audio": audio, "video": video}
}
