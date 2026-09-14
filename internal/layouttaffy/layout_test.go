package layouttaffy

import (
	"encoding/json"
	"testing"
)

func TestNestedFlexBasis(t *testing.T) {
	auto := Length{}
	px := func(v float32) Length { return Length{Kind: 1, Value: v} }
	pct := func(v float32) Length { return Length{Kind: 2, Value: v} }
	nodes := []Node{
		{ID: 1, Parent: -1, Display: 2, Width: px(400), Height: auto, MeasureWidth: -1, MeasureHeight: -1},
		{ID: 2, Parent: 0, Display: 1, Width: px(40), MeasureWidth: -1, MeasureHeight: -1},
		{ID: 3, Parent: 0, Display: 2, FlexBasis: pct(0), FlexGrow: 1, FlexShrink: 1, MinWidth: px(0), MeasureWidth: -1, MeasureHeight: -1},
		{ID: 4, Parent: 2, Display: 2, FlexBasis: pct(1), FlexGrow: 1, FlexShrink: 1, MinWidth: px(0), MeasureWidth: 168, MeasureHeight: 36},
		{ID: 5, Parent: 0, Display: 1, Width: px(40), MeasureWidth: -1, MeasureHeight: -1},
	}
	boxes, err := Layout(Request{Viewport: [2]float32{800, 600}, Nodes: nodes})
	if err != nil {
		t.Fatal(err)
	}
	if boxes[2].Width != 320 || boxes[3].Width != 320 {
		t.Fatalf("nested flex widths: slot=%v textarea=%v", boxes[2].Width, boxes[3].Width)
	}
}

func TestJSONAdapterPreservesStyles(t *testing.T) {
	px := func(v float32) Length { return Length{Kind: 1, Value: v} }
	request := Request{Viewport: [2]float32{300, 600}, Nodes: []Node{
		{ID: 1, Parent: -1, Display: 2, Width: px(300), MeasureWidth: -1, MeasureHeight: -1},
		{ID: 2, Parent: 0, Display: 1, Width: px(200), FlexShrink: 1, MeasureWidth: -1, MeasureHeight: -1},
		{ID: 3, Parent: 0, Display: 1, Width: px(200), FlexShrink: 1, MeasureWidth: -1, MeasureHeight: -1},
		{ID: 4, Parent: 0, Display: 1, Width: px(200), FlexShrink: 1, MeasureWidth: -1, MeasureHeight: -1},
	}}
	raw, _ := json.Marshal(request)
	var response Response
	if err := json.Unmarshal([]byte(LayoutJSON(string(raw))), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != "" || response.Boxes[1].Width != 100 {
		t.Fatalf("JSON adapter: %s => %#v", raw, response)
	}
}

func TestFlexShrinkAndAutoMargin(t *testing.T) {
	px := func(v float32) Length { return Length{Kind: 1, Value: v} }
	autoEdges := Edges{Left: Length{}}
	nodes := []Node{{ID: 1, Parent: -1, Display: 2, Width: px(300), MeasureWidth: -1, MeasureHeight: -1},
		{ID: 2, Parent: 0, Display: 1, Width: px(200), FlexShrink: 1, MeasureWidth: -1, MeasureHeight: -1},
		{ID: 3, Parent: 0, Display: 1, Width: px(200), FlexShrink: 1, MeasureWidth: -1, MeasureHeight: -1},
		{ID: 4, Parent: 0, Display: 1, Width: px(200), FlexShrink: 1, Margin: autoEdges, MeasureWidth: -1, MeasureHeight: -1}}
	boxes, err := Layout(Request{Viewport: [2]float32{800, 600}, Nodes: nodes})
	if err != nil {
		t.Fatal(err)
	}
	if boxes[1].Width != 100 || boxes[2].Width != 100 || boxes[3].Width != 100 {
		t.Fatalf("flex shrink: %#v", boxes)
	}
}

func TestNestedFlexItemsShrink(t *testing.T) {
	px := func(v float32) Length { return Length{Kind: 1, Value: v} }
	nodes := []Node{{ID: 1, Parent: -1, Display: 2, Width: px(300), MeasureWidth: -1, MeasureHeight: -1}}
	for i := 0; i < 3; i++ {
		parent := int32(len(nodes))
		nodes = append(nodes, Node{ID: uint64(2 + i*2), Parent: 0, Display: 2, FlexShrink: 1, MinWidth: px(0), MeasureWidth: -1, MeasureHeight: -1})
		nodes = append(nodes, Node{ID: uint64(3 + i*2), Parent: parent, Display: 1, Width: px(200), MeasureWidth: -1, MeasureHeight: -1})
	}
	boxes, err := Layout(Request{Viewport: [2]float32{800, 600}, Nodes: nodes})
	if err != nil {
		t.Fatal(err)
	}
	if boxes[1].Width != 100 || boxes[3].Width != 100 || boxes[5].Width != 100 {
		t.Fatalf("nested flex shrink: %#v", boxes)
	}
}

func TestGridTracksGapAndPlacement(t *testing.T) {
	px := func(v float32) Length { return Length{Kind: 1, Value: v} }
	nodes := []Node{
		{ID: 1, Parent: -1, Display: 3, Width: px(300), Height: px(40), GridColumnCount: 3, GapX: px(10), AlignItems: 7, JustifyItems: 7, MeasureWidth: -1, MeasureHeight: -1},
		{ID: 2, Parent: 0, Display: 1, Width: px(100), GridColumnStart: 1, MeasureWidth: -1, MeasureHeight: -1},
		{ID: 3, Parent: 0, Display: 1, Width: px(190), GridColumnStart: 2, GridColumnEnd: -2, MeasureWidth: -1, MeasureHeight: -1},
	}
	tracks := []Track{{Kind: 1, Value: 100}, {Kind: 3, Value: 1}, {Kind: 1, Value: 50}}
	boxes, err := Layout(Request{Viewport: [2]float32{800, 600}, Nodes: nodes, Tracks: tracks})
	if err != nil {
		t.Fatal(err)
	}
	if boxes[1].X != 0 || boxes[1].Width != 100 || boxes[2].X != 110 || boxes[2].Width != 190 {
		t.Fatalf("grid tracks: %#v", boxes)
	}
}
