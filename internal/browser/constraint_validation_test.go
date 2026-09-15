package browser

import (
	"context"
	"encoding/json"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"os"
	"reflect"
	"testing"
)

func TestConstraintValidationMatchesChrome152(t *testing.T) {
	p := validationPage(t)
	defer p.Close()
	probe, err := os.ReadFile("testdata/constraint_validation_probe.js")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/constraint_validation_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Result any `json:"result"`
	}
	if err = json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	actual, err := p.Evaluate(context.Background(), string(probe))
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	var normalized any
	json.Unmarshal(serialized, &normalized)
	if !reflect.DeepEqual(normalized, oracle.Result) {
		t.Fatalf("constraint validation differs: %s", serialized)
	}
}

func TestConstraintValidationEventsAndSubmission(t *testing.T) {
	p := validationPage(t)
	defer p.Close()
	result, err := p.Evaluate(context.Background(), `(()=>{
 const form=document.createElement('form');form.innerHTML='<input required><button>Send</button>';document.body.appendChild(form);
 const input=form.querySelector('input'),button=form.querySelector('button'),events=[];
 input.addEventListener('invalid',e=>{events.push('invalid');e.preventDefault()});form.addEventListener('submit',e=>{events.push('submit');e.preventDefault()});
 button.click();input.value='ok';button.click();input.setCustomValidity('custom');const object=input.validity;form.reset();
 const customPersists=object.customError&&input.validationMessage==='custom';input.setCustomValidity('');input.disabled=true;
 return events.join('|')==='invalid|submit'&&customPersists&&input.checkValidity()&&input.validity===object;
 })()`)
	if err != nil || result != true {
		t.Fatalf("form validation: %v %v", result, err)
	}
}

func validationPage(t *testing.T) *Page {
	t.Helper()
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	t.Cleanup(func() { c.Close() })
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConstraintValidationIsolatedWorldSharesOwnerState(t *testing.T) {
	p := validationPage(t)
	defer p.Close()
	ctx := context.Background()
	if _, err := p.Evaluate(ctx, `document.body.innerHTML='<input id="entry" required>';document.getElementById('entry').setCustomValidity('main error')`); err != nil {
		t.Fatal(err)
	}
	world, err := p.IsolatedWorld(ctx, p.Top.ID, "validation")
	if err != nil {
		t.Fatal(err)
	}
	debugger := NewDebugger(p)
	defer debugger.Close()
	got, err := debugger.Evaluate(ctx, p.Top.ID, world, `(()=>{const e=document.getElementById('entry'),v=e.validity,was=v.customError&&e.validationMessage==='main error';e.setCustomValidity('');e.value='filled';return was&&v===e.validity&&v.valid&&e.checkValidity()})()`, DebuggerOptions{ReturnByValue: true})
	if err != nil || got["result"].(map[string]any)["value"] != true {
		t.Fatalf("isolated validity: %v %v", got, err)
	}
	value, err := p.Evaluate(ctx, `(()=>{const entry=document.getElementById('entry');return entry.value==='filled'&&entry.validity.valid&&!entry.validity.customError})()`)
	if err != nil || value != true {
		t.Fatalf("owner validity: %v %v", value, err)
	}
}

func TestConstraintValidationUserNumberInput(t *testing.T) {
	p := validationPage(t)
	defer p.Close()
	ctx := context.Background()
	for _, tc := range []struct {
		text, value         string
		bad, missing, valid bool
	}{{"-", "", true, true, false}, {"1e", "", true, true, false}, {"abc", "", false, true, false}, {"1.5", "1.5", false, false, false}, {"1e2", "1e2", false, false, true}} {
		if _, err := p.Evaluate(ctx, `document.body.innerHTML='<input type="number" required>';document.querySelector('input').focus()`); err != nil {
			t.Fatal(err)
		}
		if err := p.DispatchProtocolInput(ctx, "Input.insertText", map[string]any{"text": tc.text}); err != nil {
			t.Fatal(err)
		}
		got, err := p.Evaluate(ctx, `(()=>{const e=document.querySelector('input');return [e.value,e.validity.badInput,e.validity.valueMissing,e.validity.valid]})()`)
		if err != nil || !reflect.DeepEqual(got, []any{tc.value, tc.bad, tc.missing, tc.valid}) {
			t.Fatalf("insert %q: %v %v", tc.text, got, err)
		}
		got, err = p.Evaluate(ctx, `(()=>{const e=document.querySelector('input');e.value='';return !e.validity.badInput})()`)
		if err != nil || got != true {
			t.Fatalf("programmatic reset: %v %v", got, err)
		}
	}
}

func TestValidityStateBorrowedAcrossRealms(t *testing.T) {
	p := validationPage(t)
	defer p.Close()
	ctx := context.Background()
	value, err := p.Evaluate(ctx, `new Promise(resolve=>{const frame=document.createElement('iframe');frame.onload=()=>{const e=frame.contentDocument.createElement('input');e.required=true;frame.contentDocument.body.appendChild(e);const v=e.validity,get=Object.getOwnPropertyDescriptor(ValidityState.prototype,'valid').get;const before=get.call(v);e.value='ok';resolve(!before&&get.call(v)&&v===e.validity&&Object.getPrototypeOf(v)===frame.contentWindow.ValidityState.prototype)};frame.srcdoc='<body></body>';document.body.appendChild(frame)})`)
	if err != nil || value != true {
		t.Fatalf("cross-realm validity: %v %v", value, err)
	}
}

func TestConstraintValidationPatternAndNeighboringFlags(t *testing.T) {
	p := validationPage(t)
	defer p.Close()
	value, err := p.Evaluate(context.Background(), `(()=>{
 const pattern=document.createElement('input');pattern.pattern='[a-z]{3}';pattern.value='ABC';
 const email=document.createElement('input');email.type='email';email.value='missing-at';
 const number=document.createElement('input');number.type='number';number.min='2';number.max='8';number.step='2';number.value='5';
 const custom=document.createElement('textarea');custom.setCustomValidity('problem');
 const disabled=document.createElement('input');disabled.required=true;disabled.disabled=true;
 return pattern.validity.patternMismatch&&!pattern.validity.valid&&email.validity.typeMismatch&&
   number.validity.stepMismatch&&!number.validity.rangeUnderflow&&!number.validity.rangeOverflow&&
   custom.validity.customError&&custom.validationMessage==='problem'&&!custom.checkValidity()&&
   !disabled.willValidate&&disabled.validity.valid;
 })()`)
	if err != nil || value != true {
		t.Fatalf("neighboring validity flags: %v %v", value, err)
	}
}
