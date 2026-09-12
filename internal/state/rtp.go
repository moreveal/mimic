package state

import (
	"fmt"
	"strconv"
	"strings"
)

// RTPCodec describes a codec profile once. Sender, receiver and initial offer
// ordering are separate policies; RTX associations and SDP lines are generated.
type RTPCodec struct {
	Name                         string
	ClockRate, Channels          int
	Parameters, SenderParameters string
	Feedback                     []string
	Payload, RTXPayload          int
	Redundant                    []string
}
type RTPMediaCatalog struct {
	Codecs                                 map[string]RTPCodec
	SenderOrder, ReceiverOrder, OfferOrder []string
	HeaderExtensions                       []string
	ReceiverRTT                            bool
}
type RTPCatalog map[string]RTPMediaCatalog

func (c RTPCatalog) Capabilities(kind, role string) any {
	media, ok := c[kind]
	if !ok {
		return nil
	}
	order := media.SenderOrder
	if role == "receiver" {
		order = media.ReceiverOrder
	}
	codecs := make([]any, 0, len(order))
	extensions := make([]any, 0, len(media.HeaderExtensions))
	for _, key := range order {
		codec := media.Codecs[key]
		row := map[string]any{"mimeType": kind + "/" + codec.Name, "clockRate": codec.ClockRate}
		if codec.Channels > 0 {
			row["channels"] = codec.Channels
		}
		parameters := codec.Parameters
		if role == "sender" && codec.SenderParameters != "" {
			parameters = codec.SenderParameters
		}
		if parameters != "" {
			row["sdpFmtpLine"] = parameters
		}
		codecs = append(codecs, row)
	}
	for _, uri := range media.HeaderExtensions {
		extensions = append(extensions, map[string]any{"uri": uri, "direction": "sendrecv"})
	}
	return map[string]any{"codecs": codecs, "headerExtensions": extensions}
}

type RTPMediaDescription struct {
	Payloads   []int    `json:"payloads"`
	Extensions []string `json:"extensions"`
	Attributes []string `json:"attributes"`
}

func (c RTPCatalog) Media(kind string, direction ...string) RTPMediaDescription {
	out := RTPMediaDescription{Payloads: []int{}, Extensions: []string{}, Attributes: []string{}}
	media, ok := c[kind]
	if !ok {
		return out
	}
	order := media.OfferOrder
	if len(direction) > 0 && direction[0] == "recvonly" {
		order = []string{}
		for _, key := range media.ReceiverOrder {
			if media.Codecs[key].Name != "rtx" {
				order = append(order, key)
			}
		}
	}
	for _, key := range order {
		codec := media.Codecs[key]
		out.Payloads = append(out.Payloads, codec.Payload)
		if codec.RTXPayload >= 0 {
			out.Payloads = append(out.Payloads, codec.RTXPayload)
		}
	}
	for index, uri := range media.HeaderExtensions {
		out.Extensions = append(out.Extensions, fmt.Sprintf("a=extmap:%d %s", index+1, uri))
	}
	if media.ReceiverRTT {
		out.Attributes = append(out.Attributes, "a=rtcp-xr:rcvr-rtt=all")
		for _, pt := range out.Payloads {
			out.Attributes = append(out.Attributes, fmt.Sprintf("a=rtcp-fb:%d rrtr", pt))
		}
	}
	for _, key := range order {
		codec := media.Codecs[key]
		mapping := fmt.Sprintf("a=rtpmap:%d %s/%d", codec.Payload, codec.Name, codec.ClockRate)
		if codec.Channels > 1 {
			mapping += "/" + strconv.Itoa(codec.Channels)
		}
		out.Attributes = append(out.Attributes, mapping)
		for _, feedback := range codec.Feedback {
			out.Attributes = append(out.Attributes, fmt.Sprintf("a=rtcp-fb:%d %s", codec.Payload, feedback))
		}
		parameters := codec.Parameters
		if len(codec.Redundant) > 0 {
			values := []string{}
			for _, name := range codec.Redundant {
				values = append(values, strconv.Itoa(media.Codecs[name].Payload))
			}
			parameters = strings.Join(values, "/")
		}
		if parameters != "" {
			out.Attributes = append(out.Attributes, fmt.Sprintf("a=fmtp:%d %s", codec.Payload, parameters))
		}
		if codec.RTXPayload >= 0 {
			out.Attributes = append(out.Attributes, fmt.Sprintf("a=rtpmap:%d rtx/%d", codec.RTXPayload, codec.ClockRate), fmt.Sprintf("a=fmtp:%d apt=%d", codec.RTXPayload, codec.Payload))
		}
	}
	return out
}
