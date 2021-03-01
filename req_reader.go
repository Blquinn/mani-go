package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"

	"github.com/valyala/fasthttp"
)

////////////////////////////////////////
// RequestReader

const (
	urlHeader    = "x-mani-url"
	methodHeader = "x-mani-method"
)

type requestReader interface {
	// Should return io.EOF when there are no more requests.
	readNextRequest(*fasthttp.Request) error
}

///////////////////////////
// multipart reader

type multipartRequestReader struct {
	// TODO: Add /store common headers
	mpr *multipart.Reader
}

func (r multipartRequestReader) readNextRequest(req *fasthttp.Request) error {
	part, err := r.mpr.NextRawPart()
	if err != nil {
		return err
	}

	url := part.Header.Get(urlHeader)
	if url == "" {
		return errors.New("part missing url header")
	}
	part.Header.Del(urlHeader)

	method := part.Header.Get(methodHeader)
	if method == "" {
		return errors.New("part missing method header")
	}
	part.Header.Del(methodHeader)

	req.Header.SetMethod(method)
	req.SetRequestURI(url)
	for k, v := range part.Header {
		for _, vv := range v {
			req.Header.Set(k, vv)
		}
	}

	// TODO: Can we stream the body???
	body, err := ioutil.ReadAll(part)
	if err != nil {
		return err
	}

	req.SetBodyRaw(body)

	return nil
}

///////////////////////////
// json reader

type jsonRequestBody struct {
	Requests []jsonRequest `json:"requests"`
}

type jsonRequest struct {
	URL     string       `json:"url"`
	Method  string       `json:"method"`
	Headers [][]string   `json:"headers"`
	Body    ManiBodyEnum `json:"body"`
}

type jsonRequestReader struct {
	// TODO: Add /store common headers
	dec            *json.Decoder
	parsedBody     *jsonRequestBody
	currentRequest int
}

func newJsonRequestReader(reader io.Reader) *jsonRequestReader {
	return &jsonRequestReader{
		dec: json.NewDecoder(reader),
	}
}

// Init reads the head of the json body.
func (r *jsonRequestReader) init() error {
	// TODO: Streamed json parsing.

	req := new(jsonRequestBody)
	if err := r.dec.Decode(req); err != nil {
		return err
	}

	r.parsedBody = req

	return nil
}

func (r *jsonRequestReader) readNextRequest(req *fasthttp.Request) error {
	if r.currentRequest >= len(r.parsedBody.Requests) {
		return io.EOF
	}

	defer func() { r.currentRequest++ }()

	rr := r.parsedBody.Requests[r.currentRequest]

	req.Header.SetMethod(rr.Method)
	req.SetRequestURI(rr.URL)

	// TODO: Validate headers.
	for _, h := range rr.Headers {
		req.Header.Add(h[0], h[1])
	}

	// TODO: Can we stream the body???

	if rr.Body.Bytes != nil {
		req.SetBodyRaw(rr.Body.Bytes)
	} else if rr.Body.JSON != nil {
		req.SetBodyRaw(rr.Body.JSON)
	}

	return nil
}
