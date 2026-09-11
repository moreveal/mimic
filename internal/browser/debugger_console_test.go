package browser

import (
	"context"
	"testing"
)

func TestDebuggerConsoleArgumentsRemainLiveObjects(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		d := NewDebugger(page)
		defer d.Close()
		var events [][]any
		d.Console = func(realmID, name string, args []any) {
			if name == "log" {
				events = append(events, args)
			}
		}
		debuggerEval(t, d, `globalThis.logged={nested:{answer:42}};console.log('label',logged,null,undefined,NaN);logged.nested.answer=43`, DebuggerOptions{})
		if len(events) != 1 || len(events[0]) != 5 {
			t.Fatalf("console events: %#v", events)
		}
		id := events[0][1].(map[string]any)["objectId"].(string)
		result, err := d.CallFunction(context.Background(), "", "", `function(){return this===logged && this.nested.answer===43}`, map[string]any{"objectId": id}, DebuggerOptions{})
		if err != nil || result["result"].(map[string]any)["value"] != true {
			t.Fatalf("console identity %#v %v", result, err)
		}
		if events[0][2].(map[string]any)["subtype"] != "null" || events[0][3].(map[string]any)["type"] != "undefined" || events[0][4].(map[string]any)["unserializableValue"] != "NaN" {
			t.Fatal(events)
		}
	})
}
