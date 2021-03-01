package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/textproto"

	"github.com/valyala/fasthttp"
)

type responseWriter interface {
	Open() error
	WriteNextResponse(response *fasthttp.Response) error
	WriteNextError(err error) error
	Close() error
}

////////////////////////////////
// multipartResponseWriter

type multipartResponseWriter struct {
	resWriter *multipart.Writer
}

func (w multipartResponseWriter) Open() error {
	return nil
}

func (w multipartResponseWriter) WriteNextResponse(res *fasthttp.Response) error {
	mimeHeader := textproto.MIMEHeader{}
	res.Header.VisitAll(func(key, value []byte) {
		mimeHeader.Set(string(key), string(value))
	})

	writePart, err := w.resWriter.CreatePart(mimeHeader)
	if err != nil {
		return err
	}

	_, err = writePart.Write(res.Body())
	return err
}

func (w multipartResponseWriter) WriteNextError(err error) error {
	mimeHeader := textproto.MIMEHeader{}
	mimeHeader.Set("x-mani-error", err.Error())

	writePart, werr := w.resWriter.CreatePart(mimeHeader)
	if werr != nil {
		return werr
	}

	_, err = writePart.Write([]byte(err.Error()))
	return err
}

func (w multipartResponseWriter) Close() error {
	return w.resWriter.Close()
}

////////////////////////////////
// jsonResponseWriter

// TODO: Use buffered writer?

type jsonResponseWriter struct {
	body         io.Writer
	numResponses int
	responseNum  int
}

var (
	jsonHead = []byte(`{"responses":[`)
	jsonEnd  = []byte(`]}`)
	comma    = []byte(",")
)

func (w jsonResponseWriter) Open() error {
	_, err := w.body.Write(jsonHead)
	return err
}

func (w jsonResponseWriter) Close() error {
	_, err := w.body.Write(jsonEnd)
	return err
}

// TODO: Store total
func (w jsonResponseWriter) WriteNextResponse(res *fasthttp.Response) error {
	defer func() { w.responseNum++ }()

	// We could write headers directly.
	h := make([][]string, 0, res.Header.Len())
	res.Header.VisitAll(func(k, v []byte) {
		h = append(h, []string{string(k), string(v)})
	})

	body := ManiBodyEnum{}
	if bytes.HasPrefix(res.Header.ContentType(), []byte("application/json")) {
		body.JSON = res.Body()
	} else {
		body.Bytes = res.Body()
	}

	jsonRes := ManiJSONResponse{
		StatusCode: res.StatusCode(),
		Headers:    h,
		Body:       body,
	}

	bts, err := json.Marshal(ManiJSONResponseWrapper{Response: &jsonRes})
	if err != nil {
		return err
	}

	_, err = w.body.Write(bts)
	if err != nil {
		return err
	}

	if w.responseNum < w.numResponses-1 {
		_, err = w.body.Write(comma)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w jsonResponseWriter) WriteNextError(err error) error {
	bts, err := json.Marshal(ManiJSONResponseWrapper{Error: &ManiJSONError{Detail: err.Error()}})
	if err != nil {
		return err
	}

	_, err = w.body.Write(bts)
	if err != nil {
		return err
	}

	if w.responseNum < w.numResponses-1 {
		_, err = w.body.Write(comma)
		if err != nil {
			return err
		}
	}

	return nil
}
