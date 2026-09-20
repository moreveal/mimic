package layoutblitz

import (
	"github.com/moreveal/mimic/internal/dom"
	"testing"
)

func TestTableInternalRectsUseTracksNotSpanningCellUnion(t *testing.T) {
	document, err := dom.Parse(`<html><head><style>table{border-spacing:0}td{padding:0;width:40px}</style></head><body><table id="table"><tbody id="group"><tr id="first"><td id="span" rowspan="2" style="height:80px"></td><td style="height:20px"></td></tr><tr id="second"><td style="height:20px"></td></tr></tbody></table></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := document.DerivedSnapshot()
	owner, err := FromSnapshot(snapshot, 1280, 800)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := owner.Resolve(0); err != nil {
		t.Fatal(err)
	}
	rects := map[string]Rect{}
	for _, node := range snapshot.Nodes {
		if id := node.Attributes["id"]; id != "" {
			rect, err := owner.Rect(uint64(node.ID))
			if err != nil {
				t.Fatal(err)
			}
			rects[id] = rect
		}
	}
	first, second, group, span := rects["first"], rects["second"], rects["group"], rects["span"]
	if first.Width <= 0 || first.Height <= 0 || second.Width <= 0 || second.Height <= 0 {
		t.Fatalf("missing internal boxes: %+v", rects)
	}
	if first.Height >= span.Height || first.Y+first.Height != second.Y {
		t.Fatalf("row geometry includes spanning cell instead of row track: %+v", rects)
	}
	if group.Y != first.Y || group.Height != first.Height+second.Height || group.Width != first.Width {
		t.Fatalf("group not actual track interval: %+v", rects)
	}
}
