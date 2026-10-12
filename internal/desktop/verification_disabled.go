//go:build !desktopverify

package desktop

import "net/http"

func registerVerification(_ *http.ServeMux, _ func(http.HandlerFunc) http.HandlerFunc) {}
