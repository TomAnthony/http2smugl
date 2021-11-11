package main

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
	"bytes"
)

type RequestParams struct {
	Target              *url.URL
	Method, ConnectAddr string
	Headers             Headers
	NoAutoHeaders       bool
	NoUserAgent         bool
	Body                []byte
	ShowRequest         bool
	Timeout             time.Duration
	AddContentLength    bool
}

type HTTPMessage struct {
	Headers Headers
	Body    []byte
	Trailers Headers
}

type ConnDropError struct {
	Wrapped error
}

func (r ConnDropError) Error() string {
	return fmt.Sprintf("server dropped connection, error=%v", r.Wrapped)
}

type TimeoutError struct {
}

func (t TimeoutError) Error() string {
	return "timeout"
}

func (t TimeoutError) Timeout() bool {
	return true
}

func (t TimeoutError) Temporary() bool {
	return false
}

func DoRequest(params *RequestParams) (*HTTPMessage, error) {
	var proto string

	switch params.Target.Scheme {
	case "https", "https+http2":
		proto = "http2"
	case "https+h3":
		proto = "http3"
	default:
		return nil, fmt.Errorf(`invalid scheme: %#v`, params.Target.Scheme)
	}

	var headers Headers
	var trailers Headers
	trailers = Headers{
		// {"get /headers_log.php?foo", " HTTP/1.1"},
		// {"host", "www.tomanthony.co.uk"},
		// {"get /robots.txt?foo", " HTTP/1.1"},
		// {"host", "www.apple.com"},
		// {"debugh2", "value"},
		// {"x:", " "},
		// {"get /headers_log.php?moof\r", " "},
		// {"host", "www.tomanthony.co.uk"},
		// {":authority", "takeawaypay.azurefd.net"},
		{"xauthority", params.Target.Host},
		{"xmethod", "GET"},
		{"xpath", "/robots2.txt"},
		{"xscheme", "https"},
	}

	if params.NoAutoHeaders {
		headers = params.Headers
	} else {
		headers = Headers{
			{":authority", params.Target.Host},
			{":method", params.Method},
			{":path", params.Target.Path},
			{":scheme", "https"},
		}

		if !params.NoUserAgent {
			headers = append(headers, Header{"user-agent", "Mozilla/5.0"})
		}

		toSkip := make(map[string]struct{})
		for i := range headers {
			h := &headers[i]
			if v, ok := params.Headers.Get(h.Name); ok {
				h.Value = v
				toSkip[h.Name] = struct{}{}
			}
		}

		for _, h := range params.Headers {
			if _, ok := toSkip[h.Name]; ok {
				delete(toSkip, h.Name)
				continue
			}
			headers = append(headers, h)
		}
	}

	if params.AddContentLength {
		headers = append(headers, Header{"content-length", strconv.Itoa(len(params.Body))})
	}

	targetAddr := params.ConnectAddr
	if targetAddr == "" {
		targetAddr = params.Target.Host
	}

	if params.ShowRequest {
		// Print the request
		for _, h := range headers {
			fmt.Printf("%s: %s\n", h.Name, h.Value)
		}

		fmt.Println()
		lines := bytes.Split(params.Body, []byte{'\n'})
		for _, l := range lines {
			fmt.Println(string(l))
		}
		fmt.Println()
		for _, h := range trailers {
			fmt.Printf("%s: %s\n", h.Name, h.Value)
		}

		fmt.Println()
	}

	switch proto {
	case "http2":
		return sendHTTP2Request(targetAddr, params.Target.Host, false, &HTTPMessage{headers, params.Body, trailers}, params.Timeout)
	case "http3":
		return sendHTTP3Request(targetAddr, params.Target.Host, false, &HTTPMessage{headers, params.Body, trailers}, params.Timeout)
	default:
		panic(fmt.Errorf("invalid proto: %#v", proto))
	}
}
