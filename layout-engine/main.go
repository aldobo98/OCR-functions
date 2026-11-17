package main

import (
	"encoding/json"
	"log/slog"
	"os"

	"github.com/aldobo98/layout-engine/process"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	file_data, err := os.ReadFile("ocr_result.json")
	if err != nil {
		logger.Error("Failed to load data from file", "error", err)
		return
	}
	var data process.OCRResult
	err = json.Unmarshal(file_data, &data)
	if err != nil {
		logger.Error("Failed to unmarshal json from data", "error", err)
		return
	}
	result, err := process.Process(data)
	if err != nil {
		logger.Error("Failed to process data", "error", err)
	}
	logger.Info("Final results", "result", result)

}
