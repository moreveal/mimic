package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestOfflineAudioOracles(t *testing.T) {
	for _, name := range []string{"audio_buffers", "offline_audio", "audio_graph", "audio_relations", "audio_lifecycle", "audio_validation", "audio_channels"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/" + name + "_oracle.js")
			if err != nil {
				t.Fatal(err)
			}
			captures := map[string]string{"audio_buffers": "audio-buffers", "offline_audio": "offline-audio", "audio_graph": "audio-graph", "audio_relations": "audio-relations", "audio_lifecycle": "audio-lifecycle", "audio_validation": "audio-validation", "audio_channels": "audio-channels"}
			data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/" + captures[name] + "-chrome152.json")
			if err != nil {
				t.Fatal(err)
			}
			var capture struct {
				Result map[string]any `json:"result"`
			}
			if err = json.Unmarshal(data, &capture); err != nil {
				t.Fatal(err)
			}
			historyTestPages(t, func(t *testing.T, p *Page) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				value, err := p.Evaluate(ctx, "(async()=>JSON.stringify(await "+string(source)+"))()")
				if err != nil {
					t.Fatal(err)
				}
				var actual map[string]any
				if err = json.Unmarshal([]byte(value.(string)), &actual); err != nil {
					t.Fatal(err)
				}
				for key, want := range capture.Result {
					if !reflect.DeepEqual(actual[key], want) {
						t.Errorf("%s: got %v want %v", key, actual[key], want)
					}
				}
			})
		})
	}
}

func TestOfflineAudioUnsupportedGraphRejects(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(async()=>{
 const c=new OfflineAudioContext(1,8,8000),source=c.createBufferSource();
 source.buffer=c.createBuffer(1,4,8000);source.connect(c.destination);source.start(.5/8000);
 let complete=false;c.oncomplete=()=>{complete=true};
 if(await c.startRendering().then(()=>'',e=>e.name)!=='NotSupportedError'||complete||c.state!=='closed')return false;
 if(await c.startRendering().then(()=>'',e=>e.name)!=='InvalidStateError')return false;
 const d=new OfflineAudioContext(1,8,8000);
 try{d.createOscillator();return false}catch(e){if(e.name!=='NotSupportedError')return false}
 return await d.decodeAudioData(new ArrayBuffer(8)).then(()=>false,e=>e.name==='NotSupportedError');
})()`)
		if err != nil || value != true {
			t.Fatalf("value=%v error=%v", value, err)
		}
	})
}
