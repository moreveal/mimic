package browser

import (
	"context"
	"strconv"
	"testing"
)

func TestWebAnimationReturnsCoherentPlaybackControls(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, page *Page) {
		value, err := page.Evaluate(context.Background(), `(()=>{
const element=document.createElement('div');document.body.append(element);
element.style.opacity='.2';const effectConstructed=new KeyframeEffect(element,{opacity:[0,1]},{duration:50,fill:'both'}),idle=new Animation(effectConstructed,document.timeline),animation=element.animate({opacity:[0,1]},{duration:50,fill:'both'});let finishes=0;animation.addEventListener('finish',()=>finishes++);
const effect=animation.effect,initial={animation:animation instanceof Animation,effect:effect instanceof KeyframeEffect,state:animation.playState,pending:animation.pending,current:animation.currentTime,timing:effect.getTiming(),frames:effect.getKeyframes(),elementAnimations:element.getAnimations()[0]===animation,documentAnimations:document.getAnimations()[0]===animation};
animation.startTime=performance.now()-25;const running={pending:animation.pending,start:typeof animation.startTime,current:animation.currentTime,computed:effect.getComputedTiming(),opacity:getComputedStyle(element).opacity,inline:element.style.opacity};animation.finish();const finished={state:animation.playState,current:animation.currentTime,opacity:getComputedStyle(element).opacity};animation.cancel();return{constructors:[Animation.length,KeyframeEffect.length,Element.prototype.animate.length,idle.playState,idle.currentTime,document.timeline instanceof DocumentTimeline],initial,running,finished,finishes,canceled:{state:animation.playState,current:animation.currentTime,count:document.getAnimations().length}};
})()`)
		if err != nil {
			t.Fatal(err)
		}
		got := value.(map[string]any)
		constructors := got["constructors"].([]any)
		if constructors[0] != int64(0) && constructors[0] != float64(0) || constructors[1] != int64(1) && constructors[1] != float64(1) || constructors[2] != int64(1) && constructors[2] != float64(1) || constructors[3] != "idle" || constructors[4] != nil || constructors[5] != true {
			t.Fatalf("animation constructors: %#v", constructors)
		}
		initial := got["initial"].(map[string]any)
		current, currentOK := numberParameter(initial["current"])
		if initial["animation"] != true || initial["effect"] != true || initial["state"] != "running" || initial["pending"] != true || !currentOK || current != 0 || initial["elementAnimations"] != true || initial["documentAnimations"] != true {
			t.Fatalf("initial animation: %#v", initial)
		}
		timing := initial["timing"].(map[string]any)
		duration, durationOK := numberParameter(timing["duration"])
		iterations, iterationsOK := numberParameter(timing["iterations"])
		if !durationOK || duration != 50 || timing["fill"] != "both" || !iterationsOK || iterations != 1 {
			t.Fatalf("animation timing: %#v", timing)
		}
		running := got["running"].(map[string]any)
		runningCurrent, runningCurrentOK := numberParameter(running["current"])
		if running["pending"] != false || running["start"] != "number" || !runningCurrentOK || runningCurrent <= 0 {
			t.Fatalf("running animation: %#v", running)
		}
		computed := running["computed"].(map[string]any)
		progress, progressOK := numberParameter(computed["progress"])
		iteration, iterationOK := numberParameter(computed["currentIteration"])
		if !progressOK || progress < .45 || progress > .65 || !iterationOK || iteration != 0 {
			t.Fatalf("computed animation timing: %#v", computed)
		}
		opacity, opacityErr := strconv.ParseFloat(running["opacity"].(string), 64)
		if opacityErr != nil || opacity < .45 || opacity > .65 || running["inline"] != "0.2" {
			t.Fatalf("sampled animation style: %#v", running)
		}
		finished := got["finished"].(map[string]any)
		finishedCurrent, finishedCurrentOK := numberParameter(finished["current"])
		if finished["state"] != "finished" || !finishedCurrentOK || finishedCurrent != 50 {
			t.Fatalf("finished animation: %#v", finished)
		}
		canceled := got["canceled"].(map[string]any)
		count, countOK := numberParameter(canceled["count"])
		if canceled["state"] != "idle" || canceled["current"] != nil || !countOK || count != 0 {
			t.Fatalf("canceled animation: %#v", canceled)
		}
	})
}
