package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeolocationOverrideCoordinatesAndSubscriptions(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	navigateCapabilityFixture(t, p)
	if err := p.ctx.SetPermission(originOf(p.URL()), "geolocation", "granted"); err != nil {
		t.Fatal(err)
	}
	latitude, longitude, accuracy := 41.0, 44.0, 5.0
	if err := p.SetGeolocationOverride(&GeolocationOverride{Latitude: &latitude, Longitude: &longitude, Accuracy: &accuracy}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	value, err := p.Evaluate(ctx, `new Promise(resolve => navigator.geolocation.getCurrentPosition(position => resolve(position.coords.latitude === 41 && position.coords.longitude === 44 && position.coords.accuracy === 5 && position.coords.altitude === null && position.timestamp > 0 && position instanceof GeolocationPosition && position.coords instanceof GeolocationCoordinates && position.toJSON().coords.latitude === 41),error => resolve(error.code)))`)
	if err != nil || value != true {
		t.Fatalf("position = %#v, %v", value, err)
	}
	value, err = p.Evaluate(ctx, `(async()=>{
	 const get = options => new Promise(resolve => navigator.geolocation.getCurrentPosition(resolve, resolve, options));
	 const a = await get();
	 const b = await get({maximumAge: 100000, timeout: 0});
	 const zero = await get({timeout: 0});
	 const json = a.toJSON();
	 a.coords.toJSON = () => { throw Error('script override'); };
	 return a === b && a.coords === b.coords && a.timestamp === b.timestamp && zero.code === 3 && zero.message === 'Timeout expired' && a.toJSON().coords.latitude === json.coords.latitude;
	})()`)
	if err != nil || value != true {
		t.Fatalf("cache/timeout = %#v, %v", value, err)
	}
	other, err := p.ctx.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if err = other.Navigate(ctx, p.URL()); err != nil {
		t.Fatal(err)
	}
	value, err = other.Evaluate(ctx, `new Promise(resolve=>navigator.geolocation.getCurrentPosition(()=>resolve(false),e=>resolve(e.code===2)))`)
	if err != nil || value != true {
		t.Fatalf("Page isolation = %#v, %v", value, err)
	}
	if err = p.Navigate(ctx, p.URL()); err != nil {
		t.Fatal(err)
	}
	value, err = p.Evaluate(ctx, `new Promise(resolve=>navigator.geolocation.getCurrentPosition(p=>resolve(p.coords.latitude===41),()=>resolve(false)))`)
	if err != nil || value != true {
		t.Fatalf("navigation persistence = %#v, %v", value, err)
	}
	_, err = p.Evaluate(ctx, `globalThis.geoUpdates=[];globalThis.geoWatch=navigator.geolocation.watchPosition(p=>geoUpdates.push(p.coords.latitude));`)
	if err != nil {
		t.Fatal(err)
	}
	latitude = 42
	if err = p.SetGeolocationOverride(&GeolocationOverride{Latitude: &latitude, Longitude: &longitude, Accuracy: &accuracy}); err != nil {
		t.Fatal(err)
	}
	value, err = p.Evaluate(ctx, `new Promise(resolve=>setTimeout(()=>resolve(geoUpdates.includes(42)),10))`)
	if err != nil || value != true {
		t.Fatalf("watch update = %#v, %v", value, err)
	}
	_, err = p.Evaluate(ctx, `navigator.geolocation.clearWatch(geoWatch);geoUpdates.length=0`)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.SetGeolocationOverride(&GeolocationOverride{}); err != nil {
		t.Fatal(err)
	}
	value, err = p.Evaluate(ctx, `new Promise(resolve=>navigator.geolocation.getCurrentPosition(()=>resolve(false),e=>resolve(e.code===2&&e.message===''&&geoUpdates.length===0)))`)
	if err != nil || value != true {
		t.Fatalf("unavailable = %#v, %v", value, err)
	}
	latitude = 100
	if err = p.SetGeolocationOverride(&GeolocationOverride{Latitude: &latitude}); err == nil {
		t.Fatal("invalid latitude accepted")
	}
}

func TestGeolocationPermissionsPolicyDeniesOverride(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Permissions-Policy", "geolocation=()")
		fmt.Fprint(w, "<!doctype html>")
	}))
	defer fixture.Close()
	if err := p.Navigate(context.Background(), fixture.URL); err != nil {
		t.Fatal(err)
	}
	if err := p.ctx.SetPermission(fixture.URL, "geolocation", "granted"); err != nil {
		t.Fatal(err)
	}
	latitude, longitude, accuracy := 1.0, 2.0, 3.0
	if err := p.SetGeolocationOverride(&GeolocationOverride{Latitude: &latitude, Longitude: &longitude, Accuracy: &accuracy}); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(context.Background(), `new Promise(resolve=>navigator.geolocation.getCurrentPosition(()=>resolve(false),e=>resolve(e.code===1)))`)
	if err != nil || value != true {
		t.Fatalf("policy: %#v, %v", value, err)
	}
}
