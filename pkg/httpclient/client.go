package httpclient

import "net/http"

// Client is a shared *http.Client backed by http.DefaultTransport, which reads
// HTTP_PROXY, HTTPS_PROXY, and NO_PROXY from the environment.
var Client = &http.Client{Transport: http.DefaultTransport}
