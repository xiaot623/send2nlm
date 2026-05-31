package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type pluginRequest struct {
	Method    string     `json:"method"`
	URL       string     `json:"url,omitempty"`
	Resources []Resource `json:"resources,omitempty"`
}

type pluginResponse struct {
	Name    string `json:"name,omitempty"`
	Match   bool   `json:"match,omitempty"`
	PDFPath string `json:"pdf_path,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ServeProducer exposes a Producer implementation over a single stdio JSON call.
func ServeProducer(p Producer) {
	serve(func(ctx context.Context, req pluginRequest) (pluginResponse, error) {
		switch req.Method {
		case "metadata":
			return pluginResponse{Name: p.Name()}, nil
		case "match":
			return pluginResponse{Name: p.Name(), Match: p.Match(req.URL)}, nil
		case "produce":
			pdfPath, err := p.Produce(ctx, req.URL)
			if err != nil {
				return pluginResponse{Name: p.Name()}, err
			}
			return pluginResponse{Name: p.Name(), PDFPath: pdfPath}, nil
		default:
			return pluginResponse{Name: p.Name()}, fmt.Errorf("unknown producer method %q", req.Method)
		}
	})
}

// ServeReceiver exposes a Receiver implementation over a single stdio JSON call.
func ServeReceiver(r Receiver) {
	serve(func(ctx context.Context, req pluginRequest) (pluginResponse, error) {
		switch req.Method {
		case "metadata":
			return pluginResponse{Name: r.Name()}, nil
		case "receive":
			if err := r.Receive(ctx, req.Resources); err != nil {
				return pluginResponse{Name: r.Name()}, err
			}
			return pluginResponse{Name: r.Name()}, nil
		default:
			return pluginResponse{Name: r.Name()}, fmt.Errorf("unknown receiver method %q", req.Method)
		}
	})
}

func serve(handle func(context.Context, pluginRequest) (pluginResponse, error)) {
	var req pluginRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil && err != io.EOF {
		writePluginResponse(pluginResponse{Error: err.Error()})
		os.Exit(1)
	}
	resp, err := handle(context.Background(), req)
	if err != nil {
		resp.Error = err.Error()
	}
	writePluginResponse(resp)
	if err != nil {
		os.Exit(1)
	}
}

func writePluginResponse(resp pluginResponse) {
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}
