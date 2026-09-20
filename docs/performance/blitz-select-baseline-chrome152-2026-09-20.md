# Select baseline receipt — Chrome 152.0.7977.82

Measured frozen executable `E:/GitHub/mimic/compatibility/.chrome-for-testing/152.0.7977.82/chrome-win64/chrome.exe`, headless, viewport1272x653, standards document. No native patch derives from an unmeasured half-pixel adjustment.

Exact fixture template (`P` parent font size, `S` optional select font size):

```html
<!doctype html><body>
<div style="position:fixed;left:0;top:0;width:300px;visibility:hidden;font:Ppx Times New Roman">
  <div id="p" style="border:2px solid"><select id="s" style="font-size:Spx"><option>Alpha</option></select><span id="b" style="display:inline-block;width:0;height:0;vertical-align:baseline"></span></div>
</div>
```

Measured fixture has no whitespace between inline children. For default S the select style attribute is omitted. All select computed line-heights are `normal`; default computed font is `13.3333px Arial`, explicit sizes use Arial. `baseline` = marker.top − select.top. Parentrect is `[0,0,300,parentHeight]`, selectleft=2, markerwidth/height=0; markerleft=selectleft+selectwidth.

| P | S | Parent height | Select top | Select width | Select height | Marker top | Baseline |
| ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: |
|12|default|23|2|57|19|16|14|
|12|12|22|2|53|18|15|13|
|12|14|24|2|58|20|17|15|
|12|16|25|2|63|21|18|16|
|12|20|30|2|74|26|22|20|
|14|default|23|2|57|19|16|14|
|14|12|22|2|53|18|15|13|
|14|14|24|2|58|20|17|15|
|14|16|25|2|63|21|18|16|
|14|20|30|2|74|26|22|20|
|16|default|23|2|57|19|16|14|
|16|12|23|3|53|18|16|13|
|16|14|24|2|58|20|17|15|
|16|16|25|2|63|21|18|16|
|16|20|30|2|74|26|22|20|
|20|default|27|6|57|19|20|14|
|20|12|27|7|53|18|20|13|
|20|14|27|5|58|20|20|15|
|20|16|27|4|63|21|20|16|
|20|20|30|2|74|26|22|20|

Conclusion: native default select baseline14 itself agrees with Chrome. A native parentheight23.5 versus Chrome23 should not be repaired by inventing selectbaseline14.5. Investigate parent strut/inline ascent rounding; this receipt separates those inputs.
