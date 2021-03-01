package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type ManiServer struct {
	logger     *zap.Logger
	httpClient *fasthttp.Client
}

func NewManiServer(logger *zap.Logger) *ManiServer {
	return &ManiServer{
		logger:     logger,
		httpClient: &fasthttp.Client{}, // TODO: Set timeouts etc.
	}
}

type responseType uint

const (
	responseTypeMultipart responseType = iota
	responseTypeJson
)

func (s *ManiServer) HandleMani(ctx *fasthttp.RequestCtx) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("Recovered from panic.")
			ctx.SetStatusCode(500)
		}
	}()

	rr, resType, err := s.parseRequest(ctx)
	if err != nil {
		ctx.SetStatusCode(400)
		ctx.SetBodyString(fmt.Sprintf("Failed to parse request: %s.", err))
		return
	}

	var resWriter responseWriter

	switch resType {
	case responseTypeJson:
		ctx.Response.Header.SetContentType("application/json")
		body := ctx.Response.BodyWriter()
		resWriter = jsonResponseWriter{
			body:         body,
			numResponses: len(rr.(*jsonRequestReader).parsedBody.Requests),
		}
	case responseTypeMultipart:
		mpWriter := multipart.NewWriter(ctx.Response.BodyWriter())
		ctx.Response.Header.SetContentType("multipart/mixed; boundary=" + mpWriter.Boundary())
		resWriter = multipartResponseWriter{resWriter: mpWriter}
	}

	if err := resWriter.Open(); err != nil {
		s.logger.Error("Failed to open response: %s", zap.Error(err))
		ctx.Response.Header.SetContentType("text/plain")
		ctx.Response.SetBodyString(fmt.Sprintf("Failed to open response: %s", err))
		return
	}

	// TODO: Perform requests concurrently.
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

Loop:
	for {
		req.Reset()
		res.Reset()

		err := rr.readNextRequest(req)
		switch err {
		case io.EOF:
			break Loop
		case nil:
			break
		default:
			s.logger.Error("Failed to read request.", zap.Error(err))
			ctx.SetBodyString(fmt.Sprintf("Failed to read multipart message: %s", err))
			ctx.SetStatusCode(400)
			return
		}

		// TODO: If one of these fails the response will be garbled.

		if err := s.httpClient.Do(req, res); err != nil {
			s.logger.Warn("Request failed.", zap.Error(err))

			if err := resWriter.WriteNextError(err); err != nil {
				s.logger.Error("Failed to write error.", zap.Error(err))
				return
			}
			continue
		}

		if err := resWriter.WriteNextResponse(res); err != nil {
			s.logger.Error("Failed to write response part.", zap.Error(err))
			ctx.SetBodyString(fmt.Sprintf("Failed to write part %s.", err))
			ctx.SetStatusCode(500)
			return
		}
	}

	if err := resWriter.Close(); err != nil {
		s.logger.Error("Failed to close response writer.", zap.Error(err))
		ctx.SetBodyString(fmt.Sprintf("Failed to write part %s.", err))
		ctx.SetStatusCode(500)
		return
	}
}

func (s *ManiServer) parseRequest(ctx *fasthttp.RequestCtx) (requestReader, responseType, error) {
	cth := string(ctx.Request.Header.ContentType())
	var rr requestReader
	var resType responseType
	if strings.HasPrefix(cth, "multipart/mixed") {
		resType = responseTypeMultipart
		var err error
		rr, err = s.parseMultipartRequest(ctx, cth)
		if err != nil {
			return rr, resType, err
		}
	} else if strings.HasPrefix(cth, "application/json") {
		resType = responseTypeJson
		var err error
		rr, err = s.parseJsonRequest(ctx, cth)
		if err != nil {
			return rr, resType, err
		}
	} else {
		s.logger.Warn("Non multipart request received.", zap.String("content-type", cth))
		return rr, resType, fmt.Errorf("unsupported content type: %s", cth)
	}

	return rr, resType, nil
}

func (s *ManiServer) parseJsonRequest(ctx *fasthttp.RequestCtx, cth string) (requestReader, error) {
	var requestBody io.Reader
	if ctx.IsBodyStream() {
		requestBody = ctx.RequestBodyStream()
	} else {
		requestBody = bytes.NewReader(ctx.PostBody())
	}
	jr := newJsonRequestReader(requestBody)
	if err := jr.init(); err != nil {
		s.logger.Warn("Failed to parse json body.", zap.String("content-type", cth), zap.Error(err))
		return nil, fmt.Errorf("failed to parse json body: %w", err)
	}
	return jr, nil
}

func (s *ManiServer) parseMultipartRequest(ctx *fasthttp.RequestCtx, cth string) (requestReader, error) {
	var boundary string
	for _, c := range strings.Split(cth, ";") {
		trimmed := strings.Trim(c, " ")
		if strings.HasPrefix(trimmed, "boundary=") {
			boundary = strings.TrimPrefix(trimmed, "boundary=")
			break
		}
	}

	if boundary == "" {
		s.logger.Warn("Failed to parse boundary from multipart header.", zap.String("content-type", cth))
		return nil, errors.New("failed to parse multipart boundary")
	}

	var requestBody io.Reader
	if ctx.IsBodyStream() {
		requestBody = ctx.RequestBodyStream()
	} else {
		requestBody = bytes.NewReader(ctx.PostBody())
	}

	return multipartRequestReader{mpr: multipart.NewReader(requestBody, boundary)}, nil
}
