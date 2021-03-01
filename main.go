package main

import (
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	logger.Info("Starting mani on :9999")

	srv := NewManiServer(logger)

	s := &fasthttp.Server{
		Handler: srv.HandleMani,
		Name:    "Mani Server",
	}

	if err := s.ListenAndServe(":9999"); err != nil {
		logger.Fatal("", zap.Error(err))
	}
}
