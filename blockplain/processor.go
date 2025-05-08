package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DataEntry represents a generic data structure that can be converted between formats
type DataEntry map[string]interface{}

// ProcessInputFile reads and processes an input file based on its extension
func ProcessInputFile(filePath string) (interface{}, error) {
	// Get file extension
	ext := strings.ToLower(filepath.Ext(filePath))

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Process based on file extension
	switch ext {
	case ".json":
		return processJSON(file)
	case ".csv":
		return processCSV(file)
	case ".yaml", ".yml":
		return processYAML(file)
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}
}

// processJSON handles JSON input files
func processJSON(reader io.Reader) (interface{}, error) {
	var data interface{}
	decoder := json.NewDecoder(reader)
	
	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}
	
	return data, nil
}

// processCSV handles CSV input files
func processCSV(reader io.Reader) (interface{}, error) {
	csvReader := csv.NewReader(reader)
	
	// Read header
	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV header: %v", err)
	}
	
	// Read records
	var records []DataEntry
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading CSV record: %v", err)
		}
		
		// Create map from header and record
		entry := make(DataEntry)
		for i, field := range record {
			if i < len(header) {
				entry[header[i]] = field
			}
		}
		
		records = append(records, entry)
	}
	
	return records, nil
}

// processYAML handles YAML input files
func processYAML(reader io.Reader) (interface{}, error) {
	var data interface{}
	decoder := yaml.NewDecoder(reader)
	
	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("error parsing YAML: %v", err)
	}
	
	return data, nil
}