package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/aldobo98/layout-engine/process"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

func main() {

	client, err := cloudevents.NewClientHTTP()
	if err != nil {
		logger.Error("Failed to create client", "error", err)
		os.Exit(1)
	}
	logger.Info("Starting CloudEvent receiver on port 8080")

	if err := client.StartReceiver(context.Background(), receive); err != nil {
		logger.Error("Failed to start receiver", "error", err)
		os.Exit(1)
	}
}

func receive(event cloudevents.Event) error {
	logger.Info("Received cloudevent", "event", event)

	var data process.StartLayoutAnalysis
	if err := event.DataAs(&data); err != nil {
		logger.Error("Failed to decode event data", "error", err)
		//Ha nem tudom dekódolni, igazából nem tudok válaszolni??
		return err
	}

	logger.Info("Received request", "JobID", data.JobID)
	result, err := process.Process(data.OcrData)
	if err != nil {
		logger.Error("Failed to process data", "error", err)
		//Send back internalservererror
		callback := process.Callback{JobID: data.JobID, Step: "layout_processing", Status: "failed", Data: json.RawMessage{}, ErrorMessage: fmt.Sprintf("Failed to process data with error %d", err)}
		err_ := sendCallback(data.CallBackUrl, callback)
		if err_ != nil {
			logger.Error("Failed to send back callback", "error", err_)
		}
		return err
	}
	logger.Info("Final results", "result", result)
	json_data, err := json.Marshal(&result)
	if err != nil {
		logger.Error("Failed to marshal results", "error", err)
		callback := process.Callback{JobID: data.JobID, Step: "layout_processing", Status: "failed", Data: json.RawMessage{}, ErrorMessage: fmt.Sprintf("Failed to marshal data with error %d", err)}
		err_ := sendCallback(data.CallBackUrl, callback)
		if err_ != nil {
			logger.Error("Failed to send back callback", "error", err)
		}
	}
	callback := process.Callback{JobID: data.JobID, Step: "layout_processing", Status: "completed", Data: json_data, ErrorMessage: ""}
	err = sendCallback(data.CallBackUrl, callback)
	if err != nil {
		logger.Error("Failed to send back callback after successful processing", "error", err)
		return err
	}
	return nil
}

func sendCallback(url string, result process.Callback) error {
	body, err := json.Marshal(result)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Error("Failed to close response body", "error", err)
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("callback returned status: %d", resp.StatusCode)
	}
	return nil
}
