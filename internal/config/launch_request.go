package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
)

const maximumLaunchRequestSize = 1 << 20

func LoadLaunchRequest(path string) (models.LaunchRequest, error) {
	file, err := os.Open(path)
	if err != nil {
		return models.LaunchRequest{}, fmt.Errorf("open launch request: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return models.LaunchRequest{}, fmt.Errorf("inspect launch request: %w", err)
	}
	if info.Size() > maximumLaunchRequestSize {
		return models.LaunchRequest{}, errors.New("launch request exceeds 1 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumLaunchRequestSize+1))
	if err != nil {
		return models.LaunchRequest{}, fmt.Errorf("read launch request: %w", err)
	}
	if len(data) > maximumLaunchRequestSize {
		return models.LaunchRequest{}, errors.New("launch request exceeds 1 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var request models.LaunchRequest
	if err := decoder.Decode(&request); err != nil {
		return models.LaunchRequest{}, fmt.Errorf("decode launch request: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values are not allowed")
		}
		return models.LaunchRequest{}, fmt.Errorf("decode launch request trailer: %w", err)
	}
	return request, nil
}
