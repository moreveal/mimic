//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"encoding/json"
	"github.com/moreveal/mimic/internal/trace"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestConsoleFamilyFrozenChromeAndRestoredRealms(t *testing.T) {
	serialBrowserTest(t)
	source, err := os.ReadFile("testdata/console_family_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	capture, err := os.ReadFile("testdata/console_family_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Result struct {
			Result struct {
				Value map[string]any `json:"value"`
			} `json:"result"`
		} `json:"result"`
	}
	if err = json.Unmarshal(capture, &oracle); err != nil {
		t.Fatal(err)
	}
	trim := func(v map[string]any) { // Profiling/timestamp instrumentation is outside this console package.
		for _, section := range []string{"shape", "returns"} {
			for _, key := range []string{"profile", "profileEnd", "timeStamp"} {
				delete(v[section].(map[string]any), key)
			}
		}
	}
	trim(oracle.Result.Result.Value)
	for _, mode := range []string{"ordinary", "restored"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "ordinary" {
				t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "1")
			} else {
				t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "0")
			}
			seed := bootstrapSnapshotPage(t)
			navigateCapabilityFixture(t, seed)
			if mode == "restored" {
				bootstrapSnapshotWarm(t, seed)
			}
			p, err := seed.ctx.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			navigateCapabilityFixture(t, p)
			if mode == "restored" && !p.Top.Realm.bootstrapRestored {
				t.Fatal("not restored")
			}
			for _, realm := range []string{"window", "iframe", "worker"} {
				t.Run(realm, func(t *testing.T) {
					expression := string(source)
					switch realm {
					case "iframe":
						expression = `(()=>{const f=document.createElement('iframe');document.body.appendChild(f);try{return f.contentWindow.eval(` + strconv.Quote(expression) + `)}finally{f.remove()}})()`
					case "worker":
						expression = `new Promise((resolve,reject)=>{const url=URL.createObjectURL(new Blob([` + strconv.Quote("postMessage("+expression+")") + `],{type:'text/javascript'}));const w=new Worker(url);w.onmessage=e=>{w.terminate();URL.revokeObjectURL(url);resolve(e.data)};w.onerror=e=>{w.terminate();URL.revokeObjectURL(url);reject(Error(e.message))}})`
					}
					value, err := p.Evaluate(context.Background(), expression)
					if err != nil {
						t.Fatal(err)
					}
					actual := value.(map[string]any)
					trim(actual)
					if difference := bootstrapSnapshotDifference("console", oracle.Result.Result.Value, actual); difference != "" {
						t.Fatal(difference)
					}
				})
			}
		})
	}
}

func TestConsoleDescriptionNativeBrandsAndFailures(t *testing.T) {
	parallelBrowserTest(t)
	p := bootstrapSnapshotPage(t)
	historyEval(t, p, `(()=>{for(const make of [()=>function(){},()=>/x/,()=>new Date(0),()=>new Error('x')]){let n=0;const v=make();v.toString=()=>{n++;throw Error('description failure')};console.log(v);if(n!==1)return 'native description';const proxy=new Proxy(v,{get(){throw Error('proxy get')},getPrototypeOf(){throw Error('proxy prototype')}});console.log(proxy);if(n!==1)return 'proxy description'}let reads=0;const plain={get toString(){reads++;throw Error('ordinary object')}};console.dir(plain);console.table(plain);return reads===0})()`, true)
}

func TestConsoleForeignArgumentsFrozenChrome(t *testing.T) {
	serialBrowserTest(t)
	source, err := os.ReadFile("testdata/console_cross_realm_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/console_cross_realm_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		Observation map[string]any `json:"observation"`
	}
	if err = json.Unmarshal(data, &capture); err != nil {
		t.Fatal(err)
	}
	for _, restored := range []bool{false, true} {
		t.Run(strconv.FormatBool(restored), func(t *testing.T) {
			t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", map[bool]string{false: "1", true: "0"}[restored])
			seed := bootstrapSnapshotPage(t)
			navigateCapabilityFixture(t, seed)
			if restored {
				bootstrapSnapshotWarm(t, seed)
			}
			p, err := seed.ctx.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			navigateCapabilityFixture(t, p)
			value, err := p.Evaluate(context.Background(), string(source))
			if err != nil {
				t.Fatal(err)
			}
			if d := bootstrapSnapshotDifference("console foreign", capture.Observation, value); d != "" {
				t.Fatal(d)
			}
		})
	}
}

func TestConsoleTimerFailedLabelStillUsesDefault(t *testing.T) {
	parallelBrowserTest(t)
	p := bootstrapSnapshotPage(t)
	historyEval(t, p, `(()=>{const failure={};for(const method of ['time','timeLog','timeEnd']){let caught=false;try{console[method]({toString(){throw failure}})}catch(e){caught=e===failure}if(!caught)return false}console.timeEnd();return true})()`, true)
	var kinds []string
	for _, e := range p.Trace().Events() {
		if e.Kind == trace.Console {
			kinds = append(kinds, e.Name)
		}
	}
	if strings.Join(kinds, ",") != "log,timeEnd,warning" {
		t.Fatalf("default timer transitions: %v", kinds)
	}
}
