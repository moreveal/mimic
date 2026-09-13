package browser

import "strings"

// Chrome 152 rewrites existing charset values in place for XHR string bodies.
// Preserve header spelling/quotes, including Blink's legacy scan inside quoted
// parameters. A MIME parse-and-reserialize changes observable header bytes.
// Reference: xml_http_request.cc, Chromium d04cdb24d67b.
func xhrStringContentType(value string) string {
	if value == "" {
		return "text/plain;charset=UTF-8"
	}
	for scan := 0; scan < len(value); {
		index := strings.Index(strings.ToLower(value[scan:]), "charset")
		if index < 0 {
			break
		}
		index += scan
		if index == 0 {
			break
		}
		scan = index + len("charset")
		if value[index-1] > ' ' && value[index-1] != ';' {
			continue
		}
		for scan < len(value) && value[scan] <= ' ' {
			scan++
		}
		if scan >= len(value) || value[scan] != '=' {
			continue
		}
		scan++
		for scan < len(value) && (value[scan] <= ' ' || value[scan] == '"' || value[scan] == '\'') {
			scan++
		}
		end := scan
		for end < len(value) && value[end] > ' ' && value[end] != '"' && value[end] != '\'' && value[end] != ';' {
			end++
		}
		if scan == end {
			break
		}
		value = value[:scan] + "UTF-8" + value[end:]
		scan += len("UTF-8")
	}
	return value
}
