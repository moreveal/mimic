package layouttaffy

/*
#cgo windows,amd64 LDFLAGS: ${SRCDIR}/../layoutblitz/native/target/x86_64-pc-windows-gnu/release/libmimic_layout_blitz.a -lws2_32 -luserenv -lbcrypt -lntdll
#cgo linux,amd64 LDFLAGS: ${SRCDIR}/../layoutblitz/native/target/x86_64-unknown-linux-gnu/release/libmimic_layout_blitz.a -ldl -lpthread -lm
#include <stdint.h>
#include <stddef.h>
typedef struct { uint32_t kind; float value; } MimicLength;
typedef struct { MimicLength top, right, bottom, left; } MimicEdges;
typedef struct { uint32_t kind; float value; } MimicTrack;
typedef struct {
 uint64_t id; int32_t parent;
 uint32_t display, position, box_sizing, flex_direction, flex_wrap;
 uint32_t align_items, align_self, align_content, justify_content, justify_self, justify_items;
 MimicLength width,height,min_width,min_height,max_width,max_height,flex_basis;
 float flex_grow,flex_shrink; MimicEdges margin,padding,border,inset;
 MimicLength gap_x,gap_y; float measure_width,measure_height;
 uint32_t grid_column_offset,grid_column_count,grid_row_offset,grid_row_count;
 int32_t grid_column_start,grid_column_end,grid_row_start,grid_row_end;
} MimicInputNode;
typedef struct { uint64_t id; float x,y,width,height,content_width,content_height; } MimicOutputBox;
int32_t mimic_taffy_layout(const MimicInputNode*, size_t, const MimicTrack*, size_t, float, float, MimicOutputBox*, size_t);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

type Length struct {
	Kind  uint32  `json:"kind"`
	Value float32 `json:"value"`
}
type Edges struct{ Top, Right, Bottom, Left Length }
type Track struct {
	Kind  uint32  `json:"kind"`
	Value float32 `json:"value"`
}
type Node struct {
	ID                                                                             uint64 `json:"id"`
	Parent                                                                         int32  `json:"parent"`
	Display, Position, BoxSizing, FlexDirection, FlexWrap                          uint32
	AlignItems, AlignSelf, AlignContent, JustifyContent, JustifySelf, JustifyItems uint32
	Width, Height, MinWidth, MinHeight, MaxWidth, MaxHeight, FlexBasis             Length
	FlexGrow, FlexShrink                                                           float32
	Margin, Padding, Border, Inset                                                 Edges
	GapX, GapY                                                                     Length
	MeasureWidth, MeasureHeight                                                    float32
	GridColumnOffset, GridColumnCount, GridRowOffset, GridRowCount                 uint32
	GridColumnStart, GridColumnEnd, GridRowStart, GridRowEnd                       int32
}
type Request struct {
	Viewport [2]float32 `json:"viewport"`
	Nodes    []Node     `json:"nodes"`
	Tracks   []Track    `json:"tracks,omitempty"`
}
type Box struct {
	ID            uint64  `json:"id"`
	X             float32 `json:"x"`
	Y             float32 `json:"y"`
	Width         float32 `json:"width"`
	Height        float32 `json:"height"`
	ContentWidth  float32 `json:"contentWidth"`
	ContentHeight float32 `json:"contentHeight"`
}
type Response struct {
	Boxes []Box  `json:"boxes,omitempty"`
	Error string `json:"error,omitempty"`
}

func cl(v Length) C.MimicLength {
	return C.MimicLength{kind: C.uint32_t(v.Kind), value: C.float(v.Value)}
}
func ce(v Edges) C.MimicEdges {
	return C.MimicEdges{top: cl(v.Top), right: cl(v.Right), bottom: cl(v.Bottom), left: cl(v.Left)}
}

func Layout(request Request) ([]Box, error) {
	if len(request.Nodes) == 0 {
		return nil, fmt.Errorf("taffy: empty tree")
	}
	in := make([]C.MimicInputNode, len(request.Nodes))
	out := make([]C.MimicOutputBox, len(request.Nodes))
	for i, n := range request.Nodes {
		in[i] = C.MimicInputNode{id: C.uint64_t(n.ID), parent: C.int32_t(n.Parent), display: C.uint32_t(n.Display), position: C.uint32_t(n.Position), box_sizing: C.uint32_t(n.BoxSizing), flex_direction: C.uint32_t(n.FlexDirection), flex_wrap: C.uint32_t(n.FlexWrap), align_items: C.uint32_t(n.AlignItems), align_self: C.uint32_t(n.AlignSelf), align_content: C.uint32_t(n.AlignContent), justify_content: C.uint32_t(n.JustifyContent), justify_self: C.uint32_t(n.JustifySelf), justify_items: C.uint32_t(n.JustifyItems), width: cl(n.Width), height: cl(n.Height), min_width: cl(n.MinWidth), min_height: cl(n.MinHeight), max_width: cl(n.MaxWidth), max_height: cl(n.MaxHeight), flex_basis: cl(n.FlexBasis), flex_grow: C.float(n.FlexGrow), flex_shrink: C.float(n.FlexShrink), margin: ce(n.Margin), padding: ce(n.Padding), border: ce(n.Border), inset: ce(n.Inset), gap_x: cl(n.GapX), gap_y: cl(n.GapY), measure_width: C.float(n.MeasureWidth), measure_height: C.float(n.MeasureHeight), grid_column_offset: C.uint32_t(n.GridColumnOffset), grid_column_count: C.uint32_t(n.GridColumnCount), grid_row_offset: C.uint32_t(n.GridRowOffset), grid_row_count: C.uint32_t(n.GridRowCount), grid_column_start: C.int32_t(n.GridColumnStart), grid_column_end: C.int32_t(n.GridColumnEnd), grid_row_start: C.int32_t(n.GridRowStart), grid_row_end: C.int32_t(n.GridRowEnd)}
	}
	tracks := make([]C.MimicTrack, len(request.Tracks))
	for i, track := range request.Tracks {
		tracks[i] = C.MimicTrack{kind: C.uint32_t(track.Kind), value: C.float(track.Value)}
	}
	var trackPointer *C.MimicTrack
	if len(tracks) > 0 {
		trackPointer = (*C.MimicTrack)(unsafe.Pointer(&tracks[0]))
	}
	count := C.mimic_taffy_layout((*C.MimicInputNode)(unsafe.Pointer(&in[0])), C.size_t(len(in)), trackPointer, C.size_t(len(tracks)), C.float(request.Viewport[0]), C.float(request.Viewport[1]), (*C.MimicOutputBox)(unsafe.Pointer(&out[0])), C.size_t(len(out)))
	if count < 0 {
		return nil, fmt.Errorf("taffy: native layout error %d", int32(count))
	}
	boxes := make([]Box, int(count))
	for i, b := range out[:count] {
		boxes[i] = Box{ID: uint64(b.id), X: float32(b.x), Y: float32(b.y), Width: float32(b.width), Height: float32(b.height), ContentWidth: float32(b.content_width), ContentHeight: float32(b.content_height)}
	}
	return boxes, nil
}

func LayoutJSON(input string) string {
	var request Request
	if err := json.Unmarshal([]byte(input), &request); err != nil {
		return encodeError(err)
	}
	boxes, err := Layout(request)
	if err != nil {
		return encodeError(err)
	}
	raw, _ := json.Marshal(Response{Boxes: boxes})
	return string(raw)
}

func encodeError(err error) string {
	raw, _ := json.Marshal(Response{Error: err.Error()})
	return string(raw)
}
