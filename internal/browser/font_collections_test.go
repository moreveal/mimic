package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestFontCollectionsOracle(t *testing.T) {
	parallelBrowserTest(t)
	for _, name := range []string{"loading", "collections", "descriptors", "constructor", "documents"} {
		t.Run(name, func(t *testing.T) {
			parallelOracle(t)
			fontOracle(t, name)
		})
	}
}
func fontOracle(t *testing.T, name string) {
	source, err := os.ReadFile("testdata/font_" + name + "_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		for _, worker := range []bool{false, true} {
			suffix := ""
			if worker {
				suffix = "-worker"
			}

			data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/font-" + name + suffix + "-chrome152.json")
			if err != nil {
				t.Fatal(err)
			}
			var capture struct {
				Result map[string]any `json:"result"`
			}
			if err = json.Unmarshal(data, &capture); err != nil {
				t.Fatal(err)
			}
			expression := "(async()=>JSON.stringify(await " + string(source) + "))()"
			if worker {
				code := "onmessage=async()=>{try{postMessage(await " + expression + ")}catch(e){postMessage(String(e))}}"
				expression = `new Promise((resolve,reject)=>{const u=URL.createObjectURL(new Blob([` + strconv.Quote(code) + `])),w=new Worker(u);w.onmessage=e=>{w.terminate();URL.revokeObjectURL(u);resolve(e.data)};w.onerror=e=>reject(Error(e.message));w.postMessage(null)})`
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			value, err := p.Evaluate(ctx, expression)
			cancel()
			if err != nil {
				t.Fatalf("worker=%v: %v", worker, err)
			}
			var actual map[string]any
			if err = json.Unmarshal([]byte(value.(string)), &actual); err != nil {
				t.Fatalf("worker=%v: %v (%v)", worker, value, err)
			}
			for key, want := range capture.Result {
				if !reflect.DeepEqual(actual[key], want) {
					t.Errorf("worker=%v %s: got %v want %v", worker, key, actual[key], want)
				}
			}
		}
	})
}

func TestFontResourceBoundaryAndDocumentOwnership(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), `(async()=>{
 const a=document.implementation.createHTMLDocument('a'),b=document.implementation.createHTMLDocument('b');
 const face=new FontFace('Example','local(mimic-no-such-font-98347)');
 a.fonts.add(face);
 if(a.fonts===b.fonts||a.fonts.size!==1||b.fonts.size!==0||document.fonts.size!==0)return false;
 const promise=face.load();
 if(promise!==face.loaded||face.status==='loaded')return false;
 if(await promise.then(()=>'',e=>e.name)!=='NetworkError')return false;
 if(a.fonts.size!==1||!a.fonts.has(face))return false;
 if(a.fonts.check('12px Example')!==false)return false;
 return await a.fonts.load('12px Example').then(()=>false,e=>e.name==='NetworkError');
})()`)
		if err != nil || value != true {
			t.Fatalf("value=%v error=%v", value, err)
		}
	})
}

func TestDocumentFontsIncludesCSSConnectedFaces(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), `JSON.stringify((()=>{
 const style=document.createElement('style');
 style.textContent='@font-face{font-family:PixelSans;src:url(https://example.test/pixel.woff2);font-weight:100 900}@font-face{font-family:"Pixel Mono";src:local(Arial)}';
 document.head.append(style);
 const first=Array.from(document.fonts),again=Array.from(document.fonts);
 const before=[document.fonts.size,first.map(face=>face.family).join('|'),first[0]===again[0],[...style.sheet.cssRules].map(rule=>[rule.style.fontFamily,rule.style.src,rule.style.fontWeight])];
 document.fonts.clear();
 const afterClear=document.fonts.size;
 style.remove();
 return [before,afterClear,document.fonts.size];
})())`)
		if err != nil {
			t.Fatal(err)
		}
		want := `[[2,"PixelSans|\"Pixel Mono\"",true,[["PixelSans","url(https://example.test/pixel.woff2)","100 900"],["\"Pixel Mono\"","local(Arial)",""]]],2,0]`
		if value != want {
			t.Fatalf("CSS-connected fonts: got %#v want %#v", value, want)
		}
	})
}
